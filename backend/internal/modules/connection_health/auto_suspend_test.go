package connection_health

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

func TestTransition_SuspensionOffOnlyChangesHealth(t *testing.T) {
	for _, result := range []ResultKey{ResultServerError, ResultAuth, ResultModelNotFound, ResultRateLimited, ResultNetworkFluctuation, ResultInvalidResponse} {
		t.Run(string(result), func(t *testing.T) {
			in := TransitionInput{Current: StateHealthy, CurrentWeight: 100, Result: result, Now: time.Now(), Policy: Policy{FailureThreshold: 1, RecoveryStepPercent: 100}}
			for attempt := 1; attempt <= 8; attempt++ {
				out := Transition(in)
				if out.NextState != StateDegraded || out.Weight != 100 || out.CooldownUntil != nil || out.ObservingUntil != nil || out.TriggerRemoteDegrade || out.TriggerRemoteRestore || out.ConsecutiveFailures != attempt {
					t.Fatalf("attempt %d must only degrade health: %+v", attempt, out)
				}
				in.Current, in.CurrentWeight, in.ConsecutiveFailures = out.NextState, out.Weight, out.ConsecutiveFailures
			}
			in.Result = ResultOK
			for successes := 1; successes <= 2; successes++ {
				out := Transition(in)
				if out.TriggerRemoteRestore || out.Weight != 100 || (successes == 2 && out.NextState != StateHealthy) {
					t.Fatalf("priority recovery must not change upstream: %+v", out)
				}
				in.Current, in.ConsecutiveSuccesses = out.NextState, out.ConsecutiveSuccesses
			}
		})
	}
}

func TestSuspensionOffClearsOldStatesAndKeepsManualDisable(t *testing.T) {
	for _, old := range []State{StateSuspended, StateObserving, StateRecovering, StateDisabled} {
		t.Run(string(old), func(t *testing.T) {
			repo := newFakeRepository()
			future := time.Now().Add(time.Hour)
			repo.states["target"] = map[string]ConnectionHealthState{"model": {ConnectionID: "target", ModelName: "model", State: old, CooldownUntil: &future, ObservingUntil: &future}}
			svc := &Service{repo: repo}
			models, _ := modelHealthForSpecs(repo.states["target"], []probeModelSpec{{modelName: "model"}})
			if old == StateDisabled {
				if models[0].State != StateDisabled || svc.isDue(context.Background(), "target", "model", Policy{}, time.Now()) {
					t.Fatal("manual disable must remain disabled")
				}
				return
			}
			if models[0].State != StateDegraded || !svc.isDue(context.Background(), "target", "model", Policy{}, time.Now()) {
				t.Fatal("old suspension must not block display/scheduling")
			}
			stored := repo.states["target"]["model"]
			if stored.State != StateDegraded || stored.CurrentWeight != 100 || stored.CooldownUntil != nil || stored.ObservingUntil != nil {
				t.Fatalf("old suspension was not cleared: %+v", stored)
			}
		})
	}
}

func TestProbeTarget_SuspensionPermissionAndInFlightRevocation(t *testing.T) {
	for _, platformName := range []upstream.Platform{upstream.PlatformSub2API, upstream.PlatformNewAPI} {
		for _, mode := range []string{"default_off", "enabled", "revoked_in_flight"} {
			t.Run(fmt.Sprintf("%s/%s", platformName, mode), func(t *testing.T) {
				repo := newFakeRepository()
				policy := sub2APIProbePolicy(true)
				policy.AutoSuspendEnabled = mode != "default_off"
				repo.policies = []Policy{policy}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if mode == "revoked_in_flight" {
						repo.policies[0].AutoSuspendEnabled = false
					}
					w.WriteHeader(http.StatusUnauthorized)
				}))
				defer server.Close()
				status := "active"
				if platformName == upstream.PlatformNewAPI {
					status = "1"
				}
				weight := 37
				reader := fakePlatformGroupReader{
					groups:        []upstream.AdminGroupInfo{{ID: "g1", Name: "vip"}},
					accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{"g1": {{ID: "100", Name: "channel", Status: status, Weight: &weight, Models: "gpt-4o", BaseURL: server.URL}}},
					credByAccount: map[string]upstream.ProbeCredential{"100": {BaseURL: server.URL, Key: "test"}},
				}
				actions := &fakePlatformActioner{}
				svc := newAdminTargetsRemoteActionService(reader, fakeMySitesReader{session: upstream.Session{Platform: platformName}}, repo, actions)
				results, err := svc.ProbeTarget(context.Background(), "user1", string(platformName)+":ws1:100", []string{"gpt-4o"})
				if err != nil || len(results) != 1 {
					t.Fatalf("probe failed: %v %+v", err, results)
				}
				if mode == "enabled" {
					if results[0].State != StateSuspended || len(actions.calls)+len(actions.sub2APICalls) != 1 {
						t.Fatal("explicit opt-in must allow suspension")
					}
				} else if results[0].State != StateDegraded || len(actions.calls)+len(actions.sub2APICalls) != 0 {
					t.Fatalf("suspension off must block upstream changes: %+v %+v", results, actions)
				}
			})
		}
	}
}

