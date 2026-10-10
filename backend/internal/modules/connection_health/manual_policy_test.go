package connection_health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"
	"transithub/backend/internal/modules/upstream"
)

func TestManualPolicyProbeSharesThresholdsSourceActionsAndHistory(t *testing.T) {
	ctx := context.Background()
	failing := true
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if failing {
			w.WriteHeader(500)
			fmt.Fprint(w, `{"error":"temporarily unavailable"}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer server.Close()
	repo := newFakeRepository()
	p := sub2APIProbePolicy(true)
	p.FailureThreshold, p.SuccessThreshold = 2, 2
	disabled := p
	disabled.ID, disabled.Enabled = "disabled", false
	repo.policies = []Policy{disabled, p}
	repo.groupAssignments = []GroupPolicyAssignment{
		{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "first", PolicyID: disabled.ID},
		{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "second", PolicyID: p.ID},
	}
	const target = "sub2api:ws1:a"
	// Manual probes must still apply policy thresholds and history after automatic probes are disabled.
	_ = repo.SetChannelAutoProbe(ctx, "user1", "ws1", target, false)
	reader := fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "first", Name: "disabled"}, {ID: "second", Name: "active"}}, accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{
		"first": {{ID: "a", Status: "active", Models: "gpt-4o"}}, "second": {{ID: "a", Status: "active", Models: "gpt-4o"}},
	}, credByAccount: map[string]upstream.ProbeCredential{"a": {BaseURL: server.URL, Key: "secret"}}}
	platform := &fakePlatformActioner{}
	svc := newAdminTargetsRemoteActionService(reader, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API}}, repo, platform)
	// Legacy state was written through the first group, which is now disabled.
	repo.states[target] = map[string]ConnectionHealthState{"gpt-4o": {ConnectionID: target, UserID: "user1", AdminAccountID: "ws1", ModelName: "gpt-4o", OwnGroupID: "first", State: StateHealthy, CurrentWeight: 100, ConsecutiveFailures: 1}}
	out, err := svc.manualProbeTarget(ctx, "user1", target, []string{"gpt-4o", " gpt-4o "}, false)
	st := repo.states[target]["gpt-4o"]
	if err != nil || len(out) != 1 || requests != 1 || st.State != StateSuspended || st.ConsecutiveFailures != 2 || st.OwnGroupID != "second" || st.UpstreamGroupID != "second" {
		t.Fatalf("manual failure/source: %+v %+v %v", out, st, err)
	}
	if len(platform.sub2APICalls) != 1 || platform.sub2APICalls[0].status != "inactive" {
		t.Fatalf("missing suspension: %+v", platform.sub2APICalls)
	}
	paused := qualityHealthPausedTargets([]ConnectionHealthState{st}, repo.policies, nil, repo.groupAssignments, nil, nil)
	if !paused[target] {
		t.Fatal("quality bypassed suspension through disabled first group")
	}
	if len(repo.events) != 1 || !repo.events[0].Manual || repo.events[0].PolicyID != p.ID || repo.events[0].AdminGroupID != "second" {
		t.Fatal("manual policy history missing")
	}
	failing = false
	for _, group := range []string{"first", "second"} {
		reader.accountsByGrp[group][0].Status = "inactive"
	}
	for i := 1; i <= 2; i++ {
		out, err = svc.ManualProbeTarget(ctx, "user1", target, []string{"gpt-4o"})
		if err != nil || !out[0].Healthy {
			t.Fatalf("recovery probe: %v", err)
		}
		if i == 1 && repo.states[target]["gpt-4o"].State != StateSuspended {
			t.Fatal("recovered before success threshold")
		}
	}
	if repo.states[target]["gpt-4o"].State != StateHealthy || len(platform.sub2APICalls) != 2 || platform.sub2APICalls[1].status != "active" {
		t.Fatal("manual successes did not restore")
	}
	samples, err := repo.ListPriorityProbeSamples(ctx, "user1", "ws1", []string{target}, time.Now().Add(-time.Hour))
	if err != nil || len(samples) != 2 {
		t.Fatalf("manual successes missing from priority samples: %d %v", len(samples), err)
	}
	if s := repo.events[2]; !s.Manual || s.PolicyID != p.ID {
		t.Fatal("manual recovery event missing")
	}
}

func TestManualPolicyProbeModesModelsAndBudget(t *testing.T) {
	for _, mode := range []string{ProbeModeLight, ProbeModeArithmetic, ProbeModeFirstToken, ProbeModeSub2API} {
		t.Run(mode, func(t *testing.T) {
			svc, repo, reader := cadenceFixture(1)
			svc.accounts = fakeAdminAccountResolver{id: "ws1"}
			svc.dispatcher = panicIfCalledRemoteActionRunner{}
			repo.policies[0].ProbeMode = mode
			repo.policies[0].DailyProbeBudget = 1
			repo.policies[0].ModelTargets[0].ProbePrompt = "configured prompt"
			calls := 0
			svc.probeRunner.client.Transport = latencyTestTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case ProbeModeSub2API:
					if r.URL.Path != "/api/v1/admin/accounts/0/test" {
						t.Fatal(r.URL.Path)
					}
				case ProbeModeFirstToken:
					if body["stream"] != true {
						t.Fatal("manual probe ignored streaming policy")
					}
				case ProbeModeLight:
					data, _ := json.Marshal(body)
					if !strings.Contains(string(data), "configured prompt") {
						t.Fatal("manual ignored prompt")
					}
				}
				return nil, context.DeadlineExceeded
			})
			// Native testing must work without credential export or /v1/models.
			if mode == ProbeModeSub2API {
				svc.mySites = fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API, BaseURL: "https://admin.test", AdminAPIKey: "admin"}}
				reader.credErr = map[string]error{"0": &upstream.ProbeCredentialError{Reason: upstream.ReasonCredentialUnavailable}}
			}
			models, err := svc.DiscoverTargetModels(context.Background(), "user1", "sub2api:ws1:0")
			if err != nil || len(models) != 1 || models[0].ProbeMode != mode || reader.credentials.Load() != 0 {
				t.Fatalf("configured models: %+v %v", models, err)
			}
			if _, err = svc.ManualProbeTarget(context.Background(), "user1", "sub2api:ws1:0", []string{"outside-policy"}); err == nil || calls != 0 {
				t.Fatal("unconfigured model bypassed policy")
			}
			results, err := svc.ManualProbeTarget(context.Background(), "user1", "sub2api:ws1:0", []string{"model-0"})
			if err != nil || len(results) != 1 || results[0].ProbeMode != mode || calls != 1 || len(repo.events) != 1 || !repo.events[0].Manual {
				t.Fatalf("mode ignored: %+v calls=%d err=%v", results, calls, err)
			}
			results, err = svc.ManualProbeTarget(context.Background(), "user1", "sub2api:ws1:0", []string{"model-0"})
			if err != nil || results[0].ErrorKey != "admin.connectionHealth.errors.probeBudgetExhausted" || calls != 1 || len(repo.events) != 1 {
				t.Fatal("budget bypassed or fabricated history")
			}
			if mode == ProbeModeSub2API && reader.credentials.Load() != 0 {
				t.Fatal("native mode exported credentials")
			}
		})
	}
}

func TestManualProbeHonorsChannelAndPolicyActionSwitches(t *testing.T) {
	for _, scenario := range []string{"channel-off", "policy-pause-off", "remote-off", "degrade-off", "revoked-in-flight"} {
		t.Run(scenario, func(t *testing.T) {
			svc, repo, _ := cadenceFixture(1)
			svc.accounts = fakeAdminAccountResolver{id: "ws1"}
			svc.dispatcher = panicIfCalledRemoteActionRunner{}
			p := &repo.policies[0]
			p.AutoSuspendEnabled, p.AutoRemoteActionEnabled, p.FailureThreshold = true, true, 1
			const target = "sub2api:ws1:0"
			switch scenario {
			case "channel-off":
				_ = repo.SetChannelSuspension(context.Background(), "user1", "ws1", target, false)
			case "policy-pause-off":
				p.AutoSuspendEnabled = false
			case "remote-off":
				p.AutoRemoteActionEnabled = false
			case "degrade-off":
				p.AutoDegradeEnabled = false
			}
			svc.probeRunner.client.Transport = latencyTestTransport(func(r *http.Request) (*http.Response, error) {
				if scenario == "revoked-in-flight" {
					_ = repo.SetChannelSuspension(context.Background(), "user1", "ws1", target, false)
				}
				return nil, context.DeadlineExceeded
			})
			out, err := svc.ManualProbeTarget(context.Background(), "user1", target, []string{"model-0"})
			if err != nil || len(out) != 1 || len(repo.events) != 1 {
				t.Fatalf("manual result: %v", err)
			}
			state := repo.states[target]["model-0"]
			if state.ConsecutiveFailures != 1 {
				t.Fatal("manual failure was not counted")
			}
			want := StateDegraded
			if scenario == "remote-off" {
				want = StateSuspended
			}
			if scenario == "degrade-off" {
				want = StateHealthy
			}
			if state.State != want {
				t.Fatalf("switch ignored: %s want %s", state.State, want)
			}
		})
	}
}

func TestManualProbeSerializesWithQueuedAutomaticProbe(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, repo, _ := cadenceFixture(1)
		svc.accounts = fakeAdminAccountResolver{id: "ws1"}
		svc.dispatcher = panicIfCalledRemoteActionRunner{}
		svc.repo = &manualLeaseTestRepo{fakeRepository: repo.fakeRepository, gate: make(chan struct{}, 1)}
		jobs := svc.collectAdminProbeJobsWithGroups(context.Background(), repo.policies, nil, repo.groupAssignments, nil)
		if len(jobs) != 1 {
			t.Fatal("missing automatic job")
		}
		started, unblock := make(chan struct{}), make(chan struct{})
		calls := 0
		svc.probeRunner.client.Transport = latencyTestTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			close(started)
			<-unblock
			return nil, context.DeadlineExceeded
		})
		manualDone, autoDone := make(chan error, 1), make(chan struct{})
		go func() {
			_, err := svc.ManualProbeTarget(context.Background(), "user1", "sub2api:ws1:0", []string{"model-0"})
			manualDone <- err
		}()
		<-started
		go func() { svc.runAdminProbeJob(context.Background(), jobs[0]); close(autoDone) }()
		synctest.Wait()
		if calls != 1 {
			t.Fatal("automatic probe overlapped manual request")
		}
		close(unblock)
		if err := <-manualDone; err != nil {
			t.Fatal(err)
		}
		<-autoDone
		if calls != 1 || len(repo.events) != 1 || !repo.events[0].Manual {
			t.Fatal("queued automatic probe duplicated manual result")
		}
	})
}

// A channel-backed lease is durably blocked under Go's virtual test clock.
type manualLeaseTestRepo struct {
	*fakeRepository
	gate chan struct{}
}

func (r *manualLeaseTestRepo) AcquireTargetLease(ctx context.Context, key string) (func(), error) {
	if strings.HasPrefix(key, "suspension:") {
		return func() {}, nil
	}
	select {
	case r.gate <- struct{}{}:
		return func() { <-r.gate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
