package connection_health

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGroupProbeHistoryKeepsQuietAndSharedTargetsWithoutLeakingWorkspaceEvents(t *testing.T) {
	repo := newFakeRepository()
	now := time.Now()
	for i := 0; i < 140; i++ {
		repo.events = append(repo.events, ConnectionHealthEvent{
			ID: fmt.Sprintf("busy-%03d", i), ConnectionID: "busy", UserID: "user", AdminAccountID: "workspace",
			Result: string(ResultOK), ModelName: "model", CreatedAt: now.Add(time.Duration(i) * time.Minute),
		})
	}
	for _, event := range []ConnectionHealthEvent{
		{ID: "quiet", ConnectionID: "quiet", UserID: "user", AdminAccountID: "workspace", Result: string(ResultRateLimited), CreatedAt: now.Add(-time.Hour)},
		{ID: "other-user", ConnectionID: "quiet", UserID: "other", AdminAccountID: "workspace", Result: string(ResultOK), CreatedAt: now},
		{ID: "other-workspace", ConnectionID: "quiet", UserID: "user", AdminAccountID: "other", Result: string(ResultOK), CreatedAt: now},
		{ID: "manual", ConnectionID: "quiet", UserID: "user", AdminAccountID: "workspace", Result: "manual_restore", CreatedAt: now},
		{ID: "credential", ConnectionID: "quiet", UserID: "user", AdminAccountID: "workspace", Result: "credential_unavailable", CreatedAt: now},
	} {
		repo.events = append(repo.events, event)
	}
	for i := 0; i < 25; i++ {
		repo.events = append(repo.events, ConnectionHealthEvent{ID: fmt.Sprintf("group-%02d", i), ConnectionID: groupProbeTargetID("workspace", "busy-group"), UserID: "user", AdminAccountID: "workspace", Result: string(ResultRateLimited), CreatedAt: now.Add(time.Duration(i) * time.Minute)})
	}
	groups := []AdminGroupHealth{
		{ID: "busy-group", Accounts: []AdminGroupAccount{{TargetID: "busy"}, {TargetID: "busy"}}},
		{ID: "quiet-group", Accounts: []AdminGroupAccount{{TargetID: "quiet"}}},
		{ID: "shared-group", Accounts: []AdminGroupAccount{{TargetID: "busy"}, {TargetID: "quiet"}}},
		{ID: "empty-group"},
	}
	svc := &Service{repo: repo}
	svc.attachGroupProbeHistory(context.Background(), "user", "workspace", groups)
	for _, index := range []int{0, 2} {
		probes := groups[index].Accounts[0].RecentProbes
		if len(probes) != 100 || probes[0].ID != "busy-139" || probes[99].ID != "busy-040" {
			t.Fatalf("group %s has incorrect latest 100 probes: %+v", groups[index].ID, probes)
		}
	}
	if probes := groups[1].Accounts[0].RecentProbes; len(probes) != 1 || probes[0].ID != "quiet" {
		t.Fatalf("quiet target history was lost or polluted: %+v", probes)
	}
	if probes := groups[0].RecentProbes; len(probes) != 20 || probes[0].ID != "group-24" || probes[19].ID != "group-05" {
		t.Fatalf("group must keep only its own latest 20 gateway probes: %+v", probes)
	}
	for _, index := range []int{1, 2} {
		if len(groups[index].RecentProbes) != 0 {
			t.Fatal("channel samples must never become group samples")
		}
	}
	if groups[3].RecentProbes == nil || len(groups[3].RecentProbes) != 0 {
		t.Fatal("empty group must return an empty array")
	}
}

type failingProbeHistoryRepository struct{ *fakeRepository }

func (f failingProbeHistoryRepository) ListRecentProbesByTargets(context.Context, string, string, []string) ([]GroupProbeSample, error) {
	return nil, errors.New("history unavailable")
}

func TestGroupProbeHistoryFailureKeepsGroupDetails(t *testing.T) {
	groups := []AdminGroupHealth{{ID: "g", Accounts: []AdminGroupAccount{{TargetID: "a"}}}}
	svc := &Service{repo: failingProbeHistoryRepository{newFakeRepository()}}
	svc.attachGroupProbeHistory(context.Background(), "user", "workspace", groups)
	if groups[0].ProbeHistoryError == "" || len(groups[0].Accounts) != 1 || groups[0].RecentProbes == nil {
		t.Fatalf("history failure must preserve details and expose unavailable history: %+v", groups)
	}
}

func TestRecentProbesByTargetsPostgres(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL repository tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	// One connection keeps all queries on the same temporary table. No persistent
	// application table or schema is created or changed by this test.
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("could not create test connection")
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE TEMP TABLE connection_health_events (
		id text, connection_id text, model_name text, result text, latency_ms int,
		created_at timestamptz, user_id text, admin_account_id text
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO connection_health_events
		SELECT 'busy-' || n, 'busy', 'model', 'ok', n, NOW() + n * INTERVAL '1 minute', 'user', 'workspace'
		FROM generate_series(1, 130) n;
		INSERT INTO connection_health_events VALUES
		('quiet', 'quiet', 'model', 'rate_limited', NULL, NOW() - INTERVAL '1 day', 'user', 'workspace'),
		('foreign-user', 'quiet', 'model', 'ok', 1, NOW(), 'other', 'workspace'),
		('foreign-workspace', 'quiet', 'model', 'ok', 1, NOW(), 'user', 'other'),
		('manual', 'quiet', 'model', 'manual_restore', 1, NOW(), 'user', 'workspace'),
		('credential', 'quiet', 'model', 'credential_unavailable', 1, NOW(), 'user', 'workspace'),
		('unrequested', 'other-target', 'model', 'ok', 1, NOW(), 'user', 'workspace')
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, probeModeHistorySchema+`; ALTER TABLE connection_health_events ADD COLUMN manual boolean NOT NULL DEFAULT false`); err != nil {
		t.Fatal(err)
	}
	samples, err := NewRepository(pool).ListRecentProbesByTargets(ctx, "user", "workspace", []string{"busy", "quiet"})
	if err != nil {
		t.Fatal(err)
	}
	busy, quiet := 0, 0
	for _, sample := range samples {
		switch sample.TargetID {
		case "busy":
			busy++
			if sample.LatencyMs == nil || *sample.LatencyMs <= 30 {
				t.Fatalf("old busy sample escaped the per-target limit: %+v", sample)
			}
		case "quiet":
			quiet++
			if sample.ID != "quiet" || sample.LatencyMs != nil {
				t.Fatalf("unexpected quiet sample: %+v", sample)
			}
		default:
			t.Fatalf("unrequested target leaked: %+v", sample)
		}
	}
	if busy != 100 || quiet != 1 {
		t.Fatalf("per-target history counts: busy=%d quiet=%d", busy, quiet)
	}
}
