package connection_health

import (
	"context"
	"errors"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

func qualityHealthPauseFixture(t *testing.T) (*Service, *fakeQualityRepo, *fakeRepository, *fakeQuestionRunner) {
	t.Helper()
	svc, quality, health, runner := qualityTestService(t)
	enableManualQuality(t, svc)
	health.policies[0].AutoSuspendEnabled = true
	health.policies[0].AutoDegradeEnabled = true
	health.policies[0].ModelTargets = []ModelTarget{{ModelName: "health-model", Enabled: true}}
	return svc, quality, health, runner
}

func setQualityTestHealth(health *fakeRepository, target string, state State) {
	health.states[target] = map[string]ConnectionHealthState{
		"health-model": {ConnectionID: target, ModelName: "health-model", UserID: "user", AdminAccountID: "ws1", OwnGroupID: "one", State: state, ConsecutiveFailures: 5},
	}
}

func TestQualityFollowsHealthSuspensionAcrossGroupsAndResumes(t *testing.T) {
	svc, quality, health, runner := qualityHealthPauseFixture(t)
	ctx := context.Background()
	const target = "sub2api:ws1:a"
	if _, err := svc.ProbeChannelQuality(ctx, "user", target, "one", "questions"); err != nil {
		t.Fatal(err)
	}
	// Keep the next automatic check due throughout suspension and recovery.
	st := quality.states[qualityScopeKey("user", "ws1")][target]
	st.NextProbeAt = time.Now().Add(-time.Second)
	quality.states[qualityScopeKey("user", "ws1")][target] = st
	setQualityTestHealth(health, target, StateSuspended)
	scope := QualityScope{UserID: "user", WorkspaceID: "ws1"}
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	for _, group := range []string{"one", "two"} {
		for _, method := range []string{"questions", "manxue_candy", "manxue_pelican"} {
			if _, err := svc.ProbeChannelQuality(ctx, "user", target, group, method); err != requestError(qualityPrefix+"healthPausedHint") {
				t.Fatalf("%s/%s bypassed health suspension: %v", group, method, err)
			}
		}
	}
	groups, err := svc.AdminGroups(ctx, "user")
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		for _, account := range group.Accounts {
			if account.TargetID == target && (!account.QualityPausedByHealth || !account.QualityEnabled || !account.QualitySelected || len(account.QualityHistory) != 1) {
				t.Fatal("pause lost selection, switch, history or shared channel display")
			}
		}
	}
	if runner.calls != 1 || len(quality.history[qualityScopeKey("user", "ws1")]) != 1 || len(quality.channels) != 0 {
		t.Fatal("suspended channel ran or rewrote saved preference/history")
	}
	// A success below the recovery threshold still leaves the health state suspended.
	partial := health.states[target]["health-model"]
	partial.ConsecutiveSuccesses = 1
	partial.ConsecutiveFailures = 0
	health.states[target]["health-model"] = partial
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	if runner.calls != 1 {
		t.Fatal("quality resumed before health recovery")
	}
	setQualityTestHealth(health, target, StateHealthy)
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	if runner.calls != 2 || len(quality.history[qualityScopeKey("user", "ws1")]) != 2 {
		t.Fatal("quality did not automatically resume with retained history")
	}
	groups, _ = svc.AdminGroups(ctx, "user")
	for _, group := range groups {
		for _, account := range group.Accounts {
			if account.TargetID == target && account.QualityPausedByHealth {
				t.Fatal("recovered channel still displayed as paused")
			}
		}
	}
}

