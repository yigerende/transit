package connection_health

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestChannelProbeBudgetPostgresUpgradeAndConcurrency(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	config.MaxConns = 8
	id, _ := newID()
	schema := "test_budget_" + strings.ReplaceAll(id, "-", "")
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
	migration, err := os.ReadFile("../../database/migrations/000030_channel_probe_budget.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Migrations run before runtime schema initialization on fresh installs.
	if _, err = pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	if err = repo.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	// Recreate the pre-upgrade state: only the exhausted policy-wide counter exists.
	if _, err = pool.Exec(ctx, "DROP TABLE connection_health_channel_probe_budget_usage"); err != nil {
		t.Fatal(err)
	}
	day := probeBudgetDayStart(time.Now())
	if _, err = pool.Exec(ctx, `INSERT INTO connection_health_probe_budget_usage
	 (user_id,admin_account_id,policy_id,day_start,used) VALUES ('user','ws','policy',$1,2000)`, day); err != nil {
		t.Fatal(err)
	}
	for i, result := range []string{"ok", string(ResultNetworkFluctuation), "quality_suspend", "quality_restore"} {
		event := ConnectionHealthEvent{ID: fmt.Sprint(i), ConnectionID: "channel-a", UserID: "user", AdminAccountID: "ws", PolicyID: "policy", ModelName: fmt.Sprint(i), Result: result, Manual: i == 1}
		if err = repo.InsertEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	// Exclude both the previous day and the following day, even when reading history.
	for i, at := range []time.Time{day.Add(-time.Second), day.Add(24 * time.Hour)} {
		event := ConnectionHealthEvent{ID: fmt.Sprintf("outside-%d", i), ConnectionID: "channel-a", UserID: "user", AdminAccountID: "ws", PolicyID: "policy", ModelName: "model", Result: "ok"}
		if err = repo.InsertEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, "UPDATE connection_health_events SET created_at=$1 WHERE id=$2", at, event.ID); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err = pool.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
		if err = repo.EnsureSchema(ctx); err != nil {
			t.Fatal(err)
		}
	}
	count := func(user, workspace, policy, target string, when time.Time, want int) {
		t.Helper()
		got, err := repo.CountProbesToday(ctx, user, workspace, policy, target, when)
		if err != nil || got != want {
			t.Fatalf("%s/%s/%s/%s count=%d want=%d err=%v", user, workspace, policy, target, got, want, err)
		}
	}
	count("user", "ws", "policy", "channel-a", day, 2)
	count("user", "ws", "policy", "channel-b", day, 0)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.TryConsumeProbeBudget(ctx, "user", "ws", "policy", "channel-a", day, 20)
			if err != nil {
				t.Error(err)
			}
			if ok {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 18 {
		t.Fatalf("concurrent budget reservations accepted %d, want 18 after 2 historical probes", accepted.Load())
	}
	count("user", "ws", "policy", "channel-a", day, 20)
	// Every dimension is independent. Other models/groups intentionally have no key dimension.
	for _, key := range [][4]string{{"user", "ws", "policy", "channel-b"}, {"other", "ws", "policy", "channel-a"}, {"user", "other", "policy", "channel-a"}, {"user", "ws", "other", "channel-a"}} {
		ok, err := repo.TryConsumeProbeBudget(ctx, key[0], key[1], key[2], key[3], day, 1)
		if err != nil || !ok {
			t.Fatalf("independent budget blocked: %v %v", key, err)
		}
		count(key[0], key[1], key[2], key[3], day, 1)
	}
	usage, err := repo.ListProbeBudgetUsage(ctx, "user", "ws", day)
	if err != nil || len(usage) != 3 {
		t.Fatalf("workspace usage: %+v %v", usage, err)
	}
	for _, item := range usage {
		want := 1
		if item.PolicyID == "policy" && item.TargetID == "channel-a" {
			want = 20
		}
		if item.Used != want {
			t.Fatalf("wrong display count: %+v", item)
		}
	}
	// Reinitialization and pruning event history cannot restore already reserved quota.
	if err = repo.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "DELETE FROM connection_health_events"); err != nil {
		t.Fatal(err)
	}
	count("user", "ws", "policy", "channel-a", day, 20)
	ok, err := repo.TryConsumeProbeBudget(ctx, "user", "ws", "policy", "channel-a", day.Add(24*time.Hour), 20)
	if err != nil || !ok {
		t.Fatalf("next day did not reset: %v", err)
	}
	count("user", "ws", "policy", "channel-a", day.Add(24*time.Hour), 1)
}

func TestProbeBudgetDayResetsAtChinaMidnight(t *testing.T) {
	before := time.Date(2026, 10, 10, 15, 59, 59, 0, time.UTC)
	after := before.Add(time.Second)
	if !probeBudgetDayStart(after).Equal(time.Date(2026, 10, 10, 16, 0, 0, 0, time.UTC)) || probeBudgetDayStart(after).Sub(probeBudgetDayStart(before)) != 24*time.Hour {
		t.Fatal("daily quota must reset at midnight UTC+8")
	}
}
