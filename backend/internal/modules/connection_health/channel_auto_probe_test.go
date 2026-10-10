package connection_health

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"transithub/backend/internal/shared/authctx"
)

func (f *fakeRepository) GetChannelAutoProbe(_ context.Context, user, workspace, target string) (bool, error) {
	enabled, exists := f.channelAutoProbes[user+"|"+workspace+"|"+target]
	return !exists || enabled, nil
}
func (f *fakeRepository) ListChannelAutoProbes(_ context.Context, user, workspace string) (map[string]bool, error) {
	result := map[string]bool{}
	prefix := user + "|" + workspace + "|"
	for key, enabled := range f.channelAutoProbes {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result[key[len(prefix):]] = enabled
		}
	}
	return result, nil
}
func (f *fakeRepository) SetChannelAutoProbe(_ context.Context, user, workspace, target string, enabled bool) error {
	f.channelAutoProbes[user+"|"+workspace+"|"+target] = enabled
	return nil
}

func TestChannelAutoProbeHandlerRequiresExplicitBoolean(t *testing.T) {
	svc, _, _, _ := qualityTestService(t)
	h := &Handler{service: svc}
	for _, tt := range []struct {
		body          string
		authenticated bool
		status        int
	}{{`{"enabled":false}`, false, 401}, {`{}`, true, 400}, {`{"enabled":null}`, true, 400}, {`{"enabled":"false"}`, true, 400}, {`{"enabled":false}`, true, 200}} {
		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(tt.body))
		r.SetPathValue("id", "sub2api:ws1:a")
		if tt.authenticated {
			r = r.WithContext(authctx.WithUserID(r.Context(), "user"))
		}
		w := httptest.NewRecorder()
		h.setChannelAutoProbe(w, r)
		if w.Code != tt.status {
			t.Fatalf("%s status=%d want=%d", tt.body, w.Code, tt.status)
		}
	}
}

