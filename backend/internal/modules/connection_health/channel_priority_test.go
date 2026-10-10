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

func (f *fakeRepository) ListChannelPriorities(_ context.Context, user, workspace string) (map[string]bool, error) {
	result := map[string]bool{}
	prefix := user + "|" + workspace + "|"
	for key, enabled := range f.channelPriorities {
		if strings.HasPrefix(key, prefix) {
			result[strings.TrimPrefix(key, prefix)] = enabled
		}
	}
	return result, nil
}

func (f *fakeRepository) SetChannelPriority(_ context.Context, user, workspace, target string, enabled bool) error {
	f.channelPriorities[user+"|"+workspace+"|"+target] = enabled
	return nil
}

func TestChannelPrioritySharedPreferenceAndDisplay(t *testing.T) {
	svc, _, repo, _ := qualityTestService(t)
	p := &repo.policies[0]
	p.AutoSuspendEnabled, p.AutoDegradeEnabled = true, true
	p.PriorityMode, p.LatencyPriority = PriorityModeLatency, defaultLatencyPriorityConfig()
	p.ModelTargets = []ModelTarget{{ModelName: "model", Enabled: true}}
	for _, enabled := range []bool{true, false, true} {
		saved, err := svc.SetChannelPriority(context.Background(), "user", "sub2api:ws1:a", enabled)
		if err != nil || saved.Enabled != enabled || saved.RestorePending {
			t.Fatalf("save: %+v %v", saved, err)
		}
		groups, err := svc.AdminGroups(context.Background(), "user")
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range groups {
			for _, a := range group.Accounts {
				if a.TargetID != saved.TargetID {
					continue
				}
				if a.PriorityEnabled != enabled || !a.SuspensionEnabled || a.PriorityRestorePending {
					t.Fatalf("shared switch lost or suspension changed: %+v", a)
				}
			}
		}
	}
	for _, target := range []string{"sub2api:other:a", "newapi:ws1:a", "sub2api:ws1:missing", "bad"} {
		if _, err := svc.SetChannelPriority(context.Background(), "user", target, false); err == nil {
			t.Fatal("accepted foreign or absent channel")
		}
	}
}

type channelPriorityActioner struct {
	calls   []int
	current *int
	err     error
}

func (a *channelPriorityActioner) UpdateAdminTargetPriority(_ upstream.Session, _ string, priority int) error {
	a.calls = append(a.calls, priority)
	if a.err != nil {
		return a.err
	}
	*a.current = priority
	return nil
}

func prioritySwitchFixture(t *testing.T) (*Service, *fakeRepository, *channelPriorityActioner, string, Policy, []GroupPolicyAssignment) {
	t.Helper()
	svc, _, repo, _ := qualityTestService(t)
	p := latencyTestPolicy()
	p.UserID = "user"
	current := 37
	reader := fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "g1"}, {ID: "g2"}}, accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{
		"g1": {{ID: "100", Priority: &current, Models: "model"}}, "g2": {{ID: "100", Priority: &current, Models: "model"}},
	}}
	actions := &channelPriorityActioner{current: &current}
	svc.platformGroups, svc.priorityActions = reader, actions
	bindings := []GroupPolicyAssignment{{UserID: "user", AdminAccountID: "ws1", AdminGroupID: "g1", PolicyID: p.ID}, {UserID: "user", AdminAccountID: "ws1", AdminGroupID: "g2", PolicyID: p.ID}}
	return svc, repo, actions, "sub2api:ws1:100", p, bindings
}

