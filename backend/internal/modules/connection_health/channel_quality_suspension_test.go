package connection_health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"transithub/backend/internal/shared/authctx"
)

func (f *fakeRepository) GetChannelQualitySuspension(_ context.Context, user, workspace, target string) (bool, error) {
	return f.channelQualitySuspensions[user+"|"+workspace+"|"+target], nil
}
func (f *fakeRepository) ListChannelQualitySuspensions(_ context.Context, user, workspace string) (map[string]bool, error) {
	result := map[string]bool{}
	prefix := user + "|" + workspace + "|"
	for key, enabled := range f.channelQualitySuspensions {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result[key[len(prefix):]] = enabled
		}
	}
	return result, nil
}
func (f *fakeRepository) SetChannelQualitySuspension(_ context.Context, user, workspace, target string, enabled bool) error {
	f.channelQualitySuspensions[user+"|"+workspace+"|"+target] = enabled
	return nil
}

func TestChannelQualitySuspensionHandlerRequiresExplicitBoolean(t *testing.T) {
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
		h.setChannelQualitySuspension(w, r)
		if w.Code != tt.status {
			t.Fatalf("%s status=%d want=%d", tt.body, w.Code, tt.status)
		}
	}
}

func TestChannelQualitySuspensionDefaultOffAndReenable(t *testing.T) {
	f := newQualityActionFixture(t)
	delete(f.health.channelQualitySuspensions, "user|ws1|"+qualityActionTarget)
	for i := 0; i < 3; i++ {
		f.probe(t, "failed", true)
	}
	f.assertActions(t)
	if !f.quality.states["user|ws1"][qualityActionTarget].Degraded {
		t.Fatal("switch erased quality evidence")
	}
	if _, err := f.svc.SetChannelQualitySuspension(context.Background(), "user", qualityActionTarget, true); err != nil {
		t.Fatal(err)
	}
	f.assertActions(t)
	f.probe(t, "failed", true)
	f.assertActions(t, "inactive")
	f.probe(t, "passed", false)
	f.assertActions(t, "inactive")
	f.probe(t, "passed", false)
	f.assertActions(t, "inactive", "active")
}

func TestChannelQualitySuspensionOffImmediatelyReleasesOnlyQualityHold(t *testing.T) {
	for _, mode := range []string{"healthy", "no health samples", "health suspended", "health recovery pending", "policy removed", "channel pause off", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			f := newQualityActionFixture(t)
			for i := 0; i < 3; i++ {
				f.probe(t, "failed", true)
			}
			before, _ := json.Marshal(f.quality.states)
			switch mode {
			case "no health samples":
				delete(f.health.states, qualityActionTarget)
			case "health suspended", "health recovery pending":
				state := f.health.states[qualityActionTarget]["quality-model"]
				state.State = StateSuspended
				if mode == "health recovery pending" {
					state.State = StateDegraded
				}
				f.health.states[qualityActionTarget]["quality-model"] = state
			case "policy removed":
				f.health.groupAssignments = nil
			case "channel pause off":
				_ = f.health.SetChannelSuspension(context.Background(), "user", "ws1", qualityActionTarget, false)
			case "conflict":
				state := f.health.targetActionStates["user|ws1|"+qualityActionTarget]
				state.Conflict = true
				f.health.targetActionStates["user|ws1|"+qualityActionTarget] = state
			}
			saved, err := f.svc.SetChannelQualitySuspension(context.Background(), "user", qualityActionTarget, false)
			if err != nil || saved.Enabled || saved.RestorePending != (mode == "conflict") {
				t.Fatalf("switch: %+v %v", saved, err)
			}
			healthHold := strings.HasPrefix(mode, "health ")
			if healthHold || mode == "conflict" {
				f.assertActions(t, "inactive")
				if healthHold && f.health.targetActionStates["user|ws1|"+qualityActionTarget].QualitySuspended {
					t.Fatal("did not clear quality reason")
				}
			} else {
				f.assertActions(t, "inactive", "active")
				if len(f.health.targetActionStates) != 0 {
					t.Fatal("owner retained after restore")
				}
			}
			after, _ := json.Marshal(f.quality.states)
			if string(before) != string(after) {
				t.Fatal("switch changed quality evidence")
			}
			f.maintain()
			if !healthHold && mode != "conflict" {
				f.assertActions(t, "inactive", "active")
			}
		})
	}
}

