package connection_health

import (
	"context"
	"errors"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

type qualityVerdictRunner struct {
	result string
	hook   func()
}

func (r *qualityVerdictRunner) ProbeQuality(_ context.Context, _ upstream.ProbeCredential, _ string, _ QualitySettings, q QualityQuestion) qualityProbeResult {
	if r.hook != nil {
		r.hook()
	}
	switch r.result {
	case "error":
		return qualityProbeResult{ErrorKey: "server_error"}
	case "failed":
		return qualityProbeResult{Answer: "wrong answer", Verdict: "failed", DurationMS: 10}
	default:
		return qualityProbeResult{Answer: q.Answer, Verdict: "passed", DurationMS: 10}
	}
}

// Model upstream writes in both group inventories, so subsequent checks see the
// actual status and exercise the production ownership/conflict safeguards.
type qualityStatusActioner struct {
	fakePlatformActioner
	reader fakePlatformGroupReader
}

func (a *qualityStatusActioner) UpdateSub2APIAdminAccountStatus(session upstream.Session, id, status string) error {
	if err := a.fakePlatformActioner.UpdateSub2APIAdminAccountStatus(session, id, status); err != nil {
		return err
	}
	for _, accounts := range a.reader.accountsByGrp {
		for i := range accounts {
			if accounts[i].ID == id {
				accounts[i].Status = status
			}
		}
	}
	return nil
}

type qualityActionFixture struct {
	svc     *Service
	quality *fakeQualityRepo
	health  *fakeRepository
	runner  *qualityVerdictRunner
	remote  *qualityStatusActioner
	q       QualitySettings
}

const qualityActionTarget = "sub2api:ws1:a"

func newQualityActionFixture(t *testing.T) *qualityActionFixture {
	t.Helper()
	svc, quality, health, _ := qualityTestService(t)
	svc.repo = health // Independent lease keys, unlike the single-mutex legacy fake.
	_ = health.SetChannelQualitySuspension(context.Background(), "user", "ws1", qualityActionTarget, true)
	p := &health.policies[0]
	p.AutoDegradeEnabled, p.AutoRemoteActionEnabled, p.AutoSuspendEnabled = true, true, true
	p.ModelTargets = []ModelTarget{{ModelName: "quality-model", ProviderFamily: ProviderOpenAI, Enabled: true}}
	health.states[qualityActionTarget] = map[string]ConnectionHealthState{"quality-model": {
		ConnectionID: qualityActionTarget, UserID: "user", AdminAccountID: "ws1", OwnGroupID: "one",
		ModelName: "quality-model", State: StateHealthy,
	}}
	runner := &qualityVerdictRunner{result: "failed"}
	svc.qualityRunner = runner
	reader := svc.platformGroups.(fakePlatformGroupReader)
	remote := &qualityStatusActioner{reader: reader}
	for _, accounts := range reader.accountsByGrp {
		for i := range accounts {
			accounts[i].Status = "active"
		}
	}
	svc.dispatcher = newRemoteActionDispatcher(nil, nil, remote)
	q := enableManualQuality(t, svc)
	q.AutoSuspendEnabled, q.FailureLimit, q.RecoveryLimit, q.Mode = true, 3, 2, "content"
	q, err := svc.SaveQualityConfiguration(context.Background(), "user", q)
	if err != nil {
		t.Fatal(err)
	}
	return &qualityActionFixture{svc, quality, health, runner, remote, q}
}

func (f *qualityActionFixture) probe(t *testing.T, result string, manual bool) {
	t.Helper()
	f.runner.result = result
	before := len(f.quality.history["user|ws1"])
	if manual {
		sample, err := f.svc.ProbeChannelQuality(context.Background(), "user", qualityActionTarget, "two", "questions")
		if err != nil || sample.Result != result || !sample.Manual {
			t.Fatalf("manual result: %+v, %v", sample, err)
		}
	} else {
		state := f.quality.states["user|ws1"][qualityActionTarget]
		state.NextProbeAt = time.Time{}
		if f.quality.states["user|ws1"] == nil {
			f.quality.states["user|ws1"] = map[string]QualityState{}
		}
		f.quality.states["user|ws1"][qualityActionTarget] = state
		f.svc.runQualityScope(context.Background(), QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
	}
	history := f.quality.history["user|ws1"]
	if len(history) != before+1 || history[len(history)-1].Result != result {
		t.Fatalf("expected exactly one shared-channel sample: %+v", history)
	}
}

func (f *qualityActionFixture) maintain() {
	states, _ := f.health.ListTargetActionStates(context.Background(), "user", "ws1")
	f.svc.restoreUnmanagedTargetActions(context.Background(), f.health.policies, f.health.assignments, f.health.groupAssignments, f.health.groupExclusions, states, make(adminInventoryCache))
}

func (f *qualityActionFixture) assertActions(t *testing.T, statuses ...string) {
	t.Helper()
	if len(f.remote.sub2APICalls) != len(statuses) {
		t.Fatalf("actions=%+v, want %v", f.remote.sub2APICalls, statuses)
	}
	for i, want := range statuses {
		if f.remote.sub2APICalls[i].status != want {
			t.Fatalf("action %d=%s, want %s", i, f.remote.sub2APICalls[i].status, want)
		}
	}
}

func TestQualitySuspensionThresholdsAndAutomaticRecovery(t *testing.T) {
	f := newQualityActionFixture(t)
	f.probe(t, "failed", true)
	f.probe(t, "error", true)
	f.probe(t, "failed", false)
	f.assertActions(t)
	f.probe(t, "failed", true)
	f.assertActions(t, "inactive")
	if !f.health.targetActionStates["user|ws1|"+qualityActionTarget].QualitySuspended {
		t.Fatal("missing persistent quality hold")
	}
	// A normal health reconciliation must not restore a degraded channel.
	if err := f.svc.reconcileQualityTarget(context.Background(), "user", "ws1", qualityActionTarget); err != nil {
		t.Fatal(err)
	}
	f.assertActions(t, "inactive")
	f.probe(t, "passed", false) // inactive upstream accounts remain quality-testable
	f.assertActions(t, "inactive")
	f.probe(t, "error", false)
	f.assertActions(t, "inactive")
	f.probe(t, "passed", false)
	f.assertActions(t, "inactive", "active")
	if len(f.health.targetActionStates) != 0 {
		t.Fatal("restored hold was not removed")
	}
	if len(f.health.events) != 2 || f.health.events[0].Result != "quality_suspend" || f.health.events[1].Result != "quality_restore" {
		t.Fatalf("bad audit events: %+v", f.health.events)
	}
	if f.health.states[qualityActionTarget]["quality-model"].State != StateHealthy {
		t.Fatal("quality changed health evidence")
	}
}

func TestQualitySuspensionHonorsAllPermissions(t *testing.T) {
	cases := map[string]func(*qualityActionFixture){
		"quality default off": func(f *qualityActionFixture) { f.q.AutoSuspendEnabled = false },
		"auto degrade off":    func(f *qualityActionFixture) { f.health.policies[0].AutoDegradeEnabled = false },
		"remote actions off":  func(f *qualityActionFixture) { f.health.policies[0].AutoRemoteActionEnabled = false },
		"policy pause off":    func(f *qualityActionFixture) { f.health.policies[0].AutoSuspendEnabled = false },
		"policy disabled":     func(f *qualityActionFixture) { f.health.policies[0].Enabled = false },
		"channel pause off": func(f *qualityActionFixture) {
			_ = f.health.SetChannelSuspension(context.Background(), "user", "ws1", qualityActionTarget, false)
		},
		"unassigned": func(f *qualityActionFixture) { f.health.groupAssignments = nil },
		"excluded from both groups": func(f *qualityActionFixture) {
			for _, group := range []string{"one", "two"} {
				f.health.groupExclusions = append(f.health.groupExclusions, GroupTargetExclusion{UserID: "user", AdminAccountID: "ws1", AdminGroupID: group, TargetID: qualityActionTarget})
			}
		},
		"permission revoked in flight": func(f *qualityActionFixture) {
			f.runner.hook = func() { f.health.policies[0].AutoSuspendEnabled = false }
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newQualityActionFixture(t)
			change(f)
			f.q.FailureLimit = 1
			var err error
			f.q, err = f.svc.SaveQualityConfiguration(context.Background(), "user", f.q)
			if err != nil {
				t.Fatal(err)
			}
			f.probe(t, "failed", true)
			f.assertActions(t)
			if len(f.health.targetActionStates) != 0 {
				t.Fatal("took ownership without permission")
			}
		})
	}
}

func TestQualitySuspensionRevisionRetainsHoldAndRecoveryThreshold(t *testing.T) {
	f := newQualityActionFixture(t)
	for i := 0; i < 3; i++ {
		f.probe(t, "failed", true)
	}
	f.q.Model = "new-quality-model"
	var err error
	f.q, err = f.svc.SaveQualityConfiguration(context.Background(), "user", f.q)
	if err != nil {
		t.Fatal(err)
	}
	f.maintain()
	f.assertActions(t, "inactive")
	f.probe(t, "passed", true)
	f.assertActions(t, "inactive")
	f.probe(t, "passed", true)
	f.assertActions(t, "inactive", "active")
}

func TestQualitySuspensionAndHealthSuspensionAreIndependent(t *testing.T) {
	for _, healthBlocked := range []bool{false, true} {
		t.Run(map[bool]string{false: "quality setting off", true: "health still suspended"}[healthBlocked], func(t *testing.T) {
			f := newQualityActionFixture(t)
			for i := 0; i < 3; i++ {
				f.probe(t, "failed", true)
			}
			if healthBlocked {
				st := f.health.states[qualityActionTarget]["quality-model"]
				st.State = StateSuspended
				f.health.states[qualityActionTarget]["quality-model"] = st
			}
			// Explicitly disabling quality actions releases only its own hold.
			f.q.AutoSuspendEnabled = false
			if _, err := f.svc.SaveQualityConfiguration(context.Background(), "user", f.q); err != nil {
				t.Fatal(err)
			}
			f.maintain()
			if healthBlocked {
				f.assertActions(t, "inactive")
				if f.health.targetActionStates["user|ws1|"+qualityActionTarget].QualitySuspended {
					t.Fatal("quality hold retained after switch off")
				}
				st := f.health.states[qualityActionTarget]["quality-model"]
				st.State = StateHealthy
				f.health.states[qualityActionTarget]["quality-model"] = st
				if err := f.svc.reconcileQualityTarget(context.Background(), "user", "ws1", qualityActionTarget); err != nil {
					t.Fatal(err)
				}
			}
			f.assertActions(t, "inactive", "active")
		})
	}
}

func TestQualitySuspensionRemoteFailureKeepsEvidenceAndRetries(t *testing.T) {
	f := newQualityActionFixture(t)
	f.remote.sub2APIErr = errors.New("upstream unavailable")
	for i := 0; i < 3; i++ {
		f.probe(t, "failed", true)
	}
	f.assertActions(t, "inactive")
	if f.health.targetActionStates["user|ws1|"+qualityActionTarget].PendingStatus != "inactive" {
		t.Fatal("no retry checkpoint")
	}
	f.remote.sub2APIErr = nil
	f.maintain()
	f.assertActions(t, "inactive", "inactive")
	f.probe(t, "passed", true)
	f.remote.sub2APIErr = errors.New("upstream unavailable")
	f.probe(t, "passed", true)
	f.assertActions(t, "inactive", "inactive", "active")
	f.remote.sub2APIErr = nil
	f.maintain()
	f.assertActions(t, "inactive", "inactive", "active", "active")
	if len(f.health.targetActionStates) != 0 {
		t.Fatal("recovery retry did not release ownership")
	}
}

func TestQualitySuspensionProtectsInitiallyDisabledChannel(t *testing.T) {
	f := newQualityActionFixture(t)
	for _, accounts := range f.remote.reader.accountsByGrp {
		for i := range accounts {
			accounts[i].Status = "inactive"
		}
	}
	for i := 0; i < 3; i++ {
		f.probe(t, "failed", true)
	}
	for i := 0; i < 2; i++ {
		f.probe(t, "passed", true)
	}
	f.assertActions(t)
	if len(f.health.targetActionStates) != 0 {
		t.Fatal("took ownership of manually disabled channel")
	}
}

func TestQualityRecoveryCannotReleaseExistingHealthHold(t *testing.T) {
	for _, healthState := range []State{StateDegraded, StateSuspended} {
		t.Run(string(healthState), func(t *testing.T) {
			f := newQualityActionFixture(t)
			for i := 0; i < 3; i++ {
				f.probe(t, "failed", true)
			}
			// A quality result can finish just before health takes its own hold.
			state := f.quality.states["user|ws1"][qualityActionTarget]
			state.Degraded, state.Status, state.Successes = false, "normal", f.q.RecoveryLimit
			f.quality.states["user|ws1"][qualityActionTarget] = state
			health := f.health.states[qualityActionTarget]["quality-model"]
			health.State = healthState
			f.health.states[qualityActionTarget]["quality-model"] = health
			f.maintain()
			f.assertActions(t, "inactive")
			if f.health.targetActionStates["user|ws1|"+qualityActionTarget].QualitySuspended {
				t.Fatal("recovered quality reason was retained")
			}
			health.State = StateHealthy
			f.health.states[qualityActionTarget]["quality-model"] = health
			if err := f.svc.reconcileQualityTarget(context.Background(), "user", "ws1", qualityActionTarget); err != nil {
				t.Fatal(err)
			}
			f.assertActions(t, "inactive", "active")
		})
	}
}

func TestQualitySuspensionManualAPIBenchmarksShareThresholds(t *testing.T) {
	f := newQualityActionFixture(t)
	// All three manual buttons use saved thresholds even with automatic checks off.
	f.q.Enabled = false
	if _, err := f.svc.SaveQualityConfiguration(context.Background(), "user", f.q); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"questions", "manxue_candy", "manxue_pelican"} {
		sample, err := f.svc.ProbeChannelQuality(context.Background(), "user", qualityActionTarget, "one", method)
		if err != nil || sample.Result != "failed" {
			t.Fatalf("%s: %+v, %v", method, sample, err)
		}
	}
	f.assertActions(t, "inactive")
	f.runner.result = "passed"
	for _, method := range []string{"manxue_pelican", "manxue_candy"} {
		if _, err := f.svc.ProbeChannelQuality(context.Background(), "user", qualityActionTarget, "two", method); err != nil {
			t.Fatal(err)
		}
	}
	f.assertActions(t, "inactive", "active")
}
