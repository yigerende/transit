package connection_health

import (
	"context"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

func TestChannelQualityInheritsGroupUntilExplicitlyEnabled(t *testing.T) {
	svc, quality, _, runner := qualityTestService(t)
	ctx := context.Background()
	q := defaultQualitySettings()
	q.Enabled = true
	if _, err := svc.SaveQualityConfiguration(ctx, "user", q); err != nil {
		t.Fatal(err)
	}
	const target = "sub2api:ws1:a"
	reader := svc.platformGroups.(fakePlatformGroupReader)
	reader.accountsByGrp["one"] = append(reader.accountsByGrp["one"], upstream.AdminGroupAccountInfo{ID: "b"})
	reader.credByAccount["b"] = reader.credByAccount["a"]
	svc.platformGroups = reader
	scope := QualityScope{UserID: "user", WorkspaceID: "ws1"}
	assertChannels := func(wantOne, wantTwo, wantNeighbor bool) {
		t.Helper()
		groups, err := svc.AdminGroups(ctx, "user")
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range groups {
			for _, a := range group.Accounts {
				want := false
				if a.TargetID == target {
					want = group.ID == "one" && wantOne || group.ID == "two" && wantTwo
				} else if a.ID == "b" {
					want = wantNeighbor
				}
				if a.QualityEnabled != want {
					t.Fatalf("group %s channel %s: enabled=%t, want %t", group.ID, a.ID, a.QualityEnabled, want)
				}
			}
		}
	}
	assertChannels(false, false, false)
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	if runner.calls != 0 {
		t.Fatal("group-off defaults triggered detection")
	}
	if _, err := svc.SetGroupQuality(ctx, "user", "one", true); err != nil {
		t.Fatal(err)
	}
	assertChannels(true, false, true)
	if _, err := svc.SetGroupQuality(ctx, "user", "one", false); err != nil {
		t.Fatal(err)
	}
	assertChannels(false, false, false)
	channel, err := svc.SetChannelQuality(ctx, "user", target, true)
	if err != nil || !channel.Enabled || !channel.Independent {
		t.Fatalf("explicit enable failed: %+v %v", channel, err)
	}
	assertChannels(true, true, false)
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	if runner.calls != 1 || len(quality.history[qualityScopeKey("user", "ws1")]) != 1 {
		t.Fatal("independent shared channel must run once and persist its result with every group off")
	}
	// Polling before the configured due time must not run another request.
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	if runner.calls != 1 {
		t.Fatal("independent channel ignored the configured interval")
	}
	if _, err := svc.SetChannelQuality(ctx, "user", target, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetGroupQuality(ctx, "user", "one", true); err != nil {
		t.Fatal(err)
	}
	assertChannels(false, false, true)
}

func TestIndependentChannelQualityHonorsHealthPause(t *testing.T) {
	svc, quality, health, runner := qualityHealthPauseFixture(t)
	ctx := context.Background()
	for _, group := range []string{"one", "two"} {
		if _, err := svc.SetGroupQuality(ctx, "user", group, false); err != nil {
			t.Fatal(err)
		}
	}
	const target = "sub2api:ws1:a"
	if _, err := svc.SetChannelQuality(ctx, "user", target, true); err != nil {
		t.Fatal(err)
	}
	setQualityTestHealth(health, target, StateSuspended)
	scope := QualityScope{UserID: "user", WorkspaceID: "ws1"}
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	if runner.calls != 0 {
		t.Fatal("independent opt-in bypassed health suspension")
	}
	setQualityTestHealth(health, target, StateHealthy)
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	if runner.calls != 1 || len(quality.history[qualityScopeKey("user", "ws1")]) != 1 {
		t.Fatal("independent opt-in did not resume after health recovery")
	}
}

func TestIndependentChannelQualityPostgresScopesAndPersistence(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	repo := NewRepository(pool)
	q := defaultQualitySettings()
	q.Enabled, q.Revision = true, "v1"
	if err := repo.SaveQualitySettings(ctx, "user", "site", q); err != nil {
		t.Fatal(err)
	}
	assertScopes := func(want int) {
		t.Helper()
		scopes, err := repo.ListQualityScopes(ctx)
		if err != nil || len(scopes) != want {
			t.Fatalf("got scopes %+v, want %d: %v", scopes, want, err)
		}
	}
	assertScopes(0)
	if err := repo.SetQualityGroup(ctx, "user", "site", "group", true); err != nil {
		t.Fatal(err)
	}
	id, _ := newID()
	st := QualityState{TargetID: "target", Revision: q.Revision, Latest: QualitySample{ID: id, TargetID: "target", Result: "passed", CreatedAt: time.Now()}}
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, st); err != nil || !ok {
		t.Fatalf("inherited save failed: %v", err)
	}
	if err := repo.SetQualityGroup(ctx, "user", "site", "group", false); err != nil {
		t.Fatal(err)
	}
	assertScopes(0)
	channels, err := repo.ListQualityChannels(ctx, "user", "site")
	if err != nil || len(channels) != 1 || channels[0].Independent {
		t.Fatal("saving history created an independent opt-in")
	}
	st.Latest.ID, _ = newID()
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, st); err != nil || ok {
		t.Fatalf("inherited channel saved after group was disabled: %v", err)
	}
	if err := repo.SetQualityChannel(ctx, "user", "site", "target", true); err != nil {
		t.Fatal(err)
	}
	repo = NewRepository(pool)
	channels, err = repo.ListQualityChannels(ctx, "user", "site")
	if err != nil || len(channels) != 1 || !channels[0].Enabled || !channels[0].Independent {
		t.Fatal("independent channel preference did not survive reload")
	}
	assertScopes(1)
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, st); err != nil || !ok {
		t.Fatalf("independent channel did not save with group off: %v", err)
	}
	if err := repo.SetQualityChannel(ctx, "user", "site", "target", false); err != nil {
		t.Fatal(err)
	}
	assertScopes(0)
	st.Latest.ID, _ = newID()
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, st); err != nil || ok {
		t.Fatalf("disabled independent channel accepted a late result: %v", err)
	}
	history, err := repo.ListQualityHistory(ctx, "user", "site", []string{"target"}, 100)
	if err != nil || len(history) != 2 {
		t.Fatalf("switches changed completed history: count=%d err=%v", len(history), err)
	}
	if err := repo.SetQualityChannel(ctx, "user", "site", "target", true); err != nil {
		t.Fatal(err)
	}
	q.Enabled = false
	if err := repo.SaveQualitySettings(ctx, "user", "site", q); err != nil {
		t.Fatal(err)
	}
	assertScopes(0)
}

func TestIndependentChannelQualityMigrationPreservesLegacyPreferences(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	if _, err := pool.Exec(ctx, `ALTER TABLE connection_health_quality_channels DROP COLUMN independent;
	 INSERT INTO connection_health_quality_channels(user_id,admin_account_id,target_id,enabled)
	 VALUES('user','site','inherited',true),('user','site','disabled',false)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := pool.Exec(ctx, qualityChannelIndependentSchema); err != nil {
			t.Fatal(err)
		}
	}
	channels, err := NewRepository(pool).ListQualityChannels(ctx, "user", "site")
	if err != nil || len(channels) != 2 {
		t.Fatal(err)
	}
	for _, channel := range channels {
		if channel.Independent || channel.Enabled != (channel.TargetID == "inherited") {
			t.Fatalf("migration changed legacy preference: %+v", channel)
		}
	}
}