func TestChannelQualitySuspensionFailedRestoreRetries(t *testing.T) {
	f := newQualityActionFixture(t)
	for i := 0; i < 3; i++ {
		f.probe(t, "failed", true)
	}
	f.remote.sub2APIErr = errors.New("offline")
	saved, err := f.svc.SetChannelQualitySuspension(context.Background(), "user", qualityActionTarget, false)
	if err != nil || saved.Enabled || !saved.RestorePending {
		t.Fatalf("saved state: %+v %v", saved, err)
	}
	if f.health.targetActionStates["user|ws1|"+qualityActionTarget].PendingStatus != "active" {
		t.Fatal("missing retry checkpoint")
	}
	f.remote.sub2APIErr = nil
	f.maintain()
	f.assertActions(t, "inactive", "active", "active")
	if len(f.health.targetActionStates) != 0 {
		t.Fatal("retry retained owner")
	}
	f.probe(t, "failed", true)
	f.assertActions(t, "inactive", "active", "active")
}

func TestChannelQualitySuspensionOffPreservesHealthUntilRecoveryThreshold(t *testing.T) {
	f := newQualityActionFixture(t)
	for i := 0; i < 3; i++ {
		f.probe(t, "failed", true)
	}
	state := f.health.states[qualityActionTarget]["quality-model"]
	state.State, state.ConsecutiveFailures = StateSuspended, 5
	f.health.states[qualityActionTarget]["quality-model"] = state
	if _, err := f.svc.SetChannelQualitySuspension(context.Background(), "user", qualityActionTarget, false); err != nil {
		t.Fatal(err)
	}
	f.assertActions(t, "inactive")
	if current := f.health.states[qualityActionTarget]["quality-model"]; current.State != StateSuspended || current.ConsecutiveFailures != 5 {
		t.Fatal("quality switch changed health state or counters")
	}
	p := f.health.policies[0]
	p.SuccessThreshold = 2
	f.health.policies[0] = p
	for i := 1; i <= 2; i++ {
		out := Transition(TransitionInput{Current: state.State, ConsecutiveSuccesses: state.ConsecutiveSuccesses, ConsecutiveFailures: state.ConsecutiveFailures, Policy: p, Result: ResultOK})
		state.State, state.ConsecutiveSuccesses, state.ConsecutiveFailures = out.NextState, out.ConsecutiveSuccesses, out.ConsecutiveFailures
		f.health.states[qualityActionTarget]["quality-model"] = state
		if err := f.svc.reconcileQualityTarget(context.Background(), "user", "ws1", qualityActionTarget); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			f.assertActions(t, "inactive")
		} else {
			f.assertActions(t, "inactive", "active")
		}
	}
}

func TestChannelQualitySuspensionOffDuringProbe(t *testing.T) {
	f := newQualityActionFixture(t)
	f.probe(t, "failed", true)
	f.probe(t, "failed", true)
	f.runner.hook = func() {
		if _, err := f.svc.SetChannelQualitySuspension(context.Background(), "user", qualityActionTarget, false); err != nil {
			t.Fatal(err)
		}
	}
	f.probe(t, "failed", true)
	f.assertActions(t)
}

type unavailableQualityPausePolicyRepo struct{ *fakeRepository }

func (r unavailableQualityPausePolicyRepo) GetPolicy(context.Context, string, string, string) (*Policy, error) {
	return nil, errors.New("policy read unavailable")
}

