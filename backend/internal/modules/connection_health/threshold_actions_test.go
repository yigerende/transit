package connection_health

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

func TestProbeTarget_ThresholdSuspensionAndFullRecovery(t *testing.T) {
	for _, platformName := range []upstream.Platform{upstream.PlatformSub2API, upstream.PlatformNewAPI} {
		for _, mode := range []string{"enabled", "suspension_off", "remote_off", "automation_off"} {
			t.Run(fmt.Sprintf("%s/%s", platformName, mode), func(t *testing.T) {
				repo := newFakeRepository()
				policy := sub2APIProbePolicy(true)
				policy.FailureThreshold, policy.SuccessThreshold = 5, 2
				policy.MaxLatencyMs = 20000
				policy.AutoSuspendEnabled = mode != "suspension_off"
				policy.AutoRemoteActionEnabled = mode != "remote_off"
				policy.AutoDegradeEnabled = mode != "automation_off"
				repo.policies = []Policy{policy}
				originalWeight := 37
				status := "active"
				if platformName == upstream.PlatformNewAPI {
					status = "1"
				}
				reader := fakePlatformGroupReader{
					groups:        []upstream.AdminGroupInfo{{ID: "g1", Name: "vip"}},
					accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{"g1": {{ID: "100", Status: status, Weight: &originalWeight, Models: "gpt-4o"}}},
					credByAccount: map[string]upstream.ProbeCredential{"100": {BaseURL: "https://probe.test", Key: "test"}},
				}
				actions := &fakePlatformActioner{}
				svc := newAdminTargetsRemoteActionService(reader, fakeMySitesReader{session: upstream.Session{Platform: platformName}}, repo, actions)
				attempt := 0
				svc.probeRunner.client.Transport = latencyTestTransport(func(r *http.Request) (*http.Response, error) {
					deadline, ok := r.Context().Deadline()
					if !ok || time.Until(deadline) < 19*time.Second {
						t.Fatal("configured 20s deadline was not forwarded")
					}
					attempt++
					if attempt <= 5 || attempt == 7 {
						return nil, context.DeadlineExceeded
					}
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"choices":[]}`))}, nil
				})
				targetID := string(platformName) + ":ws1:100"
				for step := 1; step <= 9; step++ {
					results, err := svc.ProbeTarget(context.Background(), "user1", targetID, []string{"gpt-4o"})
					if err != nil || len(results) != 1 {
						t.Fatalf("step %d: %v %+v", step, err, results)
					}
					wantState := StateHealthy
					if step >= 5 && step < 9 && mode != "automation_off" {
						wantState = StateSuspended
						if mode == "suspension_off" {
							wantState = StateDegraded
						}
					}
					if results[0].State != wantState {
						t.Fatalf("step %d: got %s, want %s", step, results[0].State, wantState)
					}
					expectedActions := 0
					if mode == "enabled" && step >= 5 {
						expectedActions = 1
					}
					if mode == "enabled" && step == 9 {
						expectedActions = 2
					}
					if len(actions.calls)+len(actions.sub2APICalls) != expectedActions {
						t.Fatalf("step %d: premature/repeated upstream action: %+v", step, actions)
					}
					st := repo.states[targetID]["gpt-4o"]
					if st.CooldownUntil != nil || st.ObservingUntil != nil {
						t.Fatal("obsolete timers must stay empty")
					}
					if len(actions.sub2APICalls) > 0 {
						reader.accountsByGrp["g1"][0].Status = actions.sub2APICalls[len(actions.sub2APICalls)-1].status
					}
					if len(actions.calls) > 0 {
						last := actions.calls[len(actions.calls)-1]
						reader.accountsByGrp["g1"][0].Status = fmt.Sprint(last.status)
						weight := last.weight
						reader.accountsByGrp["g1"][0].Weight = &weight
					}
				}
				if platformName == upstream.PlatformNewAPI && mode == "enabled" && actions.calls[1].weight != originalWeight {
					t.Fatal("must restore exact original weight in one action")
				}
			})
		}
	}
}

func TestReconcileTargetRemoteAction_LegacyZeroWeightCannotSuspend(t *testing.T) {
	repo := newFakeRepository()
	platform := &fakePlatformActioner{}
	svc := &Service{repo: repo, dispatcher: newRemoteActionDispatcher(nil, nil, platform)}
	targetID := "sub2api:ws1:100"
	repo.states[targetID] = map[string]ConnectionHealthState{"model": {ConnectionID: targetID, ModelName: "model", State: StateDegraded, CurrentWeight: 0, ConsecutiveFailures: 4}}
	policy := sub2APIProbePolicy(true)
	policy.FailureThreshold = 5
	target := AdminProbeTarget{TargetID: targetID, Platform: string(upstream.PlatformSub2API), AccountID: "100", AccountStatus: "active"}
	action, err := svc.reconcileTargetRemoteAction(context.Background(), "user1", "ws1", upstream.Session{Platform: upstream.PlatformSub2API}, target, []probeModelSpec{{modelName: "model", policy: policy}})
	if err != nil || action != "" || len(platform.sub2APICalls) != 0 {
		t.Fatalf("legacy weight must not bypass threshold: %q %v %+v", action, err, platform)
	}
}
