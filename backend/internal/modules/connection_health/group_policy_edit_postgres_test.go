package connection_health

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEditGroupPolicyPostgresAtomicSave(t *testing.T) {
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
	schema := "test_group_edit_" + strings.ReplaceAll(id, "-", "")
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
	policy, targets, err := buildPolicyAndTargets("user", "site", "p1", PolicyInput{
		Name: "before", Enabled: true,
		ModelTargets: []ModelTargetInput{{ModelName: "old-model", ProviderFamily: ProviderOpenAI, Enabled: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreatePolicyAndReplaceGroupConfiguration(ctx, policy, targets, "g1", "group", []string{"p1"}, []string{"old-exclusion"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceGroupPolicyConfiguration(ctx, "user", "site", "g2", "other group", []string{"p1"}, []string{"other-exclusion"}, nil); err != nil {
		t.Fatal(err)
	}
	// Fail the last part of the transaction, after policy, models and bindings changed.
	if _, err := pool.Exec(ctx, `ALTER TABLE connection_health_group_target_exclusions
		ADD CONSTRAINT reject_test_exclusion CHECK (target_id <> 'reject-me')`); err != nil {
		t.Fatal(err)
	}
	policy.Name = "after"
	policy.AutoSuspendEnabled = true
	targets[0].ModelName = "new-model"
	err = repo.UpdatePolicyAndReplaceGroupConfiguration(ctx, policy, targets, "g1", "changed group", []string{"p1"}, []string{"reject-me"}, nil)
	if err == nil {
		t.Fatal("expected exclusion constraint to fail")
	}
	loaded, err := repo.GetPolicy(ctx, "p1", "user", "site")
	if err != nil || loaded == nil || loaded.Name != "before" || loaded.AutoSuspendEnabled || len(loaded.ModelTargets) != 1 || loaded.ModelTargets[0].ModelName != "old-model" {
		t.Fatalf("failed selection write must roll back policy and models: %+v %v", loaded, err)
	}
	assignments, err := repo.ListGroupPolicyAssignmentsByWorkspace(ctx, "user", "site")
	if err != nil || len(assignments) != 2 {
		t.Fatalf("lost assignments: %+v %v", assignments, err)
	}
	for _, assignment := range assignments {
		if assignment.AdminGroupID == "g1" && assignment.AdminGroupName != "group" {
			t.Fatal("failed save did not roll back group bindings")
		}
	}
	exclusions, err := repo.ListGroupTargetExclusionsByWorkspace(ctx, "user", "site")
	if err != nil || len(exclusions) != 2 {
		t.Fatalf("lost exclusions: %+v %v", exclusions, err)
	}
	for _, exclusion := range exclusions {
		if exclusion.AdminGroupID == "g1" && exclusion.TargetID != "old-exclusion" {
			t.Fatal("failed save did not roll back selection")
		}
	}
	if err := repo.UpdatePolicyAndReplaceGroupConfiguration(ctx, policy, targets, "g1", "group", []string{"p1"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	loaded, err = repo.GetPolicy(ctx, "p1", "user", "site")
	if err != nil || loaded == nil || loaded.Name != "after" || !loaded.AutoSuspendEnabled || loaded.ModelTargets[0].ModelName != "new-model" {
		t.Fatalf("successful save lost policy or model changes: %+v %v", loaded, err)
	}
	exclusions, err = repo.ListGroupTargetExclusionsByWorkspace(ctx, "user", "site")
	if err != nil || len(exclusions) != 1 || exclusions[0].AdminGroupID != "g2" || exclusions[0].TargetID != "other-exclusion" {
		t.Fatalf("reselection should clear only current group exclusions: %+v %v", exclusions, err)
	}
	for _, invalid := range []Policy{
		{ID: "p1", UserID: "other", AdminAccountID: "site"},
		{ID: "p1", UserID: "user", AdminAccountID: "other"},
		{ID: "missing", UserID: "user", AdminAccountID: "site"},
	} {
		err := repo.UpdatePolicyAndReplaceGroupConfiguration(ctx, invalid, nil, "g1", "group", nil, nil, nil)
		if err == nil || err.Error() != ErrorPolicyNotFound {
			t.Fatalf("missing/foreign policy must not be upserted: %v", err)
		}
	}
}
