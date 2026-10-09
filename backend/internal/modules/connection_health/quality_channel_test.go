package connection_health

import (
	"context"
	"errors"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

func TestChannelQualitySwitchPreservesHistoryAndHealthSelection(t *testing.T) {
	svc, quality, health, runner := qualityTestService(t)
	ctx := context.Background()
	q := defaultQualitySettings()
	q.Enabled = true
	if _, err := svc.SaveQualityConfiguration(ctx, "user", q); err != nil {
		t.Fatal(err)
	}
	for _, group := range []string{"one", "two"} {
		if _, err := svc.SetGroupQuality(ctx, "user", group, true); err != nil {
			t.Fatal(err)
		}
	}
	target := "sub2api:ws1:a"
	scope := QualityScope{UserID: "user", WorkspaceID: "ws1"}
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	if runner.calls != 1 {
		t.Fatal("default channel setting must preserve existing checks")
	}
	for _, enabled := range []bool{false, true} {
		result, err := svc.SetChannelQuality(ctx, "user", target, enabled)
		if err != nil || result.TargetID != target || result.Enabled != enabled {
			t.Fatalf("switch failed: %+v %v", result, err)
		}
		groups, err := svc.AdminGroups(ctx, "user")
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range groups {
			if group.ID != "one" && group.ID != "two" {
				continue
			}
			channel := group.Accounts[0]
			if channel.QualityEnabled != enabled || !channel.QualitySelected || len(channel.QualityHistory) != 1 {
				t.Fatalf("shared channel switch lost history or automation selection: %+v", channel)
			}
		}
		quality.mu.Lock()
		st := quality.states[qualityScopeKey("user", "ws1")][target]
		st.NextProbeAt = time.Time{}
		quality.states[qualityScopeKey("user", "ws1")][target] = st
		quality.mu.Unlock()
		svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
		want := 1
		if enabled {
			want = 2
		}
		if runner.calls != want {
			t.Fatalf("enabled=%t: got %d checks, want %d", enabled, runner.calls, want)
		}
	}
	if len(health.groupExclusions) != 0 || len(health.groupAssignments) != 2 || len(health.states) != 0 || len(health.events) != 0 {
		t.Fatal("quality switch changed normal health monitoring")
	}
}

func TestChannelQualityStillRequiresGroupAndPolicySelection(t *testing.T) {
	for _, scenario := range []string{"group-off", "global-off", "not-selected"} {
		t.Run(scenario, func(t *testing.T) {
			svc, _, health, runner := qualityTestService(t)
			ctx := context.Background()
			q := defaultQualitySettings()
			q.Enabled = true
			_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
			_, _ = svc.SetGroupQuality(ctx, "user", "one", true)
			if scenario == "group-off" {
				_, _ = svc.SetGroupQuality(ctx, "user", "one", false)
			}
			if scenario == "global-off" {
				q.Enabled = false
				_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
			}
			if scenario == "not-selected" {
				health.groupExclusions = []GroupTargetExclusion{{UserID: "user", AdminAccountID: "ws1", AdminGroupID: "one", TargetID: "sub2api:ws1:a"}}
			}
			if _, err := svc.SetChannelQuality(ctx, "user", "sub2api:ws1:a", true); err != nil {
				t.Fatal(err)
			}
			svc.runQualityScope(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
			if runner.calls != 0 {
				t.Fatal("channel switch bypassed another detection gate")
			}
		})
	}
}

func TestChannelQualityRejectsForeignOrMissingTargets(t *testing.T) {
	for _, target := range []string{"sub2api:other:a", "newapi:ws1:a", "sub2api:ws1:missing", "invalid"} {
		for _, enabled := range []bool{false, true} {
			svc, quality, _, _ := qualityTestService(t)
			if _, err := svc.SetChannelQuality(context.Background(), "user", target, enabled); err == nil || err.Error() != ErrorProbeTargetNotFound {
				t.Fatalf("invalid target %s was accepted: %v", target, err)
			}
			if len(quality.channels) != 0 {
				t.Fatal("invalid target wrote channel preference")
			}
		}
	}
}

func TestChannelQualityRechecksQueuedAndInFlightSwitches(t *testing.T) {
	for _, scenario := range []string{"queued", "in-flight"} {
		t.Run(scenario, func(t *testing.T) {
			svc, quality, _, runner := qualityTestService(t)
			ctx := context.Background()
			q := defaultQualitySettings()
			q.Enabled, q.Concurrency = true, 1
			_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
			_, _ = svc.SetGroupQuality(ctx, "user", "one", true)
			target := "sub2api:ws1:a"
			if scenario == "queued" {
				reader := svc.platformGroups.(fakePlatformGroupReader)
				reader.accountsByGrp["one"] = append(reader.accountsByGrp["one"], upstream.AdminGroupAccountInfo{ID: "b"})
				reader.credByAccount["b"] = reader.credByAccount["a"]
				svc.platformGroups = reader
				target = "sub2api:ws1:b"
			}
			runner.hook = func() { _ = quality.SetQualityChannel(ctx, "user", "ws1", target, false) }
			svc.runQualityScope(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
			if runner.calls != 1 {
				t.Fatal("disabled queued channel was requested")
			}
			for _, sample := range quality.history[qualityScopeKey("user", "ws1")] {
				if sample.TargetID == target {
					t.Fatal("disabled channel appended in-flight result")
				}
			}
		})
	}
}

type unavailableChannelSwitchRepo struct{ qualityRepository }

func (r unavailableChannelSwitchRepo) ListQualityChannels(context.Context, string, string) ([]QualityChannel, error) {
	return nil, errors.New("unavailable")
}

func TestChannelQualityDoesNotRunWhenSwitchReadFails(t *testing.T) {
	svc, quality, _, runner := qualityTestService(t)
	ctx := context.Background()
	q := defaultQualitySettings()
	q.Enabled = true
	_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
	_, _ = svc.SetGroupQuality(ctx, "user", "one", true)
	svc.qualityRepo = unavailableChannelSwitchRepo{quality}
	svc.runQualityScope(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
	if runner.calls != 0 {
		t.Fatal("failed switch load allowed detection")
	}
}

func TestChannelQualityPostgresPersistenceAndResultGuard(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	repo := NewRepository(pool)
	q := defaultQualitySettings()
	q.Enabled, q.Revision = true, "v1"
	if err := repo.SaveQualitySettings(ctx, "user", "site", q); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetQualityGroup(ctx, "user", "site", "group", true); err != nil {
		t.Fatal(err)
	}
	id, _ := newID()
	state := QualityState{TargetID: "target", Revision: q.Revision, Latest: QualitySample{ID: id, TargetID: "target", Result: "passed", CreatedAt: time.Now()}}
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, state); err != nil || !ok {
		t.Fatalf("default-on check failed: %v", err)
	}
	if err := repo.SetQualityChannel(ctx, "user", "site", "target", false); err != nil {
		t.Fatal(err)
	}
	channels, err := NewRepository(pool).ListQualityChannels(ctx, "user", "site")
	if err != nil || len(channels) != 1 || channels[0].Enabled {
		t.Fatalf("disabled channel did not survive reload: %+v %v", channels, err)
	}
	for _, scope := range [][2]string{{"other", "site"}, {"user", "other"}} {
		channels, err := repo.ListQualityChannels(ctx, scope[0], scope[1])
		if err != nil || len(channels) != 0 {
			t.Fatal("channel preference crossed scope")
		}
	}
	state.Latest.ID, _ = newID()
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, state); err != nil || ok {
		t.Fatalf("disabled channel accepted a late result: %v", err)
	}
	history, err := repo.ListQualityHistory(ctx, "user", "site", []string{"target"}, 100)
	if err != nil || len(history) != 1 {
		t.Fatal("switch erased or appended history")
	}
	if err := repo.SetQualityChannel(ctx, "user", "site", "target", true); err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, state); err != nil || !ok {
		t.Fatalf("re-enabled channel failed: %v", err)
	}
	// Also guard the first result of a channel switched off before any check.
	if err := repo.SetQualityChannel(ctx, "user", "site", "new-target", false); err != nil {
		t.Fatal(err)
	}
	state.TargetID, state.Latest.TargetID = "new-target", "new-target"
	state.Latest.ID, _ = newID()
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, state); err != nil || ok {
		t.Fatalf("first result overwrote disabled preference: %v", err)
	}
}