func TestChannelPrioritySwitchRestoresAndReenables(t *testing.T) {
	ctx := context.Background()
	svc, repo, actions, target, policy, bindings := prioritySwitchFixture(t)
	for _, mode := range []string{PriorityModeLatency, PriorityModeMultiplier} {
		policy.PriorityMode = mode
		reader := svc.platformGroups.(fakePlatformGroupReader)
		for i := range reader.groups {
			reader.groups[i].Multiplier = float64Ptr(1)
		}
		svc.platformGroups = reader
		_, _ = svc.SetChannelPriority(ctx, "user", target, true)
		svc.syncMultiplierPriorities(ctx, []Policy{policy}, nil, bindings, nil, nil)
		if *actions.current == 37 || repo.priorityStates["user|ws1|"+target].OriginalPriority != 37 {
			t.Fatal("switch did not allow policy priority or preserve original")
		}
		saved, err := svc.SetChannelPriority(ctx, "user", target, false)
		if err != nil || saved.Enabled || saved.RestorePending || saved.Priority == nil || *saved.Priority != 37 || *actions.current != 37 || len(repo.priorityStates) != 0 {
			t.Fatalf("original priority not immediately restored: %+v %v", saved, err)
		}
		calls := len(actions.calls)
		svc.syncMultiplierPriorities(ctx, []Policy{policy}, nil, bindings, nil, nil)
		if len(actions.calls) != calls || len(repo.priorityStates) != 0 {
			t.Fatal("disabled shared channel was taken over again")
		}
	}
	// On the next opt-in, preserve the current original value, including user edits.
	*actions.current = 42
	_, _ = svc.SetChannelPriority(ctx, "user", target, true)
	svc.syncMultiplierPriorities(ctx, []Policy{policy}, nil, bindings, nil, nil)
	if repo.priorityStates["user|ws1|"+target].OriginalPriority != 42 {
		t.Fatal("re-enabling reused a stale original priority")
	}
}

func TestChannelPriorityRestoreRetryAndManualChanges(t *testing.T) {
	for _, manual := range []bool{false, true} {
		ctx := context.Background()
		svc, repo, actions, target, policy, bindings := prioritySwitchFixture(t)
		svc.syncMultiplierPriorities(ctx, []Policy{policy}, nil, bindings, nil, nil)
		if manual {
			*actions.current = 23
		}
		actions.err = errors.New("upstream unavailable")
		saved, err := svc.SetChannelPriority(ctx, "user", target, false)
		if err != nil || saved.Enabled || saved.RestorePending == manual {
			t.Fatalf("wrong restore result: %+v %v", saved, err)
		}
		if !manual && repo.priorityStates["user|ws1|"+target].OriginalPriority != 37 {
			t.Fatal("failed restore lost original priority")
		}
		actions.err = nil
		stale, _ := repo.ListPrioritySyncStates(ctx, "user", "ws1")
		svc.syncMultiplierPriorities(ctx, []Policy{policy}, nil, bindings, nil, stale)
		want := 37
		if manual {
			want = 23
		}
		if *actions.current != want || len(repo.priorityStates) != 0 {
			t.Fatal("restore did not retry or overwrote manual priority")
		}
	}
}

type priorityLeaseCallbackRepo struct {
	*fakeRepository
	onLease func()
}

func (r priorityLeaseCallbackRepo) AcquireTargetLease(_ context.Context, key string) (func(), error) {
	if strings.HasPrefix(key, "priority:") {
		r.onLease()
	}
	return func() {}, nil
}

func TestChannelPrioritySyncRefreshesInputsAfterSwitchSave(t *testing.T) {
	ctx := context.Background()
	svc, repo, actions, target, policy, bindings := prioritySwitchFixture(t)
	svc.syncMultiplierPriorities(ctx, []Policy{policy}, nil, bindings, nil, nil)
	stale, _ := repo.ListPrioritySyncStates(ctx, "user", "ws1")
	cache := make(adminInventoryCache)
	_, _ = svc.loadAdminInventory(ctx, "user", "ws1", cache)
	svc.repo = priorityLeaseCallbackRepo{repo, func() {
		// A save completed while sync was waiting for its lease.
		_ = repo.SetChannelPriority(ctx, "user", "ws1", target, false)
		*actions.current = 37
		_ = repo.DeletePrioritySyncState(ctx, "user", "ws1", target)
	}}
	calls := len(actions.calls)
	svc.syncMultiplierPrioritiesWithCache(ctx, []Policy{policy}, nil, bindings, nil, stale, cache)
	if len(actions.calls) != calls || len(repo.priorityStates) != 0 {
		t.Fatal("stale maintenance inputs undid a completed opt-out")
	}
}

type unavailablePrioritySwitchRepo struct{ *fakeRepository }

func (r unavailablePrioritySwitchRepo) ListChannelPriorities(context.Context, string, string) (map[string]bool, error) {
	return nil, errors.New("database unavailable")
}

func TestChannelPriorityReadFailureDoesNotWrite(t *testing.T) {
	svc, repo, actions, _, p, bindings := prioritySwitchFixture(t)
	svc.repo = unavailablePrioritySwitchRepo{repo}
	svc.syncMultiplierPriorities(context.Background(), []Policy{p}, nil, bindings, nil, nil)
	if len(actions.calls) != 0 {
		t.Fatal("unreadable switch allowed priority changes")
	}
}

