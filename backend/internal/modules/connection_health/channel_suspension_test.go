package connection_health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"transithub/backend/internal/modules/upstream"
	"transithub/backend/internal/shared/authctx"
)

func (f *fakeRepository) GetChannelSuspension(_ context.Context, user, workspace, target string) (bool, error) {
	enabled, exists := f.channelSuspensions[user+"|"+workspace+"|"+target]
	return !exists || enabled, nil
}
func (f *fakeRepository) ListChannelSuspensions(_ context.Context, user, workspace string) (map[string]bool, error) {
	result := map[string]bool{}
	prefix := user + "|" + workspace + "|"
	for key, enabled := range f.channelSuspensions {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result[key[len(prefix):]] = enabled
		}
	}
	return result, nil
}
func (f *fakeRepository) SetChannelSuspension(_ context.Context, user, workspace, target string, enabled bool) error {
	f.channelSuspensions[user+"|"+workspace+"|"+target] = enabled
	return nil
}

func TestChannelSuspensionSharedPreferenceAndDisplay(t *testing.T) {
	svc, _, repo, _ := qualityTestService(t)
	p := &repo.policies[0]
	p.AutoSuspendEnabled, p.AutoDegradeEnabled = true, true
	p.PriorityMode, p.LatencyPriority = PriorityModeLatency, defaultLatencyPriorityConfig()
	p.ModelTargets = []ModelTarget{{ModelName: "model", Enabled: true}}
	target := "sub2api:ws1:a"
	repo.states[target] = map[string]ConnectionHealthState{"model": {ConnectionID: target, UserID: "user", AdminAccountID: "ws1", ModelName: "model", State: StateSuspended}}
	for _, enabled := range []bool{true, false, true} {
		saved, err := svc.SetChannelSuspension(context.Background(), "user", target, enabled)
		if err != nil || saved.Enabled != enabled {
			t.Fatalf("save: %+v %v", saved, err)
		}
		groups, err := svc.AdminGroups(context.Background(), "user")
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range groups {
			if group.ID == "off" {
				continue
			}
			a := group.Accounts[0]
			if !a.SuspensionSupported || a.SuspensionEnabled != enabled || !a.HasEnabledProbePolicy {
				t.Fatalf("shared preference/selection lost: %+v", a)
			}
			wantState, wantPriority := StateDegraded, 100
			if enabled {
				wantState, wantPriority = StateSuspended, 10000
			}
			if a.ModelHealth[0].State != wantState || a.LatencyPriority == nil || a.LatencyPriority.Priority != wantPriority {
				t.Fatalf("incorrect effective state/priority: %+v", a)
			}
		}
	}
	if !repo.policies[0].AutoSuspendEnabled || len(repo.groupAssignments) != 2 {
		t.Fatal("channel preference changed policy settings")
	}
	for _, target := range []string{"sub2api:other:a", "newapi:ws1:a", "sub2api:ws1:missing", "bad"} {
		if _, err := svc.SetChannelSuspension(context.Background(), "user", target, false); err == nil {
			t.Fatal("accepted foreign or absent channel")
		}
	}
}

func TestChannelSuspensionOnlyRemovesPermissionAndKeepsCadence(t *testing.T) {
	svc, repo, _ := cadenceFixture(1)
	p := &repo.policies[0]
	p.AutoSuspendEnabled, p.AutoRemoteActionEnabled = true, true
	p.PriorityMode = PriorityModeLatency
	target := "sub2api:ws1:0"
	_ = repo.SetChannelSuspension(context.Background(), "user1", "ws1", target, false)
	specs, err := svc.currentScheduledModels(context.Background(), adminProbeJob{userID: "user1", adminAccountID: "ws1", target: AdminProbeTarget{TargetID: target, Models: []string{"model-0"}}, groups: []upstream.AdminGroupInfo{{ID: "g1"}}})
	if err != nil || len(specs) != 1 || specs[0].policy.AutoSuspendEnabled || !specs[0].policy.AutoDegradeEnabled || specs[0].policy.PriorityMode != PriorityModeLatency || specs[0].policy.ProbeIntervalSeconds != 60 {
		t.Fatalf("switch changed probing instead of just suspension: %+v %v", specs, err)
	}
	p.AutoSuspendEnabled = false
	_ = repo.SetChannelSuspension(context.Background(), "user1", "ws1", target, true)
	if svc.currentTargetActionPermissions(context.Background(), "user1", "ws1", target, *p).AutoSuspendEnabled {
		t.Fatal("channel enabled bypassed policy opt-out")
	}
}

