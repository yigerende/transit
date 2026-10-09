package connection_health

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func qualityTestPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	migration, err := os.ReadFile("../../database/migrations/000020_channel_quality_detection.sql")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(strings.TrimSpace(string(migration)), "\r\n", "\n") != strings.TrimSpace(qualitySchema) {
		t.Fatal("migration and runtime schema differ")
	}
	if _, err = pool.Exec(ctx, strings.ReplaceAll(qualitySchema, "CREATE TABLE IF NOT EXISTS", "CREATE TEMP TABLE IF NOT EXISTS")); err != nil {
		t.Fatal(err)
	}
	channelMigration, err := os.ReadFile("../../database/migrations/000023_channel_quality_switches.sql")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(strings.TrimSpace(string(channelMigration)), "\r\n", "\n") != strings.TrimSpace(qualityChannelSchema) {
		t.Fatal("channel migration and runtime schema differ")
	}
	if _, err := pool.Exec(ctx, strings.ReplaceAll(qualityChannelSchema, "CREATE TABLE IF NOT EXISTS", "CREATE TEMP TABLE IF NOT EXISTS")); err != nil {
		t.Fatal(err)
	}
	return ctx, pool
}
func TestQualityPostgresPersistenceIsolationAndStaleWrites(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	repo := NewRepository(pool)
	q := defaultQualitySettings()
	q.Enabled = true
	q.Revision = "v1"
	q.HistoryLimit = 2
	if err := repo.SaveQualitySettings(ctx, "user", "site", q); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewRepository(pool).GetQualitySettings(ctx, "user", "site")
	if err != nil || loaded.Model != q.Model || loaded.Revision != "v1" {
		t.Fatal("settings did not survive reload")
	}
	_ = repo.SetQualityGroup(ctx, "user", "site", "group", true)
	scopes, err := repo.ListQualityScopes(ctx)
	if err != nil || len(scopes) != 1 || scopes[0].WorkspaceID != "site" {
		t.Fatal("enabled scope missing")
	}
	for i := 0; i < 3; i++ {
		id, _ := newID()
		sample := QualitySample{ID: id, TargetID: "target", QuestionName: "clock", Result: "passed", Answer: "7.5", CreatedAt: time.Now().Add(time.Duration(i) * time.Second)}
		state := QualityState{TargetID: "target", Revision: "v1", Status: "normal", NextProbeAt: time.Now().Add(time.Minute), Latest: sample}
		ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, state)
		if err != nil || !ok {
			t.Fatalf("saving quality result: %v", err)
		}
	}
	history, err := repo.ListQualityHistory(ctx, "user", "site", []string{"target"}, 100)
	if err != nil || len(history) != 2 || history[0].CreatedAt.Before(history[1].CreatedAt) {
		t.Fatal("history retention/order failed")
	}
	states, err := repo.ListQualityStates(ctx, "user", "site")
	if err != nil || len(states) != 1 || states[0].Latest.Answer != "7.5" {
		t.Fatal("state not persisted")
	}
	for _, scope := range [][2]string{{"other", "site"}, {"user", "other"}} {
		history, err := repo.ListQualityHistory(ctx, scope[0], scope[1], []string{"target"}, 100)
		if err != nil || len(history) != 0 {
			t.Fatal("history crossed scope")
		}
		states, err := repo.ListQualityStates(ctx, scope[0], scope[1])
		if err != nil || len(states) != 0 {
			t.Fatal("state crossed scope")
		}
	}
	st := states[0]
	st.Latest.ID, _ = newID()
	_ = repo.SetQualityGroup(ctx, "user", "site", "group", false)
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, st); err != nil || ok {
		t.Fatal("disabled group accepted in-flight result")
	}
	_ = repo.SetQualityGroup(ctx, "user", "site", "group", true)
	updated := q
	updated.Revision = "v2"
	_ = repo.SaveQualitySettings(ctx, "user", "site", updated)
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, q, st); err != nil || ok {
		t.Fatal("stale revision accepted")
	}
	history, err = repo.ListQualityHistory(ctx, "user", "site", []string{"target"}, 100)
	if err != nil || len(history) != 2 {
		t.Fatal("configuration change hid completed checks from the timeline")
	}
	_, err = pool.Exec(ctx, `DELETE FROM connection_health_quality_settings WHERE user_id='user' AND admin_account_id='site'`)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.SaveQualityResult(ctx, "user", "site", []string{"group"}, updated, st); err != nil || ok {
		t.Fatal("workspace deletion resurrected task")
	}
}

