package connection_health

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGroupProbeConfigPostgres(t *testing.T) {
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	poolConfig.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, strings.Replace(groupProbeConfigSchema, "CREATE TABLE IF NOT EXISTS", "CREATE TEMP TABLE", 1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO connection_health_group_probe_configs(user_id,admin_account_id,group_id,model) VALUES('legacy','workspace','42','model')`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../database/migrations/000033_group_probe_modes.sql")
	if err != nil || strings.TrimSpace(strings.ReplaceAll(string(migration), "\r\n", "\n")) != groupProbeModeSchema {
		t.Fatal("group probe mode migration differs", err)
	}
	for i := 0; i < 2; i++ {
		if _, err = pool.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewRepository(pool)
	legacy, err := repo.GetGroupProbeConfig(ctx, "legacy", "workspace", "42")
	if err != nil || legacy == nil || legacy.ProbeMode != ProbeModeLight {
		t.Fatal("migration changed existing probe method")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM connection_health_group_probe_configs WHERE user_id='legacy'`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	c := GroupProbeConfig{UserID: "user", AdminAccountID: "workspace", GroupID: "42", Model: "model", IntervalSeconds: 60, Enabled: true, NextProbeAt: &now, CustomKeyID: "17"}
	if err = repo.SaveGroupProbeConfig(ctx, c); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewRepository(pool).GetGroupProbeConfig(ctx, "user", "workspace", "42")
	if err != nil || loaded == nil || !loaded.HasCustomKey || loaded.CustomKeyID != "17" {
		t.Fatalf("roundtrip failed: %+v %v", loaded, err)
	}
	for _, mode := range []string{ProbeModeArithmetic, ProbeModeFirstToken, ProbeModeLight} {
		c.ProbeMode = mode
		if err = repo.SaveGroupProbeConfig(ctx, c); err != nil {
			t.Fatal(err)
		}
		loaded, err = NewRepository(pool).GetGroupProbeConfig(ctx, "user", "workspace", "42")
		if err != nil || loaded == nil || loaded.ProbeMode != mode || loaded.CustomKeyID != "17" {
			t.Fatal("method/key did not survive reload")
		}
		configs, listErr := repo.ListGroupProbeConfigs(ctx, "user", "workspace")
		due, dueErr := repo.ListDueGroupProbeConfigs(ctx, now)
		if listErr != nil || dueErr != nil || len(configs) != 1 || len(due) != 1 || configs[0].ProbeMode != mode || due[0].ProbeMode != mode {
			t.Fatal("method missing from scheduled/listed config")
		}
	}
	for _, scope := range [][2]string{{"other", "workspace"}, {"user", "other"}} {
		foreign, err := repo.GetGroupProbeConfig(ctx, scope[0], scope[1], "42")
		if err != nil || foreign != nil {
			t.Fatal("config crossed workspace/user")
		}
		configs, err := repo.ListGroupProbeConfigs(ctx, scope[0], scope[1])
		if err != nil || len(configs) != 0 {
			t.Fatal("list crossed workspace/user")
		}
	}
	due, err := repo.ListDueGroupProbeConfigs(ctx, now)
	if err != nil || len(due) != 1 {
		t.Fatal("due task not found")
	}
	if err = repo.CompleteGroupProbe(ctx, c, now, "server_error"); err != nil {
		t.Fatal(err)
	}
	loaded, _ = repo.GetGroupProbeConfig(ctx, "user", "workspace", "42")
	if loaded.LastErrorKey != "server_error" || loaded.NextProbeAt.Sub(*loaded.LastProbeAt) != 60*time.Second {
		t.Fatal("completion did not persist interval/error")
	}
	due, err = repo.ListDueGroupProbeConfigs(ctx, now)
	if err != nil || len(due) != 0 {
		t.Fatal("future task returned as due")
	}
	loaded.Enabled = false
	loaded.NextProbeAt = nil
	if err = repo.SaveGroupProbeConfig(ctx, *loaded); err != nil {
		t.Fatal(err)
	}
	due, err = repo.ListDueGroupProbeConfigs(ctx, now.Add(time.Hour))
	if err != nil || len(due) != 0 {
		t.Fatal("paused task returned as due")
	}
	_, err = pool.Exec(ctx, `DELETE FROM connection_health_group_probe_configs`)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.CompleteGroupProbe(ctx, c, now, ""); err != nil {
		t.Fatal(err)
	}
	loaded, err = repo.GetGroupProbeConfig(ctx, "user", "workspace", "42")
	if err != nil || loaded != nil {
		t.Fatal("completion resurrected deleted task")
	}
}
