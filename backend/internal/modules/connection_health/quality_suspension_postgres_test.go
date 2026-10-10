package connection_health

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestQualitySuspensionPostgresMigrationAndPersistence(t *testing.T) {
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
	schema := "test_quality_suspend_" + strings.ReplaceAll(id, "-", "")
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
	state := TargetActionState{UserID: "user", AdminAccountID: "ws", TargetID: "sub2api:ws:a", OriginalStatus: "active", LastAppliedStatus: "inactive"}
	if err = repo.UpsertTargetActionState(ctx, state); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "ALTER TABLE connection_health_target_action_states DROP COLUMN quality_suspended"); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../database/migrations/000029_quality_suspension.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = pool.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := repo.GetTargetActionState(ctx, "user", "ws", state.TargetID)
	if err != nil || loaded == nil || loaded.QualitySuspended {
		t.Fatalf("upgrade changed existing hold: %+v, %v", loaded, err)
	}
	q, err := repo.GetQualitySettings(ctx, "user", "ws")
	if err != nil || q.AutoSuspendEnabled {
		t.Fatal("quality actions must default off")
	}
	for _, enabled := range []bool{true, false} {
		state.QualitySuspended = enabled
		state.PendingStatus = "inactive"
		if err = repo.UpsertTargetActionState(ctx, state); err != nil {
			t.Fatal(err)
		}
		loaded, err = repo.GetTargetActionState(ctx, "user", "ws", state.TargetID)
		if err != nil || loaded == nil || loaded.QualitySuspended != enabled || loaded.PendingStatus != "inactive" {
			t.Fatalf("get lost checkpoint: %+v, %v", loaded, err)
		}
		listed, err := repo.ListTargetActionStates(ctx, "user", "ws")
		if err != nil || len(listed) != 1 || listed[0].QualitySuspended != enabled {
			t.Fatalf("workspace lost hold: %+v, %v", listed, err)
		}
		listed, err = repo.ListAllTargetActionStates(ctx)
		if err != nil || len(listed) != 1 || listed[0].QualitySuspended != enabled {
			t.Fatalf("scheduler lost hold: %+v, %v", listed, err)
		}
		q.AutoSuspendEnabled, q.FailureLimit, q.RecoveryLimit, q.Revision = enabled, 3, 4, "revision"
		if err = repo.SaveQualitySettings(ctx, "user", "ws", q); err != nil {
			t.Fatal(err)
		}
		read, err := repo.GetQualitySettings(ctx, "user", "ws")
		if err != nil || read.AutoSuspendEnabled != enabled || read.FailureLimit != 3 || read.RecoveryLimit != 4 {
			t.Fatalf("quality settings lost: %+v, %v", read, err)
		}
	}
	if other, err := repo.GetTargetActionState(ctx, "other", "ws", state.TargetID); err != nil || other != nil {
		t.Fatal("hold leaked across users")
	}
}
