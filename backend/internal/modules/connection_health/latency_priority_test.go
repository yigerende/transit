package connection_health

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

func latencyTestPolicy() Policy {
	return Policy{ID: "p1", Name: "latency", UserID: "user1", AdminAccountID: "ws1", Enabled: true, PriorityMode: PriorityModeLatency,
		AutoDegradeEnabled: true, LatencyPriority: defaultLatencyPriorityConfig(), ModelTargets: []ModelTarget{{ModelName: "model", Enabled: true}}}
}

func TestLatencyAverageUsesOnlyFreshSelectedModel(t *testing.T) {
	now := time.Now()
	p := latencyTestPolicy()
	samples := []PriorityProbeSample{
		{ID: "old", ModelName: "model", LatencyMs: 100, CreatedAt: now.Add(-301 * time.Second)},
		{ID: "fifth", ModelName: "model", LatencyMs: 2000, CreatedAt: now.Add(-240 * time.Second)},
		{ID: "fourth", ModelName: "model", LatencyMs: 3000, CreatedAt: now.Add(-180 * time.Second)},
		{ID: "third", ModelName: "model", LatencyMs: 4000, CreatedAt: now.Add(-120 * time.Second)},
		{ID: "other", ModelName: "other-model", LatencyMs: 10, CreatedAt: now},
		{ID: "latest", ModelName: "model", LatencyMs: 8000, CreatedAt: now},
		{ID: "second", ModelName: "model", LatencyMs: 5000, CreatedAt: now.Add(-60 * time.Second)},
		{ID: "future", ModelName: "model", LatencyMs: 1, CreatedAt: now.Add(time.Second)},
	}
	d := evaluateLatencyPriority(p, nil, samples, nil, now)
	if d.AverageMs == nil || math.Abs(*d.AverageMs-5100) > .001 || d.SampleCount != 5 || d.Priority != 2 {
		t.Fatalf("unexpected average: %+v", d)
	}
	p.LatencyPriority.SampleCount = 2
	p.LatencyPriority.Weights = []float64{3, 1}
	d = evaluateLatencyPriority(p, nil, samples, nil, now)
	if *d.AverageMs != 7250 || d.SampleCount != 2 {
		t.Fatalf("custom sample count/weights ignored: %+v", d)
	}
	d = evaluateLatencyPriority(p, nil, samples[:len(samples)-1], nil, now.Add(300*time.Second))
	if d.SampleCount != 1 || *d.AverageMs != 8000 {
		t.Fatalf("expiry boundary/partial weight normalization: %+v", d)
	}
	p.LatencyPriority.MinSamples = 2
	d = evaluateLatencyPriority(p, nil, samples[:len(samples)-1], nil, now.Add(300*time.Second))
	if d.Reason != "insufficient" || d.Priority != 50 {
		t.Fatalf("minimum sample threshold: %+v", d)
	}
}

func TestLatencyBandsRespectBoundaryMarginAndHealth(t *testing.T) {
	now := time.Now()
	p := latencyTestPolicy()
	p.LatencyPriority.Bands[0].Priority = 7
	p.LatencyPriority.Bands[1].Priority = 19
	for _, tt := range []struct{ ms, previous, want int }{{5000, 0, 7}, {5100, 7, 7}, {5600, 7, 19}, {4900, 19, 19}, {4400, 19, 7}, {15001, 0, 4}} {
		var previous *PrioritySyncState
		if tt.previous != 0 {
			previous = &PrioritySyncState{LastAppliedPriority: tt.previous}
		}
		d := evaluateLatencyPriority(p, nil, []PriorityProbeSample{{ModelName: "model", LatencyMs: tt.ms, CreatedAt: now}}, previous, now)
		if d.Priority != tt.want {
			t.Fatalf("%+v got %+v", tt, d)
		}
	}
	for _, tt := range []struct {
		state         State
		suspend, auto bool
		want          int
	}{{StateSuspended, true, true, 10000}, {StateSuspended, false, true, 100}, {StateDegraded, false, true, 100}, {StateDegraded, false, false, 7}, {StateDisabled, false, false, 10000}} {
		p.AutoSuspendEnabled = tt.suspend
		p.AutoDegradeEnabled = tt.auto
		d := evaluateLatencyPriority(p, []ConnectionHealthState{{ModelName: "model", State: tt.state}}, []PriorityProbeSample{{ModelName: "model", LatencyMs: 2000, CreatedAt: now}}, nil, now)
		if d.Priority != tt.want {
			t.Fatalf("health override %+v got %+v", tt, d)
		}
	}
}

func TestLatencyConfigRejectsInvalidRangesAndWeights(t *testing.T) {
	for name, mutate := range map[string]func(*LatencyPriorityConfig){
		"zero samples":       func(c *LatencyPriorityConfig) { c.SampleCount = 0 },
		"weight mismatch":    func(c *LatencyPriorityConfig) { c.Weights = []float64{1} },
		"zero weight":        func(c *LatencyPriorityConfig) { c.Weights[0] = 0 },
		"nan":                func(c *LatencyPriorityConfig) { c.Weights[0] = math.NaN() },
		"expired":            func(c *LatencyPriorityConfig) { c.MaxAgeSeconds = 0 },
		"gap":                func(c *LatencyPriorityConfig) { c.Bands[1].MinSeconds = 6 },
		"overlap":            func(c *LatencyPriorityConfig) { c.Bands[1].MinSeconds = 4 },
		"empty range":        func(c *LatencyPriorityConfig) { c.Bands[1].MaxSeconds = latencyFloat(5) },
		"no final range":     func(c *LatencyPriorityConfig) { c.Bands[3].MaxSeconds = latencyFloat(20) },
		"duplicate priority": func(c *LatencyPriorityConfig) { c.Bands[1].Priority = 1 },
		"negative priority":  func(c *LatencyPriorityConfig) { c.DegradedPriority = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			c := defaultLatencyPriorityConfig()
			mutate(c)
			if validateLatencyPriority(*c) {
				t.Fatal("accepted invalid config")
			}
		})
	}
	if !validateLatencyPriority(*defaultLatencyPriorityConfig()) {
		t.Fatal("default config invalid")
	}
}

