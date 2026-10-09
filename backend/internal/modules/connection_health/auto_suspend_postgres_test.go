package connection_health

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAutoSuspendPostgresMigrationAndPersistence(t *testing.T) {
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
	schema := "test_suspend_" + strings.ReplaceAll(id, "-", "")
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	repo := NewRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	policy, targets, err := buildPolicyAndTargets("user", "site", "p1", PolicyInput{Name: "old", Enabled: true, AutoDegradeEnabled: true, AutoRemoteActionEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SavePolicyWithTargets(ctx, policy, targets); err != nil {
		t.Fatal(err)
	}
	// Upgrade an existing policy created before the new column existed.
	if _, err := pool.Exec(ctx, "ALTER TABLE connection_health_policies DROP COLUMN auto_suspend_enabled"); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../database/migrations/000021_connection_health_auto_suspend.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.GetPolicy(ctx, "p1", "user", "site")
	if err != nil || loaded == nil || loaded.AutoSuspendEnabled {
		t.Fatalf("migration must default existing policies off: %v %+v", err, loaded)
	}
	for _, enabled := range []bool{true, false} {
		policy.AutoSuspendEnabled = enabled
		if err := repo.SavePolicyWithTargets(ctx, policy, targets); err != nil {
			t.Fatal(err)
		}
		loaded, err = repo.GetPolicy(ctx, "p1", "user", "site")
		if err != nil || loaded == nil || loaded.AutoSuspendEnabled != enabled {
			t.Fatal("GetPolicy lost suspension setting")
		}
		listed, err := repo.ListPolicies(ctx, "user", "site")
		if err != nil || len(listed) != 1 || listed[0].AutoSuspendEnabled != enabled {
			t.Fatalf("ListPolicies lost setting: %v", err)
		}
		listed, err = repo.ListEnabledPolicies(ctx)
		if err != nil || len(listed) != 1 || listed[0].AutoSuspendEnabled != enabled {
			t.Fatalf("scheduler lost setting: %v", err)
		}
	}
	policy.ID = "wizard"
	policy.AutoSuspendEnabled = true
	if err := repo.CreatePolicyAndReplaceGroupConfiguration(ctx, policy, nil, "g1", "group", []string{"wizard"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	loaded, err = repo.GetPolicy(ctx, "wizard", "user", "site")
	if err != nil || loaded == nil || !loaded.AutoSuspendEnabled {
		t.Fatalf("wizard lost setting: %v", err)
	}
}
