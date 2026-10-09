package connection_health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"transithub/backend/internal/modules/upstream"
)

// Exercise real scheduler goroutines using a virtual clock and an in-memory DB.
type cadenceRepo struct {
	*fakeRepository
	mu sync.Mutex
}

func (r *cadenceRepo) GetState(ctx context.Context, target, model string) (*ConnectionHealthState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeRepository.GetState(ctx, target, model)
}
func (r *cadenceRepo) UpsertState(ctx context.Context, state ConnectionHealthState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeRepository.UpsertState(ctx, state)
}
func (r *cadenceRepo) InsertEvent(ctx context.Context, event ConnectionHealthEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeRepository.InsertEvent(ctx, event)
}
func (r *cadenceRepo) CountProbesToday(ctx context.Context, user, workspace, policy string, day time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeRepository.CountProbesToday(ctx, user, workspace, policy, day)
}
func (r *cadenceRepo) TryConsumeProbeBudget(ctx context.Context, user, workspace, policy string, day time.Time, limit int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fakeRepository.TryConsumeProbeBudget(ctx, user, workspace, policy, day, limit)
}

type cadenceReader struct {
	fakePlatformGroupReader
	inventories   atomic.Int32
	credentials   atomic.Int32
	slowInventory bool
}

func (r *cadenceReader) FetchAdminAllGroups(session upstream.Session) ([]upstream.AdminGroupInfo, error) {
	if r.inventories.Add(1) == 2 && r.slowInventory {
		time.Sleep(65 * time.Second)
	}
	return r.fakePlatformGroupReader.FetchAdminAllGroups(session)
}
func (r *cadenceReader) ResolveProbeCredential(session upstream.Session, account upstream.AdminGroupAccountInfo) (upstream.ProbeCredential, error) {
	r.credentials.Add(1)
	return r.fakePlatformGroupReader.ResolveProbeCredential(session, account)
}

func cadenceFixture(count int) (*Service, *cadenceRepo, *cadenceReader) {
	repo := &cadenceRepo{fakeRepository: newFakeRepository()}
	reader := &cadenceReader{fakePlatformGroupReader: fakePlatformGroupReader{
		groups: []upstream.AdminGroupInfo{{ID: "g1", Name: "group"}}, accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{}, credByAccount: map[string]upstream.ProbeCredential{},
	}}
	policy := Policy{ID: "p1", UserID: "user1", AdminAccountID: "ws1", Enabled: true, ProbeIntervalSeconds: 60, MaxLatencyMs: 20000, FailureThreshold: 5, SuccessThreshold: 1, AutoDegradeEnabled: true, DailyProbeBudget: 1000}
	for i := 0; i < count; i++ {
		id := fmt.Sprint(i)
		model := "model-" + id
		reader.accountsByGrp["g1"] = append(reader.accountsByGrp["g1"], upstream.AdminGroupAccountInfo{ID: id, Status: "active", Models: model})
		reader.credByAccount[id] = upstream.ProbeCredential{BaseURL: "https://probe.test", Key: "test"}
		policy.ModelTargets = append(policy.ModelTargets, ModelTarget{ModelName: model, Enabled: true})
	}
	repo.policies = []Policy{policy}
	repo.groupAssignments = []GroupPolicyAssignment{{UserID: "user1", AdminAccountID: "ws1", PolicyID: "p1", AdminGroupID: "g1"}}
	svc := &Service{repo: repo, mySites: fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API}}, platformGroups: reader, probeRunner: NewRealProbeRunner()}
	return svc, repo, reader
}

func TestScheduler_TimeoutsRepeatAt60SecondsDespiteSlowInventory(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, repo, reader := cadenceFixture(3)
		reader.slowInventory = true
		repo.policies[0].AutoSuspendEnabled = true
		var mu sync.Mutex
		starts := map[string][]time.Time{}
		svc.probeRunner.client.Transport = latencyTestTransport(func(r *http.Request) (*http.Response, error) {
			var body struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				panic(err)
			}
			mu.Lock()
			starts[body.Model] = append(starts[body.Model], time.Now())
			mu.Unlock()
			<-r.Context().Done()
			return nil, r.Context().Err()
		})
		ctx, cancel := context.WithCancel(context.Background())
		svc.StartScheduler(ctx)
		time.Sleep(321 * time.Second)
		synctest.Wait()
		cancel()
		synctest.Wait()
		for model, times := range starts {
			if len(times) != 6 {
				t.Fatalf("%s: expected six starts including suspended probes, got %v", model, times)
			}
			for i := 1; i < len(times); i++ {
				gap := times[i].Sub(times[i-1])
				if gap < 60*time.Second || gap > 61*time.Second {
					t.Fatalf("%s: gap %s, want 60s", model, gap)
				}
			}
		}
		if len(starts) != 3 {
			t.Fatalf("expected all three channels, got %v", starts)
		}
		if reader.inventories.Load() > 12 {
			t.Fatal("fast scans must not repeatedly fetch upstream inventory")
		}
	})
}