func TestLatencySharedChannelWritesOnceAndRestoresOriginalPriority(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	p := latencyTestPolicy()
	q := latencyTestPolicy()
	q.ID = "p2"
	q.LatencyPriority.Bands[0].Priority = 9
	priority := 37
	reader := fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "g1"}, {ID: "g2"}}, accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{
		"g1": {{ID: "100", Priority: &priority, Models: "model"}}, "g2": {{ID: "100", Priority: &priority, Models: "model"}},
	}}
	actions := &fakeTargetPriorityActioner{}
	s := &Service{repo: repo, mySites: fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API}}, platformGroups: reader, priorityActions: actions}
	id := "sub2api:ws1:100"
	repo.events = []ConnectionHealthEvent{
		{ID: "success", UserID: "user1", AdminAccountID: "ws1", ConnectionID: id, PolicyID: p.ID, ModelName: "model", Result: "ok", LatencyMs: intPtr(3000), CreatedAt: time.Now()},
		{ID: "error", UserID: "user1", AdminAccountID: "ws1", ConnectionID: id, PolicyID: p.ID, ModelName: "model", Result: "server_error", LatencyMs: intPtr(100), CreatedAt: time.Now()},
		{ID: "manual", UserID: "user1", AdminAccountID: "ws1", ConnectionID: id, ModelName: "model", Result: "ok", LatencyMs: intPtr(19000), CreatedAt: time.Now()},
	}
	bindings := []GroupPolicyAssignment{{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", PolicyID: p.ID}, {UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g2", PolicyID: q.ID}}
	s.syncMultiplierPriorities(ctx, []Policy{q, p}, nil, bindings, nil, nil)
	if len(actions.calls) != 1 || actions.calls[0].priority != 9 {
		t.Fatalf("shared channel must have one conservative result: %+v", actions.calls)
	}
	stored := repo.priorityStates["user1|ws1|"+id]
	if stored.OriginalPriority != 37 {
		t.Fatalf("original priority lost: %+v", stored)
	}
	priority = 9
	s.syncMultiplierPriorities(ctx, []Policy{p, q}, nil, bindings, nil, []PrioritySyncState{stored})
	if len(actions.calls) != 1 {
		t.Fatal("unchanged priority rewritten")
	}
	s.syncMultiplierPriorities(ctx, nil, nil, nil, nil, []PrioritySyncState{stored})
	if len(actions.calls) != 2 || actions.calls[1].priority != 37 {
		t.Fatalf("original priority not restored: %+v", actions.calls)
	}
}

func TestLatencySharedDecisionOrderAndPlatformDirection(t *testing.T) {
	p := latencyTestPolicy()
	q := latencyTestPolicy()
	q.ID = "p2"
	q.LatencyPriority.InsufficientPriority = 90
	for _, ps := range [][]Policy{{p, q}, {q, p}} {
		d := latencyDecisionForTarget(upstream.PlatformSub2API, ps, nil, nil, nil, time.Now())
		if d.Priority != 90 || d.SharedPolicyCount != 2 || d.PolicyID != "p2" {
			t.Fatalf("unstable shared decision: %+v", d)
		}
		d = latencyDecisionForTarget(upstream.PlatformNewAPI, ps, nil, nil, nil, time.Now())
		if d.Priority != 50 {
			t.Fatalf("NewAPI must prefer lower result for conservative merge: %+v", d)
		}
	}
}

type unavailableLatencySamplesRepository struct{ *fakeRepository }

func (r unavailableLatencySamplesRepository) ListPriorityProbeSamples(context.Context, string, string, []string, time.Time) ([]PriorityProbeSample, error) {
	return nil, errors.New("sample database unavailable")
}

func TestLatencySampleReadFailurePreservesUpstreamPriority(t *testing.T) {
	repo := unavailableLatencySamplesRepository{newFakeRepository()}
	p := latencyTestPolicy()
	id := "sub2api:ws1:100"
	stored := PrioritySyncState{UserID: "user1", AdminAccountID: "ws1", TargetID: id, OriginalPriority: 37, LastAppliedPriority: 2}
	repo.priorityStates["user1|ws1|"+id] = stored
	actions := &fakeTargetPriorityActioner{}
	s := &Service{repo: repo, priorityActions: actions}
	inventory := map[string]*priorityTargetInventory{id: {target: AdminProbeTarget{AccountID: "100"}, policies: []Policy{p}, currentPriority: 2}}
	s.syncWorkspacePriorities(context.Background(), upstream.Session{Platform: upstream.PlatformSub2API}, "user1", "ws1", inventory, true, nil, []PrioritySyncState{stored})
	if len(actions.calls) != 0 || repo.priorityStates["user1|ws1|"+id].LastAppliedPriority != 2 {
		t.Fatal("sample read failure changed an upstream priority or lost the checkpoint")
	}
	groups := []AdminGroupHealth{{Accounts: []AdminGroupAccount{{TargetID: id, AssignedPolicyIDs: []string{p.ID}}}}}
	s.attachLatencyPriorities(context.Background(), "user1", "ws1", upstream.PlatformSub2API, groups, []Policy{p}, nil, []PrioritySyncState{stored})
	account := groups[0].Accounts[0]
	if !account.LatencyPriorityError || account.LatencyPriority != nil {
		t.Fatal("sample read failure was displayed as a valid decision")
	}
}