func TestQualityPostgresSchedulerRunsWithoutBrowser(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	svc, _, health, runner := qualityTestService(t)
	svc.qualityRepo = NewRepository(pool)
	q := defaultQualitySettings()
	q.Enabled = true
	q.IntervalSeconds = 10
	if _, err := svc.SaveQualityConfiguration(ctx, "user", q); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetGroupQuality(ctx, "user", "one", true); err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	svc.StartQualityScheduler(runCtx)
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("background scheduler did not produce two results")
		case <-tick.C:
			var count int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM connection_health_quality_history`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count >= 2 {
				_, err := svc.SetGroupQuality(ctx, "user", "one", false)
				if err != nil {
					t.Fatal(err)
				}
				cancel()
				runner.mu.Lock()
				calls := runner.calls
				runner.mu.Unlock()
				if calls != 2 {
					t.Fatalf("unexpected scheduled count %d", calls)
				}
				states, err := svc.qualityRepo.ListQualityStates(ctx, "user", "ws1")
				if err != nil || len(states) != 1 || states[0].Successes != 2 {
					t.Fatal("second answer did not extend saved streak")
				}
				if len(health.events) != 0 || len(health.states) != 0 {
					t.Fatal("question scheduler wrote health data")
				}
				encoded, _ := json.Marshal(states)
				if strings.Contains(string(encoded), "secret") {
					t.Fatal("credentials persisted")
				}
				return
			}
		}
	}
}

func TestQualityPostgresHistorySurvivesConfigurationChanges(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	svc, _, _, _ := qualityTestService(t)
	svc.qualityRepo = NewRepository(pool)
	q := defaultQualitySettings()
	q.Enabled = true
	var err error
	q, err = svc.SaveQualityConfiguration(ctx, "user", q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetGroupQuality(ctx, "user", "one", true); err != nil {
		t.Fatal(err)
	}
	scope := QualityScope{UserID: "user", WorkspaceID: "ws1"}
	tokens := make(chan struct{}, 32)
	read := func(count int, current bool) {
		t.Helper()
		// A fresh service/repository read models both polling and page reloads.
		restarted := *svc
		restarted.qualityRepo = NewRepository(pool)
		groups := []AdminGroupHealth{{ID: "one", Accounts: []AdminGroupAccount{{TargetID: "sub2api:ws1:a"}}}}
		restarted.attachQuality(ctx, "user", "ws1", groups)
		account := groups[0].Accounts[0]
		if len(account.QualityHistory) != count {
			t.Fatalf("configuration change hid channel history: got %d, want %d", len(account.QualityHistory), count)
		}
		if (account.QualityState != nil) != current {
			t.Fatal("history from an earlier configuration became the current verdict")
		}
		if current && account.QualityState.Successes != 1 {
			t.Fatal("old answers counted toward the new configuration's streak")
		}
		for i := 1; i < len(account.QualityHistory); i++ {
			if !account.QualityHistory[i-1].CreatedAt.After(account.QualityHistory[i].CreatedAt) {
				t.Fatal("history is not ordered newest first")
			}
		}
	}
	for i := 1; i <= 3; i++ {
		svc.runQualityScope(ctx, scope, tokens)
		read(i, true)
		q.IntervalSeconds += 10
		if i == 2 {
			q.Model = "changed-model"
		}
		q, err = svc.SaveQualityConfiguration(ctx, "user", q)
		if err != nil {
			t.Fatal(err)
		}
		read(i, false)
	}
	for _, enabled := range []bool{false, true} {
		q.Enabled = enabled
		q, err = svc.SaveQualityConfiguration(ctx, "user", q)
		if err != nil {
			t.Fatal(err)
		}
		read(3, false)
	}
	svc.runQualityScope(ctx, scope, tokens)
	read(4, true)
}
