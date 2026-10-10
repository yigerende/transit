package connection_health

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func TestManualHistoryAndSharedSourcePostgresUpgrade(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	config.MaxConns = 1
	id, _ := newID()
	schema := "test_manual_" + strings.ReplaceAll(id, "-", "")
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	repo := NewRepository(pool)
	if err = repo.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, scenario := range []string{"matching", "newer-state", "foreign-event", "manual-only"} {
		target := "sub2api:ws:" + scenario
		probeTime := now.Add(-time.Minute)
		if scenario == "newer-state" {
			probeTime = now.Add(time.Minute)
		}
		state := defaultTargetState("user", "ws", AdminProbeTarget{TargetID: target, AdminGroupID: "disabled-first", AdminGroupName: "old"}, "model")
		state.State, state.ConsecutiveFailures, state.LastProbeAt = StateSuspended, 5, &probeTime
		if err = repo.UpsertState(ctx, state); err != nil {
			t.Fatal(err)
		}
		event := ConnectionHealthEvent{ID: scenario, ConnectionID: target, UserID: "user", AdminAccountID: "ws", PolicyID: "policy", AdminGroupID: "active-second", OwnGroupName: "second", UpstreamGroupName: "second", ModelName: "model", Result: "ok", ToState: string(StateSuspended), LatencyMs: intPtr(1000), Manual: true, ProbeMode: ProbeModeFirstToken}
		if scenario == "foreign-event" {
			event.UserID = "other"
		}
		if scenario == "manual-only" {
			event.PolicyID = ""
		}
		if err = repo.InsertEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	migration, err := os.ReadFile("../../database/migrations/000028_manual_probe_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = pool.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
		if err = repo.EnsureSchema(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, scenario := range []string{"matching", "newer-state", "foreign-event", "manual-only"} {
		state, err := repo.GetState(ctx, "sub2api:ws:"+scenario, "model")
		if err != nil {
			t.Fatal(err)
		}
		want := "disabled-first"
		if scenario == "matching" {
			want = "active-second"
		}
		if state.OwnGroupID != want || state.UpstreamGroupID != want || state.State != StateSuspended || state.ConsecutiveFailures != 5 {
			t.Fatalf("unsafe source repair %s: %+v", scenario, state)
		}
	}
	target := "sub2api:ws:matching"
	samples, err := repo.ListRecentProbesByTargets(ctx, "user", "ws", []string{target})
	if err != nil || len(samples) != 1 || !samples[0].Manual || samples[0].ProbeMode != ProbeModeFirstToken {
		t.Fatalf("history marker lost: %+v %v", samples, err)
	}
	events, err := repo.ListEventsByConnection(ctx, target, "user", "ws", 100)
	if err != nil || len(events) != 1 || !events[0].Manual {
		t.Fatalf("event marker lost: %+v %v", events, err)
	}
	priority, err := repo.ListPriorityProbeSamples(ctx, "user", "ws", []string{target, "sub2api:ws:manual-only"}, now.Add(-time.Hour))
	if err != nil || len(priority) != 1 || priority[0].TargetID != target {
		t.Fatalf("managed manual sample excluded or unmanaged included: %+v %v", priority, err)
	}
}