func TestChannelSuspensionOffRestoresSystemActionAndProtectsManualDisable(t *testing.T) {
	for _, platform := range []upstream.Platform{upstream.PlatformSub2API, upstream.PlatformNewAPI} {
		for _, manual := range []bool{false, true} {
			repo := newFakeRepository()
			p := sub2APIProbePolicy(true)
			target := string(platform) + ":ws1:100"
			status, original := "inactive", "active"
			if platform == upstream.PlatformNewAPI {
				status, original = "2", "1"
			}
			stored := TargetActionState{UserID: "user1", AdminAccountID: "ws1", TargetID: target, OriginalStatus: original, OriginalWeight: intPtr(37), LastAppliedStatus: status, LastAppliedWeight: intPtr(0), Conflict: manual}
			repo.targetActionStates["user1|ws1|"+target] = stored
			_ = repo.SetChannelSuspension(context.Background(), "user1", "ws1", target, false)
			reader := fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "g1"}}, accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{"g1": {{ID: "100", Status: status, Weight: intPtr(0), Models: "gpt-4o"}}}}
			actions := &fakePlatformActioner{}
			svc := newAdminTargetsRemoteActionService(reader, fakeMySitesReader{session: upstream.Session{Platform: platform}}, repo, actions)
			svc.restoreUnmanagedTargetActions(context.Background(), []Policy{p}, nil, []GroupPolicyAssignment{{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", PolicyID: p.ID}}, nil, []TargetActionState{stored}, adminInventoryCache{})
			count := len(actions.calls) + len(actions.sub2APICalls)
			if manual {
				if count != 0 {
					t.Fatal("manual disable was overwritten")
				}
				continue
			}
			if count != 1 || len(repo.targetActionStates) != 0 {
				t.Fatal("system suspension did not restore and release ownership")
			}
			if platform == upstream.PlatformSub2API && actions.sub2APICalls[0].status != "active" {
				t.Fatal("wrong restored status")
			}
			if platform == upstream.PlatformNewAPI && (actions.calls[0].status != 1 || actions.calls[0].weight != 37) {
				t.Fatal("lost original channel weight")
			}
		}
	}
}

func TestChannelSuspensionOffStillWritesDegradedLatencyPriority(t *testing.T) {
	repo := newFakeRepository()
	p := latencyTestPolicy()
	p.AutoSuspendEnabled = true
	target := "sub2api:ws1:100"
	_ = repo.SetChannelSuspension(context.Background(), "user1", "ws1", target, false)
	actions := &fakeTargetPriorityActioner{}
	svc := &Service{repo: repo, priorityActions: actions}
	states := []ConnectionHealthState{{ConnectionID: target, ModelName: "model", State: StateSuspended}}
	for _, enabled := range []bool{false, true} {
		_ = repo.SetChannelSuspension(context.Background(), "user1", "ws1", target, enabled)
		inventory := map[string]*priorityTargetInventory{target: {target: AdminProbeTarget{AccountID: "100"}, policies: []Policy{p}, currentPriority: 1}}
		svc.syncWorkspacePriorities(context.Background(), upstream.Session{Platform: upstream.PlatformSub2API}, "user1", "ws1", inventory, true, states, nil)
		want := 100
		if enabled {
			want = 10000
		}
		if len(actions.calls) == 0 || actions.calls[len(actions.calls)-1].priority != want {
			t.Fatal("priority did not use effective channel permission")
		}
	}
}

type unavailableSuspensionRepo struct{ *fakeRepository }

func (r unavailableSuspensionRepo) GetChannelSuspension(context.Context, string, string, string) (bool, error) {
	return true, errors.New("unavailable")
}
func TestChannelSuspensionReadFailureCannotPause(t *testing.T) {
	repo := newFakeRepository()
	p := sub2APIProbePolicy(true)
	repo.policies = []Policy{p}
	actions := &fakePlatformActioner{}
	svc := &Service{repo: unavailableSuspensionRepo{repo}, dispatcher: newRemoteActionDispatcher(nil, nil, actions)}
	target := AdminProbeTarget{TargetID: "sub2api:ws1:100", AccountID: "100", Platform: "sub2api", AccountStatus: "active"}
	if svc.currentTargetActionPermissions(context.Background(), "user1", "ws1", target.TargetID, p).AutoSuspendEnabled {
		t.Fatal("read failure granted permission")
	}
	repo.states[target.TargetID] = map[string]ConnectionHealthState{"gpt-4o": {ModelName: "gpt-4o", State: StateSuspended}}
	_, err := svc.reconcileTargetRemoteAction(context.Background(), "user1", "ws1", upstream.Session{Platform: upstream.PlatformSub2API}, target, []probeModelSpec{{modelName: "gpt-4o", policy: p}})
	if err == nil || len(actions.sub2APICalls) != 0 {
		t.Fatal("read failure allowed a remote action")
	}
}

