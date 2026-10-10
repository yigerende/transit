package connection_health

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLatencyPriorityPostgresPersistenceAndSamples(t *testing.T) {
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
	schema := "test_latency_" + strings.ReplaceAll(id, "-", "")
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
	if err = repo.EnsureSchema(ctx); err != nil {
		t.Fatal("schema upgrade is not idempotent", err)
	}
	p := latencyTestPolicy()
	p.LatencyPriority.Weights = []float64{6, 3, 2, 1, 1}
	p.LatencyPriority.MaxAgeSeconds = 240
	p.LatencyPriority.Bands[1].Priority = 21
	if err = repo.SavePolicyWithTargets(ctx, p, p.ModelTargets); err != nil {
		t.Fatal(err)
	}
	saved, err := repo.GetPolicy(ctx, p.ID, p.UserID, p.AdminAccountID)
	if err != nil || saved == nil || !reflect.DeepEqual(saved.LatencyPriority, p.LatencyPriority) {
		t.Fatalf("config round trip: %+v %v", saved, err)
	}
	q := p
	q.ID = "quick"
	if err = repo.CreatePolicyAndReplaceGroupConfiguration(ctx, q, nil, "g", "group", []string{q.ID}, nil, nil); err != nil {
		t.Fatal(err)
	}
	q.LatencyPriority = defaultLatencyPriorityConfig()
	q.LatencyPriority.SampleCount = 2
	q.LatencyPriority.Weights = []float64{3, 2}
	if err = repo.UpdatePolicyAndReplaceGroupConfiguration(ctx, q, nil, "g", "group", []string{q.ID}, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, list := range []func(context.Context) ([]Policy, error){repo.ListEnabledPolicies, func(ctx context.Context) ([]Policy, error) { return repo.ListPolicies(ctx, p.UserID, p.AdminAccountID) }} {
		policies, err := list(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(policies) != 2 {
			t.Fatal("missing policies")
		}
		for _, policy := range policies {
			if policy.ID == q.ID && !reflect.DeepEqual(policy.LatencyPriority, q.LatencyPriority) {
				t.Fatal("atomic edit lost config")
			}
		}
	}
	now := time.Now()
	target := "sub2api:ws1:100"
	for _, tt := range []struct {
		id, user, workspace, policy, result, model string
		ms                                         int
		age                                        time.Duration
	}{
		{"valid", "user1", "ws1", "p1", "ok", "model", 7000, 0},
		{"other-model", "user1", "ws1", "p1", "ok", "other", 20, 0},
		{"error", "user1", "ws1", "p1", "server_error", "model", 20, 0},
		{"manual", "user1", "ws1", "", "ok", "model", 20, 0},
		{"expired", "user1", "ws1", "p1", "ok", "model", 20, 181 * time.Second},
		{"other-user", "user2", "ws1", "p1", "ok", "model", 20, 0},
		{"other-workspace", "user1", "ws2", "p1", "ok", "model", 20, 0},
	} {
		e := ConnectionHealthEvent{ID: tt.id, ConnectionID: target, UserID: tt.user, AdminAccountID: tt.workspace, PolicyID: tt.policy, Result: tt.result, ModelName: tt.model, LatencyMs: intPtr(tt.ms)}
		if err := repo.InsertEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE connection_health_events SET created_at=$1 WHERE id=$2`, now.Add(-tt.age), tt.id); err != nil {
			t.Fatal(err)
		}
	}
	samples, err := repo.ListPriorityProbeSamples(ctx, "user1", "ws1", []string{target}, now.Add(-180*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 2 {
		t.Fatalf("incorrect scope/result/time filtering: %+v", samples)
	}
	d := evaluateLatencyPriority(p, nil, samples, nil, now)
	if d.SampleCount != 1 || *d.AverageMs != 7000 || d.Priority != 21 {
		t.Fatalf("database-backed decision: %+v", d)
	}
	// Changing methods must preserve historical units and must not allow twenty
	// recent full responses to crowd a first-token sample out of the SQL window.
	for _, mode := range []string{ProbeModeLight, ProbeModeArithmetic, ProbeModeSub2API, ProbeModeFirstToken} {
		p.ProbeMode = mode
		if err := repo.SavePolicyWithTargets(ctx, p, p.ModelTargets); err != nil {
			t.Fatal(err)
		}
		readBack, err := repo.GetPolicy(ctx, p.ID, p.UserID, p.AdminAccountID)
		if err != nil || readBack.ProbeMode != mode {
			t.Fatalf("probe mode not persisted: %v", err)
		}
	}
	for i := 0; i < 30; i++ {
		mode := ProbeModeArithmetic
		if i == 0 {
			mode = ProbeModeFirstToken
		}
		err := repo.InsertEvent(ctx, ConnectionHealthEvent{ID: fmt.Sprint("method-", i), ConnectionID: target, UserID: "user1", AdminAccountID: "ws1", PolicyID: p.ID, ModelName: "model", Result: "ok", LatencyMs: intPtr(1234), ProbeMode: mode})
		if err != nil {
			t.Fatal(err)
		}
	}
	samples, err = repo.ListPriorityProbeSamples(ctx, "user1", "ws1", []string{target}, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	d = evaluateLatencyPriority(p, nil, samples, nil, time.Now())
	if d.SampleCount != 1 || d.AverageMs == nil || *d.AverageMs != 1234 {
		t.Fatalf("probe methods mixed/starved: %+v", d)
	}
	history, err := repo.ListRecentProbesByTargets(ctx, "user1", "ws1", []string{target})
	if err != nil || history[0].ProbeMode != ProbeModeArithmetic {
		t.Fatalf("history lost method: %v", err)
	}
	events, err := repo.ListEventsByConnection(ctx, target, "user1", "ws1", 100)
	if err != nil || events[0].ProbeMode != ProbeModeArithmetic {
		t.Fatalf("event details lost method: %v", err)
	}
}