func TestScheduler_SlowChannelDoesNotBlockOthersOrOverlap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc, repo, _ := cadenceFixture(2)
		repo.policies[0].MaxLatencyMs = 120000
		starts := make(chan string, 10)
		svc.probeRunner.client.Transport = latencyTestTransport(func(r *http.Request) (*http.Response, error) {
			var body struct {
				Model string `json:"model"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			starts <- body.Model
			if body.Model == "model-0" {
				<-r.Context().Done()
				return nil, r.Context().Err()
			}
			timer := time.NewTimer(20 * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				return nil, context.DeadlineExceeded
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		})
		ctx, cancel := context.WithCancel(context.Background())
		svc.StartScheduler(ctx)
		time.Sleep(119 * time.Second)
		synctest.Wait()
		cancel()
		synctest.Wait()
		close(starts)
		counts := map[string]int{}
		for model := range starts {
			counts[model]++
		}
		if counts["model-0"] != 1 || counts["model-1"] != 2 {
			t.Fatalf("slow task blocked or overlapped: %v", counts)
		}
	})
}

func TestScheduler_DisabledOrExcludedQueuedWorkNeverRequestsCredentials(t *testing.T) {
	for _, change := range []string{"disabled", "excluded", "unbound", "model_disabled", "already_probed", "budget"} {
		t.Run(change, func(t *testing.T) {
			svc, repo, reader := cadenceFixture(1)
			jobs := svc.collectAdminProbeJobsWithGroups(context.Background(), repo.policies, nil, repo.groupAssignments, nil)
			if len(jobs) != 1 {
				t.Fatal("missing initial job")
			}
			switch change {
			case "disabled":
				repo.policies[0].Enabled = false
			case "excluded":
				repo.groupExclusions = []GroupTargetExclusion{{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", TargetID: jobs[0].target.TargetID}}
			case "unbound":
				repo.groupAssignments = nil
			case "model_disabled":
				repo.policies[0].ModelTargets[0].Enabled = false
			case "already_probed":
				now := time.Now()
				_ = repo.UpsertState(context.Background(), ConnectionHealthState{ConnectionID: jobs[0].target.TargetID, ModelName: "model-0", State: StateHealthy, LastProbeAt: &now})
			case "budget":
				repo.policies[0].DailyProbeBudget = 1
				_, _ = repo.TryConsumeProbeBudget(context.Background(), "user1", "ws1", "p1", probeBudgetDayStart(time.Now()), 1)
			}
			requests := 0
			svc.probeRunner.client.Transport = latencyTestTransport(func(r *http.Request) (*http.Response, error) { requests++; return nil, context.DeadlineExceeded })
			svc.runAdminProbeJob(context.Background(), jobs[0])
			if requests != 0 || (change != "budget" && reader.credentials.Load() != 0) {
				t.Fatalf("queued %s task still probed: requests %d credentials %d", change, requests, reader.credentials.Load())
			}
		})
	}
}

func TestIsDue_MeasuresFromRequestStart(t *testing.T) {
	svc, repo, _ := cadenceFixture(0)
	start := time.Now()
	finished := start.Add(20 * time.Second)
	latency := 20000
	repo.states["target"] = map[string]ConnectionHealthState{"model": {ConnectionID: "target", ModelName: "model", State: StateDegraded, LastProbeAt: &finished, LastLatencyMs: &latency, ConsecutiveFailures: 8}}
	if svc.isDue(context.Background(), "target", "model", Policy{ProbeIntervalSeconds: 60}, start.Add(59*time.Second)) {
		t.Fatal("probe due before 60 seconds from start")
	}
	if !svc.isDue(context.Background(), "target", "model", Policy{ProbeIntervalSeconds: 60}, start.Add(60*time.Second)) {
		t.Fatal("20-second timeout must not extend 60-second interval to 80 seconds")
	}
}