func TestChannelPriorityOffKeepsProbeCadenceAndSuspension(t *testing.T) {
	svc, repo, _ := cadenceFixture(1)
	p := &repo.policies[0]
	p.AutoSuspendEnabled, p.AutoRemoteActionEnabled = true, true
	p.PriorityMode = PriorityModeLatency
	target := "sub2api:ws1:0"
	_ = repo.SetChannelPriority(context.Background(), "user1", "ws1", target, false)
	specs, err := svc.currentScheduledModels(context.Background(), adminProbeJob{userID: "user1", adminAccountID: "ws1", target: AdminProbeTarget{TargetID: target, Models: []string{"model-0"}}, groups: []upstream.AdminGroupInfo{{ID: "g1"}}})
	if err != nil || len(specs) != 1 || !specs[0].policy.AutoSuspendEnabled || !specs[0].policy.AutoDegradeEnabled || specs[0].policy.ProbeIntervalSeconds != 60 {
		t.Fatalf("priority switch changed probing or suspension: %+v %v", specs, err)
	}
}

func TestChannelPriorityRestoreHandlesPendingWrites(t *testing.T) {
	for _, current := range []int{37, 50, 100, 23} {
		ctx := context.Background()
		svc, repo, actions, target, _, _ := prioritySwitchFixture(t)
		*actions.current = current
		repo.priorityStates["user|ws1|"+target] = PrioritySyncState{UserID: "user", AdminAccountID: "ws1", TargetID: target, OriginalPriority: 37, LastAppliedPriority: 50, PendingPriority: intPtr(100)}
		saved, err := svc.SetChannelPriority(ctx, "user", target, false)
		want := 37
		if current == 23 {
			want = 23
		}
		if err != nil || saved.RestorePending || *actions.current != want || len(repo.priorityStates) != 0 {
			t.Fatalf("pending update not reconciled: current=%d saved=%+v err=%v", current, saved, err)
		}
	}
}

func TestChannelPriorityHandlerRequiresExplicitBoolean(t *testing.T) {
	svc, _, _, _ := qualityTestService(t)
	h := &Handler{service: svc}
	for _, tt := range []struct {
		body          string
		authenticated bool
		status        int
	}{
		{`{"enabled":false}`, false, 401}, {`{}`, true, 400}, {`{"enabled":null}`, true, 400}, {`{"enabled":"false"}`, true, 400}, {`{"enabled":false}`, true, 200},
	} {
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(tt.body))
		r.SetPathValue("id", "sub2api:ws1:a")
		if tt.authenticated {
			r = r.WithContext(authctx.WithUserID(r.Context(), "user"))
		}
		w := httptest.NewRecorder()
		h.setChannelPriority(w, r)
		if w.Code != tt.status {
			t.Fatalf("%s status=%d want=%d", tt.body, w.Code, tt.status)
		}
	}
}

func TestChannelPriorityPostgresPersistence(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	migration, err := os.ReadFile("../../database/migrations/000025_channel_priority_switches.sql")
	if err != nil || strings.TrimSpace(strings.ReplaceAll(string(migration), "\r\n", "\n")) != strings.TrimSpace(channelPrioritySchema) {
		t.Fatal("schema migration differs", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := pool.Exec(ctx, strings.ReplaceAll(channelPrioritySchema, "CREATE TABLE IF NOT EXISTS", "CREATE TEMP TABLE IF NOT EXISTS")); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewRepository(pool)
	settings, err := repo.ListChannelPriorities(ctx, "u", "w")
	if err != nil || !channelPriorityEnabled(settings, "channel") {
		t.Fatal("existing channels must follow policy by default")
	}
	for _, enabled := range []bool{false, true, false} {
		if err := repo.SetChannelPriority(ctx, "u", "w", "channel", enabled); err != nil {
			t.Fatal(err)
		}
		settings, err = NewRepository(pool).ListChannelPriorities(ctx, "u", "w")
		if err != nil || channelPriorityEnabled(settings, "channel") != enabled {
			t.Fatal("preference lost after repository reload")
		}
	}
	for _, scope := range [][2]string{{"other", "w"}, {"u", "other"}} {
		settings, err := repo.ListChannelPriorities(ctx, scope[0], scope[1])
		if err != nil || !channelPriorityEnabled(settings, "channel") {
			t.Fatal("switch leaked into another scope")
		}
	}
}
