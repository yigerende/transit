package connection_health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"transithub/backend/internal/modules/upstream"
	"transithub/backend/internal/shared/authctx"
)

func embedTestMonitor() EmbedMonitor {
	return EmbedMonitor{ID: "monitor", UserID: "user1", WorkspaceID: "ws1", GroupID: "42", Name: "Public name", GroupName: "Group", Model: "selected-model", Enabled: true,
		Health:  EmbedCheck{IntervalSeconds: 60, TimeoutSeconds: 20, ProbeMode: ProbeModeFirstToken},
		Candy:   EmbedCheck{IntervalSeconds: 300, TimeoutSeconds: 180, ReasoningEffort: "low"},
		Pelican: EmbedCheck{IntervalSeconds: 1800, TimeoutSeconds: 600, ReasoningEffort: "low"}}
}

func TestEmbedMonitorValidation(t *testing.T) {
	good := embedTestMonitor()
	if err := good.validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*EmbedMonitor){
		func(m *EmbedMonitor) { m.Health.ProbeMode = ProbeModeSub2API },
		func(m *EmbedMonitor) { m.Health.IntervalSeconds = 0 },
		func(m *EmbedMonitor) { m.Pelican.ReasoningEffort = "xhigh" },
		func(m *EmbedMonitor) { m.Candy.TimeoutSeconds = 601 },
		func(m *EmbedMonitor) { m.QualityBaseURL = "https://user:secret@example.com" },
		func(m *EmbedMonitor) { m.QualityBaseURL = "http://127.0.0.1" },
		func(m *EmbedMonitor) { m.Model = " " },
	} {
		m := good
		change(&m)
		if m.validate() == nil {
			t.Fatalf("accepted invalid config: %+v", m)
		}
	}
}