func TestChannelSuspensionHandlerRequiresExplicitBoolean(t *testing.T) {
	svc, _, _, _ := qualityTestService(t)
	h := &Handler{service: svc}
	for _, tt := range []struct {
		body          string
		authenticated bool
		status        int
	}{{`{"enabled":false}`, false, 401}, {`{}`, true, 400}, {`{"enabled":null}`, true, 400}, {`{"enabled":"false"}`, true, 400}, {`{"enabled":false}`, true, 200}} {
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(tt.body))
		r.SetPathValue("id", "sub2api:ws1:a")
		if tt.authenticated {
			r = r.WithContext(authctx.WithUserID(r.Context(), "user"))
		}
		w := httptest.NewRecorder()
		h.setChannelSuspension(w, r)
		if w.Code != tt.status {
			t.Fatalf("%s status=%d want=%d", tt.body, w.Code, tt.status)
		}
	}
}

func TestChannelSuspensionPostgresPersistence(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	migration, err := os.ReadFile("../../database/migrations/000024_channel_suspension_switches.sql")
	if err != nil || strings.TrimSpace(strings.ReplaceAll(string(migration), "\r\n", "\n")) != strings.TrimSpace(channelSuspensionSchema) {
		t.Fatal("schema migration differs", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := pool.Exec(ctx, strings.ReplaceAll(channelSuspensionSchema, "CREATE TABLE IF NOT EXISTS", "CREATE TEMP TABLE IF NOT EXISTS")); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewRepository(pool)
	if enabled, err := repo.GetChannelSuspension(ctx, "u", "w", "channel"); err != nil || !enabled {
		t.Fatal("existing channels must default to following policy")
	}
	for _, enabled := range []bool{false, true, false} {
		if err := repo.SetChannelSuspension(ctx, "u", "w", "channel", enabled); err != nil {
			t.Fatal(err)
		}
		loaded, err := NewRepository(pool).GetChannelSuspension(ctx, "u", "w", "channel")
		if err != nil || loaded != enabled {
			t.Fatal("preference did not survive repository reload")
		}
	}
	for _, scope := range [][2]string{{"other", "w"}, {"u", "other"}} {
		if enabled, err := repo.GetChannelSuspension(ctx, scope[0], scope[1], "channel"); err != nil || !enabled {
			t.Fatal("preference leaked to another scope")
		}
	}
	settings, err := repo.ListChannelSuspensions(ctx, "u", "w")
	if err != nil || len(settings) != 1 || settings["channel"] {
		t.Fatal("preference missing from bulk read")
	}
}

type revokingSuspensionLeaseRepo struct{ *fakeRepository }

func (r revokingSuspensionLeaseRepo) AcquireTargetLease(ctx context.Context, key string) (func(), error) {
	if strings.HasPrefix(key, "suspension:") {
		_ = r.SetChannelSuspension(ctx, "user1", "ws1", "sub2api:ws1:100", false)
	}
	return func() {}, nil
}

func TestChannelSuspensionRechecksPermissionInsideActionLease(t *testing.T) {
	repo := newFakeRepository()
	p := sub2APIProbePolicy(true)
	target := AdminProbeTarget{TargetID: "sub2api:ws1:100", AccountID: "100", Platform: "sub2api", AccountStatus: "active"}
	repo.states[target.TargetID] = map[string]ConnectionHealthState{"gpt-4o": {ModelName: "gpt-4o", State: StateSuspended}}
	actions := &fakePlatformActioner{}
	svc := &Service{repo: revokingSuspensionLeaseRepo{repo}, dispatcher: newRemoteActionDispatcher(nil, nil, actions)}
	_, err := svc.reconcileTargetRemoteAction(context.Background(), "user1", "ws1", upstream.Session{Platform: upstream.PlatformSub2API}, target, []probeModelSpec{{modelName: "gpt-4o", policy: p}})
	if err != nil || len(actions.sub2APICalls) != 0 {
		t.Fatal("a stale probe permission bypassed a switch saved before the remote action")
	}
}