func TestChannelAutoProbePostgresPersistence(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	migration, err := os.ReadFile("../../database/migrations/000031_channel_auto_probe_switches.sql")
	if err != nil || strings.TrimSpace(strings.ReplaceAll(string(migration), "\r\n", "\n")) != strings.TrimSpace(channelAutoProbeSchema) {
		t.Fatal("schema migration differs", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := pool.Exec(ctx, strings.ReplaceAll(channelAutoProbeSchema, "CREATE TABLE IF NOT EXISTS", "CREATE TEMP TABLE IF NOT EXISTS")); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewRepository(pool)
	if enabled, err := repo.GetChannelAutoProbe(ctx, "u", "w", "channel"); err != nil || !enabled {
		t.Fatal("existing channels must default to following policy")
	}
	for _, enabled := range []bool{false, true, false} {
		if err := repo.SetChannelAutoProbe(ctx, "u", "w", "channel", enabled); err != nil {
			t.Fatal(err)
		}
		loaded, err := NewRepository(pool).GetChannelAutoProbe(ctx, "u", "w", "channel")
		if err != nil || loaded != enabled {
			t.Fatal("preference did not survive repository reload")
		}
	}
	for _, scope := range [][2]string{{"other", "w"}, {"u", "other"}} {
		if enabled, err := repo.GetChannelAutoProbe(ctx, scope[0], scope[1], "channel"); err != nil || !enabled {
			t.Fatal("preference leaked to another scope")
		}
	}
	settings, err := repo.ListChannelAutoProbes(ctx, "u", "w")
	if err != nil || len(settings) != 1 || settings["channel"] {
		t.Fatal("preference missing from bulk read")
	}
}

func TestChannelAutoProbeSharedDisplayAndScope(t *testing.T) {
	ctx := context.Background()
	svc, _, repo, _ := qualityTestService(t)
	target := "sub2api:ws1:a"
	for _, enabled := range []bool{false, true} {
		saved, err := svc.SetChannelAutoProbe(ctx, "user", target, enabled)
		if err != nil || saved.Enabled != enabled {
			t.Fatalf("save: %+v %v", saved, err)
		}
		groups, err := svc.AdminGroups(ctx, "user")
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, g := range groups {
			for _, a := range g.Accounts {
				if a.TargetID != target {
					continue
				}
				found++
				if a.AutoProbeEnabled != (enabled && a.HasEnabledProbePolicy) {
					t.Fatalf("incorrect shared display: %+v", a)
				}
			}
		}
		if found < 2 {
			t.Fatal("fixture must share a channel across groups")
		}
	}
	for _, target := range []string{"sub2api:other:a", "newapi:ws1:a", "sub2api:ws1:missing", "bad"} {
		if _, err := svc.SetChannelAutoProbe(ctx, "user", target, false); err == nil {
			t.Fatal("accepted foreign/absent target")
		}
	}
	if len(repo.groupAssignments) != 2 || !repo.policies[0].Enabled {
		t.Fatal("switch altered policy assignments")
	}
}

func TestChannelAutoProbeStopsQueuedWorkAndResumes(t *testing.T) {
	ctx := context.Background()
	svc, repo, reader := cadenceFixture(2)
	jobs := svc.collectAdminProbeJobsWithGroups(ctx, repo.policies, nil, repo.groupAssignments, nil)
	if len(jobs) != 2 {
		t.Fatalf("expected two jobs, got %d", len(jobs))
	}
	target := jobs[0].target.TargetID
	_ = repo.SetChannelAutoProbe(ctx, "user1", "ws1", target, false)
	svc.runAdminProbeJob(ctx, jobs[0])
	if reader.credentials.Load() != 0 || len(repo.events) != 0 {
		t.Fatal("queued disabled job resolved credentials or recorded probes")
	}
	remaining := svc.collectAdminProbeJobsWithGroups(ctx, repo.policies, nil, repo.groupAssignments, nil)
	if len(remaining) != 1 || remaining[0].target.TargetID == target {
		t.Fatal("disabled channel scheduled, or other channel blocked")
	}
	_ = repo.SetChannelAutoProbe(ctx, "user1", "ws1", target, true)
	resumed := svc.collectAdminProbeJobsWithGroups(ctx, repo.policies, nil, repo.groupAssignments, nil)
	if len(resumed) != 2 {
		t.Fatal("enabled channel did not resume")
	}
}

func TestChannelAutoProbeSwitchStopsNextModelKeepsCompletedHistory(t *testing.T) {
	ctx := context.Background()
	svc, repo, reader := cadenceFixture(2)
	reader.accountsByGrp["g1"][0].Models = "model-0,model-1"
	reader.accountsByGrp["g1"] = reader.accountsByGrp["g1"][:1]
	jobs := svc.collectAdminProbeJobsWithGroups(ctx, repo.policies, nil, repo.groupAssignments, nil)
	if len(jobs) != 1 || len(jobs[0].dueSpecs) != 2 {
		t.Fatal("expected one channel with two models")
	}
	requests := 0
	svc.probeRunner.client.Transport = latencyTestTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		_ = repo.SetChannelAutoProbe(ctx, "user1", "ws1", jobs[0].target.TargetID, false)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)), Header: http.Header{}}, nil
	})
	svc.runAdminProbeJob(ctx, jobs[0])
	if requests != 1 || len(repo.events) != 1 || repo.events[0].Manual {
		t.Fatalf("requests=%d events=%+v", requests, repo.events)
	}
	if used, _ := repo.CountProbesToday(ctx, "user1", "ws1", "p1", jobs[0].target.TargetID, probeBudgetDayStart(repo.events[0].CreatedAt)); used != 1 {
		t.Fatal("disabled model consumed quota")
	}
}

type unavailableAutoProbeRepo struct{ *fakeRepository }

func (r unavailableAutoProbeRepo) GetChannelAutoProbe(context.Context, string, string, string) (bool, error) {
	return true, errors.New("unavailable")
}
func (r unavailableAutoProbeRepo) ListChannelAutoProbes(context.Context, string, string) (map[string]bool, error) {
	return nil, errors.New("unavailable")
}
func TestChannelAutoProbeReadFailureSkipsAutomaticWork(t *testing.T) {
	ctx := context.Background()
	svc, repo, reader := cadenceFixture(1)
	jobs := svc.collectAdminProbeJobsWithGroups(ctx, repo.policies, nil, repo.groupAssignments, nil)
	svc.repo = unavailableAutoProbeRepo{repo.fakeRepository}
	svc.runAdminProbeJob(ctx, jobs[0])
	if reader.credentials.Load() != 0 || len(repo.events) != 0 {
		t.Fatal("read error allowed probe")
	}
	if got := svc.collectAdminProbeJobsWithGroups(ctx, repo.policies, nil, repo.groupAssignments, nil); len(got) != 0 {
		t.Fatal("read error scheduled probe")
	}
}