func embedTestRepository(t *testing.T) *embedMonitorRepository {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := newID()
	schema := "embed_test_" + strings.ReplaceAll(id, "-", "")
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`); admin.Close() })
	for _, file := range []string{"000036_embed_group_monitor.sql", "000037_embed_monitor_visibility.sql", "000038_embed_monitor_history_limit.sql"} {
		migration, err := os.ReadFile("../../database/migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if _, err = pool.Exec(ctx, string(migration)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return &embedMonitorRepository{db: pool}
}

func TestEmbedMonitorDurableJobsAndIsolation(t *testing.T) {
	r := embedTestRepository(t)
	ctx := context.Background()
	m := embedTestMonitor()
	if err := r.save(ctx, m); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.claim(ctx); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("auto-off task ran", err)
	}
	if err := r.queue(ctx, "other", m.WorkspaceID, m.ID, "candy"); err == nil {
		t.Fatal("cross-user queue allowed")
	}
	if err := r.queue(ctx, m.UserID, "other", m.ID, "candy"); err == nil {
		t.Fatal("cross-workspace queue allowed")
	}
	if err := r.queue(ctx, m.UserID, m.WorkspaceID, m.ID, "candy"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.claim(ctx, "health"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("health worker consumed a quality task", err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	claims := []EmbedJob{}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, job, err := r.claim(ctx)
			if err == nil {
				mu.Lock()
				claims = append(claims, job)
				mu.Unlock()
			} else if !errors.Is(err, pgx.ErrNoRows) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if len(claims) != 1 || !claims[0].Manual {
		t.Fatalf("duplicate/missing manual claim: %+v", claims)
	}
	j := claims[0]
	if err := r.queue(ctx, m.UserID, m.WorkspaceID, m.ID, "candy"); err == nil {
		t.Fatal("accepted overlapping test")
	}
	sample := EmbedSample{Kind: "candy", QualitySample: QualitySample{ID: j.RunID, TargetID: m.ID, Model: m.Model, StartedAt: j.StartedAt, CreatedAt: time.Now(), Result: "passed", HTML: "large artwork", HasHTML: true, Answer: "evidence"}}
	stale := j
	stale.RunID = "stale"
	if err := r.complete(ctx, m, stale, sample); err != nil {
		t.Fatal(err)
	}
	if _, err := r.detail(ctx, m.UserID, m.WorkspaceID, j.RunID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("stale worker wrote history")
	}
	if err := r.complete(ctx, m, j, sample); err != nil {
		t.Fatal(err)
	}
	board, err := r.board(ctx, m.UserID, m.WorkspaceID, "Title", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Cards) != 1 || len(board.Cards[0].History) != 1 || board.Cards[0].History[0].HTML != "" || board.Cards[0].History[0].Answer != "" || !board.Cards[0].History[0].HasHTML {
		t.Fatal("history summary incorrect")
	}
	if _, err := r.detail(ctx, "other", m.WorkspaceID, j.RunID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("foreign history exposed")
	}
	detail, err := r.detail(ctx, m.UserID, m.WorkspaceID, j.RunID)
	if err != nil || detail.HTML != "large artwork" {
		t.Fatal("detail lost", err)
	}
	// Update settings without changing identity or deleting history.
	m.Health.Enabled = true
	m.Health.IntervalSeconds = 90
	if err = r.save(ctx, m); err != nil {
		t.Fatal(err)
	}
	_, health, err := r.claim(ctx)
	if err != nil || health.Kind != "health" || health.Manual {
		t.Fatal("enabled schedule not due", err)
	}
	changed := m
	changed.Model = "another-model"
	if r.save(ctx, changed) == nil {
		t.Fatal("mixed model histories")
	}
	// Lease recovery also works for a manual-only test after a crashed process.
	if _, err = r.db.Exec(ctx, `UPDATE embed_group_monitor_jobs SET enabled=false,running_until=now()-interval '1 second' WHERE monitor_id=$1 AND kind='health'`, m.ID); err != nil {
		t.Fatal(err)
	}
	_, recovered, err := r.claim(ctx)
	if err != nil || recovered.RunID == health.RunID {
		t.Fatal("orphan task not recovered", err)
	}
	// Keep precisely the most recent sixty records per kind, independent of others.
	for i := 0; i < 65; i++ {
		now := time.Now().Add(time.Duration(i) * time.Second)
		id := fmt.Sprintf("retained-%03d", i)
		s := EmbedSample{Kind: "candy", QualitySample: QualitySample{ID: id, StartedAt: &now, CreatedAt: now, Result: "error"}}
		if _, err = r.db.Exec(ctx, `UPDATE embed_group_monitor_jobs SET run_id=$2,started_at=$3 WHERE monitor_id=$1 AND kind='candy'`, m.ID, id, now); err != nil {
			t.Fatal(err)
		}
		if err = r.complete(ctx, m, EmbedJob{Kind: "candy", RunID: id}, s); err != nil {
			t.Fatal(err)
		}
	}
	board, err = r.board(ctx, m.UserID, m.WorkspaceID, "Title", false)
	if err != nil || len(board.Cards[0].History) != 60 {
		t.Fatal("incorrect retention", err)
	}
	if board.Cards[0].History[0].ID != "retained-064" {
		t.Fatal("wrong history order")
	}
	if _, err = r.db.Exec(ctx, `DELETE FROM embed_group_monitors WHERE id=$1`, m.ID); err != nil {
		t.Fatal(err)
	}
	if err = r.complete(ctx, m, recovered, sample); err != nil {
		t.Fatal("completion resurrected deleted target", err)
	}
}

type embedQualityRecorder struct {
	credential upstream.ProbeCredential
	settings   QualitySettings
}

func (f *embedQualityRecorder) ProbeQuality(_ context.Context, c upstream.ProbeCredential, _ string, q QualitySettings, _ QualityQuestion) qualityProbeResult {
	f.credential = c
	f.settings = q
	return qualityProbeResult{Verdict: "passed", HTML: "<svg></svg>", DurationMS: 123, InputTokens: int64Ptr(50)}
}
func int64Ptr(v int64) *int64 { return &v }

func TestEmbedMonitorClearHistoryKeepsConfigurationAndScope(t *testing.T) {
	r := embedTestRepository(t)
	ctx := context.Background()
	m := embedTestMonitor()
	monitors := []EmbedMonitor{m, m, m, m}
	monitors[1].ID = "same-workspace"
	monitors[2].ID, monitors[2].WorkspaceID = "other-workspace", "ws2"
	monitors[3].ID, monitors[3].UserID = "other-user", "user2"
	for _, monitor := range monitors {
		if err := r.save(ctx, monitor); err != nil {
			t.Fatal(err)
		}
		if _, err := r.settings(ctx, monitor.UserID, monitor.WorkspaceID); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"health", "candy", "pelican"} {
			if _, err := r.db.Exec(ctx, `INSERT INTO embed_group_monitor_samples(id,monitor_id,kind,started_at,sample) VALUES($1,$2,$3,now(),'{"result":"passed","html":"artwork"}')`, monitor.ID+"-"+kind, monitor.ID, kind); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := r.queue(ctx, m.UserID, m.WorkspaceID, m.ID, "candy"); err != nil {
		t.Fatal(err)
	}
	_, running, err := r.claim(ctx, "candy")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.queue(ctx, m.UserID, m.WorkspaceID, m.ID, "health"); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		t.Helper()
		var result string
		err := r.db.QueryRow(ctx, `SELECT jsonb_build_array(
		 (SELECT jsonb_agg(to_jsonb(b) ORDER BY user_id,admin_account_id) FROM embed_group_monitor_boards b),
		 (SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM embed_group_monitors m),
		 (SELECT jsonb_agg(to_jsonb(j) ORDER BY monitor_id,kind) FROM embed_group_monitor_jobs j))::text`).Scan(&result)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := snapshot()
	deleted, err := r.clearHistory(ctx, m.UserID, m.WorkspaceID)
	if err != nil || deleted != 6 {
		t.Fatalf("clear history: deleted=%d err=%v", deleted, err)
	}
	if snapshot() != before {
		t.Fatal("clearing history changed settings, monitors, or scheduled/running jobs")
	}
	for i, monitor := range monitors {
		var count int
		if err := r.db.QueryRow(ctx, `SELECT count(*) FROM embed_group_monitor_samples WHERE monitor_id=$1`, monitor.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		want := 0
		if i >= 2 {
			want = 3
		}
		if count != want {
			t.Fatalf("monitor %s: got %d history rows, want %d", monitor.ID, count, want)
		}
	}
	if _, err := r.detail(ctx, m.UserID, m.WorkspaceID, m.ID+"-pelican"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("cleared artwork remains accessible", err)
	}
	if deleted, err = r.clearHistory(ctx, m.UserID, m.WorkspaceID); err != nil || deleted != 0 {
		t.Fatal("clearing empty history failed", deleted, err)
	}
	newSample := EmbedSample{Kind: "candy", QualitySample: QualitySample{ID: running.RunID, TargetID: m.ID, StartedAt: running.StartedAt, CreatedAt: time.Now(), Result: "passed"}}
	if err := r.complete(ctx, m, running, newSample); err != nil {
		t.Fatal("running detection could not finish after clear", err)
	}
	if _, err := r.detail(ctx, m.UserID, m.WorkspaceID, running.RunID); err != nil {
		t.Fatal("new detection result was not saved", err)
	}
}

func TestEmbedMonitorWorkerAndReadOnlyHTTP(t *testing.T) {
	r := embedTestRepository(t)
	ctx := context.Background()
	m := embedTestMonitor()
	m.QualityBaseURL = "https://public.example.com"
	if err := r.save(ctx, m); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepository()
	provider := &fakeGroupProbeProvider{fakePlatformGroupReader: fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "42", Name: "Group", Platform: ProviderOpenAI}}}}
	health := newAdminGroupsService(provider, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API, AccessToken: "jwt", BaseURL: "http://workspace.invalid"}}, repo)
	health.dispatcher = panicIfCalledRemoteActionRunner{}
	recorder := &embedQualityRecorder{}
	health.qualityRunner = recorder
	s := &EmbedMonitorService{repo: r, health: health}
	mux := http.NewServeMux()
	RegisterEmbedMonitorRoutes(mux, s)
	call := func(method, path, user, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if user != "" {
			req = req.WithContext(authctx.WithUserID(req.Context(), user))
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}
	if w := call("GET", "/api/group-quality/board", "", "", ""); w.Code != 401 {
		t.Fatal("admin authentication missing")
	}
	w := call("POST", "/api/group-quality/monitors/monitor/run/pelican", "user1", "", "")
	if w.Code != 202 || recorder.settings.DetectionMethod != "" {
		t.Fatal("manual HTTP should queue without waiting for external API", w.Code)
	}
	if w = call("PUT", "/api/group-quality/monitors/monitor", "foreign", "", `{"name":"changed"}`); w.Code == 200 {
		t.Fatal("foreign edit accepted")
	}
	claimed, j, err := r.claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.run(ctx, claimed, j)
	if recorder.credential.Key != "group-secret" || recorder.credential.BaseURL != m.QualityBaseURL || qualityProbeModel(recorder.settings) != manxuePelicanModel || !strings.HasPrefix(provider.name, "embed-group-") {
		t.Fatal("wrong group credential, URL or model")
	}
	if len(repo.events) != 0 || len(repo.states) != 0 || len(repo.budgetClaims) != 0 {
		t.Fatal("embedded detection affected existing automation")
	}
	detail, err := r.detail(ctx, m.UserID, m.WorkspaceID, j.RunID)
	if err != nil || detail.Result != "passed" || detail.Model != manxuePelicanModel || !detail.Manual {
		t.Fatal("worker result incorrect", err)
	}
	config, err := r.settings(ctx, m.UserID, m.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if w = call("GET", "/api/embed/group-quality", "", config.Token, ""); w.Code != 404 {
		t.Fatal("disabled publication exposed data")
	}
	config.Enabled = true
	config.VisibleMonitorIDs = []string{m.ID}
	config, err = s.saveSettings(ctx, m.UserID, m.WorkspaceID, config, false)
	if err != nil {
		t.Fatal(err)
	}
	w = call("GET", "/api/embed/group-quality", "", config.Token, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if strings.Contains(w.Body.String(), m.QualityBaseURL) || strings.Contains(w.Body.String(), "group-secret") {
		t.Fatal("private fields exposed")
	}
	w = call("POST", "/api/embed/group-quality", "", config.Token, "")
	if w.Code != 405 {
		t.Fatal("public mutation route exposed")
	}
	other := m
	other.ID = "other"
	other.UserID = "foreign"
	other.WorkspaceID = "foreign"
	if err = r.save(ctx, other); err != nil {
		t.Fatal(err)
	}
	old := config.Token
	config, err = s.saveSettings(ctx, m.UserID, m.WorkspaceID, config, true)
	if err != nil {
		t.Fatal(err)
	}
	if w = call("GET", "/api/embed/group-quality", "", old, ""); w.Code != 404 {
		t.Fatal("old token remains valid")
	}
	w = call("GET", "/api/embed/group-quality", "", config.Token, "")
	var board EmbedMonitorBoard
	if err = json.Unmarshal(w.Body.Bytes(), &board); err != nil || len(board.Cards) != 1 {
		t.Fatal("public workspace isolation failed", err)
	}
	if w = call("DELETE", "/api/group-quality/history", "", config.Token, ""); w.Code != 401 {
		t.Fatal("public token can clear admin history", w.Code)
	}
	if w = call("DELETE", "/api/embed/group-quality", "", config.Token, ""); w.Code != 405 {
		t.Fatal("public mutation allowed", w.Code)
	}
	if w = call("DELETE", "/api/group-quality/history", m.UserID, "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"deleted":1`) {
		t.Fatal("authenticated clear failed", w.Code, w.Body.String())
	}
	w = call("GET", "/api/embed/group-quality", "", config.Token, "")
	if err = json.Unmarshal(w.Body.Bytes(), &board); w.Code != 200 || err != nil || len(board.Cards) != 1 || len(board.Cards[0].History) != 0 {
		t.Fatal("public board did not reflect clear or lost its monitor", w.Code, err)
	}
}