func TestQualityHealthPauseIgnoresStaleAndNonSuspendingStates(t *testing.T) {
	for _, scenario := range []string{"healthy", "degraded", "channel-opt-out", "policy-opt-out", "disabled-policy", "disabled-model", "removed-model", "removed-assignment", "excluded-source", "foreign-workspace", "foreign-user", "explicit-assignment", "legacy-observing"} {
		t.Run(scenario, func(t *testing.T) {
			svc, _, health, runner := qualityHealthPauseFixture(t)
			const target = "sub2api:ws1:a"
			setQualityTestHealth(health, target, StateSuspended)
			state := health.states[target]["health-model"]
			switch scenario {
			case "healthy":
				state.State = StateHealthy
			case "degraded":
				state.State = StateDegraded
			case "channel-opt-out":
				_ = health.SetChannelSuspension(context.Background(), "user", "ws1", target, false)
			case "policy-opt-out":
				health.policies[0].AutoSuspendEnabled = false
			case "disabled-policy":
				health.policies[0].Enabled = false
			case "disabled-model":
				health.policies[0].ModelTargets[0].Enabled = false
			case "removed-model":
				health.policies[0].ModelTargets = nil
			case "removed-assignment":
				health.groupAssignments = health.groupAssignments[1:]
			case "excluded-source":
				health.groupExclusions = []GroupTargetExclusion{{UserID: "user", AdminAccountID: "ws1", AdminGroupID: "one", TargetID: target}}
			case "foreign-workspace":
				state.AdminAccountID = "other"
			case "foreign-user":
				state.UserID = "other"
			case "explicit-assignment":
				state.OwnGroupID = ""
				health.assignments = []PolicyAssignment{{UserID: "user", AdminAccountID: "ws1", TargetID: target, PolicyID: health.policies[0].ID}}
			case "legacy-observing":
				state.State = StateObserving
			}
			health.states[target]["health-model"] = state
			_, err := svc.ProbeChannelQuality(context.Background(), "user", target, "two", "questions")
			paused := scenario == "explicit-assignment" || scenario == "legacy-observing"
			if paused && (err == nil || runner.calls != 0) || !paused && (err != nil || runner.calls != 1) {
				t.Fatalf("unexpected suspension gate: calls=%d err=%v", runner.calls, err)
			}
		})
	}
}

type qualityHealthCredentialHook struct {
	fakePlatformGroupReader
	hook func()
}

func (r qualityHealthCredentialHook) ResolveProbeCredential(session upstream.Session, account upstream.AdminGroupAccountInfo) (upstream.ProbeCredential, error) {
	r.hook()
	return r.fakePlatformGroupReader.ResolveProbeCredential(session, account)
}

func TestQualityRechecksHealthBeforeRequestsAndSavingResults(t *testing.T) {
	for _, stage := range []string{"credentials", "in-flight", "queued"} {
		t.Run(stage, func(t *testing.T) {
			svc, quality, health, runner := qualityHealthPauseFixture(t)
			ctx := context.Background()
			q, _ := svc.QualityConfiguration(ctx, "user")
			q.Concurrency = 1
			_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
			target := "sub2api:ws1:a"
			pause := func() { setQualityTestHealth(health, target, StateSuspended) }
			if stage == "credentials" {
				svc.platformGroups = qualityHealthCredentialHook{svc.platformGroups.(fakePlatformGroupReader), pause}
			} else {
				if stage == "queued" {
					target = "sub2api:ws1:b"
					reader := svc.platformGroups.(fakePlatformGroupReader)
					reader.accountsByGrp["one"] = append(reader.accountsByGrp["one"], upstream.AdminGroupAccountInfo{ID: "b"})
					reader.credByAccount["b"] = reader.credByAccount["a"]
					svc.platformGroups = reader
				}
				runner.hook = pause
			}
			svc.runQualityScope(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
			wantCalls := 1
			if stage == "credentials" {
				wantCalls = 0
			}
			if runner.calls != wantCalls {
				t.Fatalf("suspended channel requested: got %d want %d", runner.calls, wantCalls)
			}
			for _, sample := range quality.history[qualityScopeKey("user", "ws1")] {
				if sample.TargetID == target {
					t.Fatal("late result from suspended channel was saved")
				}
			}
		})
	}
}

type unavailableQualityHealthRepo struct{ healthRepository }

func (r unavailableQualityHealthRepo) ListStatesByWorkspace(context.Context, string, string) ([]ConnectionHealthState, error) {
	return nil, errors.New("health state unavailable")
}

func TestQualityDoesNotRunWithUnknownHealthSuspension(t *testing.T) {
	svc, quality, _, runner := qualityHealthPauseFixture(t)
	svc.repo = unavailableQualityHealthRepo{svc.repo}
	svc.runQualityScope(context.Background(), QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
	if _, err := svc.ProbeChannelQuality(context.Background(), "user", "sub2api:ws1:a", "one", "questions"); err == nil || runner.calls != 0 || len(quality.history) != 0 {
		t.Fatal("unknown health status allowed a quality check")
	}
}