func TestChannelQualitySuspensionPolicyReadFailureCannotReleaseHealth(t *testing.T) {
	f := newQualityActionFixture(t)
	for i := 0; i < 3; i++ {
		f.probe(t, "failed", true)
	}
	state := f.health.states[qualityActionTarget]["quality-model"]
	state.State = StateSuspended
	f.health.states[qualityActionTarget]["quality-model"] = state
	f.svc.repo = unavailableQualityPausePolicyRepo{f.health}
	saved, err := f.svc.SetChannelQualitySuspension(context.Background(), "user", qualityActionTarget, false)
	if err != nil || saved.Enabled || !saved.RestorePending {
		t.Fatalf("save: %+v %v", saved, err)
	}
	f.assertActions(t, "inactive")
	f.svc.repo = f.health
	f.maintain()
	f.assertActions(t, "inactive")
	if f.health.targetActionStates["user|ws1|"+qualityActionTarget].QualitySuspended {
		t.Fatal("quality reason retained after retry")
	}
}

func TestChannelQualitySuspensionSharedDisplayAndScope(t *testing.T) {
	f := newQualityActionFixture(t)
	for _, enabled := range []bool{false, true} {
		if _, err := f.svc.SetChannelQualitySuspension(context.Background(), "user", qualityActionTarget, enabled); err != nil {
			t.Fatal(err)
		}
		groups, err := f.svc.AdminGroups(context.Background(), "user")
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, g := range groups {
			for _, a := range g.Accounts {
				if a.TargetID == qualityActionTarget {
					found++
					if a.QualitySuspensionEnabled != enabled {
						t.Fatal("shared preference differs")
					}
				}
			}
		}
		if found < 2 {
			t.Fatal("fixture must share the channel")
		}
	}
	for _, target := range []string{"sub2api:other:a", "newapi:ws1:a", "sub2api:ws1:missing", "bad"} {
		if _, err := f.svc.SetChannelQualitySuspension(context.Background(), "user", target, false); err == nil {
			t.Fatal("accepted foreign target")
		}
	}
}

func TestChannelQualitySuspensionPostgresPersistence(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	if _, err := pool.Exec(ctx, `CREATE TEMP TABLE connection_health_target_action_states(user_id text, admin_account_id text, target_id text, quality_suspended boolean);
INSERT INTO connection_health_target_action_states VALUES('u','w','held',true),('u','w','health-only',false);`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../database/migrations/000032_channel_quality_suspension.sql")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(strings.ReplaceAll(string(migration), "\r\n", "\n")) != channelQualitySuspensionSchema+"\n\n"+channelQualitySuspensionBackfill {
		t.Fatal("migration differs")
	}
	for i := 0; i < 2; i++ {
		if _, err := pool.Exec(ctx, strings.ReplaceAll(string(migration), "CREATE TABLE IF NOT EXISTS", "CREATE TEMP TABLE IF NOT EXISTS")); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewRepository(pool)
	for target, want := range map[string]bool{"new": false, "health-only": false, "held": true} {
		if enabled, err := repo.GetChannelQualitySuspension(ctx, "u", "w", target); err != nil || enabled != want {
			t.Fatalf("%s=%v %v", target, enabled, err)
		}
	}
	for _, enabled := range []bool{false, true, false} {
		if err := repo.SetChannelQualitySuspension(ctx, "u", "w", "held", enabled); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, channelQualitySuspensionBackfill); err != nil {
			t.Fatal(err)
		}
		got, err := NewRepository(pool).GetChannelQualitySuspension(ctx, "u", "w", "held")
		if err != nil || got != enabled {
			t.Fatal("restart/backfill overwrote preference")
		}
	}
	for _, scope := range [][2]string{{"other", "w"}, {"u", "other"}} {
		if enabled, err := repo.GetChannelQualitySuspension(ctx, scope[0], scope[1], "held"); err != nil || enabled {
			t.Fatal("scope leak")
		}
	}
	settings, err := repo.ListChannelQualitySuspensions(ctx, "u", "w")
	if err != nil || len(settings) != 1 || settings["held"] {
		t.Fatal("bulk read lost preference")
	}
}