func TestSuspensionOffRestoresPreviouslyManagedChannel(t *testing.T) {
	repo := newFakeRepository()
	policy := sub2APIProbePolicy(true)
	policy.AutoSuspendEnabled = false
	targetID := "sub2api:ws1:100"
	stored := TargetActionState{UserID: "user1", AdminAccountID: "ws1", TargetID: targetID, OriginalStatus: "active", LastAppliedStatus: "inactive"}
	repo.targetActionStates["user1|ws1|"+targetID] = stored
	reader := fakePlatformGroupReader{
		groups:        []upstream.AdminGroupInfo{{ID: "g1", Name: "vip"}},
		accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{"g1": {{ID: "100", Status: "inactive", Models: "gpt-4o"}}},
	}
	actions := &fakePlatformActioner{}
	svc := newAdminTargetsRemoteActionService(reader, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API}}, repo, actions)
	svc.restoreUnmanagedTargetActions(context.Background(), []Policy{policy}, nil, []GroupPolicyAssignment{{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", PolicyID: policy.ID}}, nil, []TargetActionState{stored}, adminInventoryCache{})
	if len(actions.sub2APICalls) != 1 || actions.sub2APICalls[0].status != "active" || len(repo.targetActionStates) != 0 {
		t.Fatalf("turning suspension off must restore the captured state: %+v", actions)
	}
}

func TestSavePolicySuspensionDefaultAndRoundTrip(t *testing.T) {
	repo := newFakeRepository()
	svc := &Service{repo: repo, accounts: fakeAdminAccountResolver{id: "ws1"}}
	input := PolicyInput{Name: "priority only", Enabled: true, AutoDegradeEnabled: true, AutoRemoteActionEnabled: true}
	saved, err := svc.SavePolicy(context.Background(), "user1", input)
	if err != nil || saved.AutoSuspendEnabled || policyRemoteActionEnabled(saved) {
		t.Fatalf("default must deny suspension: %v %+v", err, saved)
	}
	input.ID, input.AutoSuspendEnabled = saved.ID, true
	saved, err = svc.SavePolicy(context.Background(), "user1", input)
	if err != nil || !saved.AutoSuspendEnabled || !policyRemoteActionEnabled(saved) {
		t.Fatalf("explicit opt-in lost: %v %+v", err, saved)
	}
	input.AutoSuspendEnabled = false
	saved, err = svc.SavePolicy(context.Background(), "user1", input)
	if err != nil || saved.AutoSuspendEnabled || policyRemoteActionEnabled(saved) {
		t.Fatalf("opt-out lost: %v %+v", err, saved)
	}
}

func TestSuspensionOffStillUpdatesPriority(t *testing.T) {
	repo := newFakeRepository()
	priorityActions := &fakeTargetPriorityActioner{}
	currentPriority := 1
	reader := fakePlatformGroupReader{
		groups:        []upstream.AdminGroupInfo{{ID: "g1", Name: "vip", Multiplier: float64Ptr(0.4)}},
		accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{"g1": {{ID: "100", Priority: &currentPriority, Models: "gpt-4o"}}},
	}
	svc := &Service{repo: repo, mySites: fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformNewAPI}}, platformGroups: reader, priorityActions: priorityActions}
	policy := sub2APIProbePolicy(true)
	policy.AutoSuspendEnabled = false
	policy.PriorityMode = PriorityModeMultiplier
	targetID := "newapi:ws1:100"
	repo.states[targetID] = map[string]ConnectionHealthState{"gpt-4o": {ConnectionID: targetID, ModelName: "gpt-4o", State: StateSuspended, CurrentWeight: 0, UserID: "user1", AdminAccountID: "ws1"}}
	svc.syncMultiplierPriorities(context.Background(), []Policy{policy}, nil, []GroupPolicyAssignment{{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", PolicyID: policy.ID}}, nil, nil)
	want := desiredManagedPriorityForPlatformWithExpected(upstream.PlatformNewAPI, []ConnectionHealthState{{State: StateDegraded, CurrentWeight: 100}}, 0, 1)
	if len(priorityActions.calls) != 1 || priorityActions.calls[0].priority != want {
		t.Fatalf("suspension off must still sync degraded priority, want=%d calls=%+v", want, priorityActions.calls)
	}
}

func TestSuspensionOffWinsOverOverlappingPolicies(t *testing.T) {
	allowed := sub2APIProbePolicy(true)
	denied := allowed
	denied.ID, denied.AutoSuspendEnabled = "deny", false
	if !preferProbePolicy(denied, allowed) || preferProbePolicy(allowed, denied) {
		t.Fatal("overlapping model must prefer suspension off")
	}
	repo := newFakeRepository()
	platform := &fakePlatformActioner{}
	svc := &Service{repo: repo, dispatcher: newRemoteActionDispatcher(nil, nil, platform)}
	target := AdminProbeTarget{TargetID: "sub2api:ws1:100", Platform: "sub2api", AccountID: "100", AccountStatus: "active"}
	repo.states[target.TargetID] = map[string]ConnectionHealthState{"allowed": {ConnectionID: target.TargetID, ModelName: "allowed", State: StateSuspended}}
	specs := []probeModelSpec{{modelName: "allowed", policy: allowed}, {modelName: "denied", policy: denied}}
	if hasRemoteActionModel(specs) {
		t.Fatal("an overlapping opt-out must release status management")
	}
	_, err := svc.reconcileTargetRemoteAction(context.Background(), "user1", "ws1", upstream.Session{Platform: upstream.PlatformSub2API}, target, specs)
	if err != nil || len(platform.sub2APICalls) != 0 {
		t.Fatal("a different model must not bypass suspension off")
	}
}
