package connection_health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"transithub/backend/internal/modules/upstream"
	"transithub/backend/internal/shared/authctx"
)

func TestEmbedMonitorVisibilityScopeAndPublicDetails(t *testing.T) {
	r := embedTestRepository(t)
	ctx := context.Background()
	monitors := []EmbedMonitor{embedTestMonitor(), embedTestMonitor(), embedTestMonitor(), embedTestMonitor()}
	monitors[1].ID, monitors[1].Name = "hidden", "Private test"
	monitors[2].ID, monitors[2].WorkspaceID = "other-workspace", "ws2"
	monitors[3].ID, monitors[3].UserID = "other-user", "user2"
	for _, monitor := range monitors {
		if err := r.save(ctx, monitor); err != nil {
			t.Fatal(err)
		}
		sample := EmbedSample{Kind: "pelican", QualitySample: QualitySample{ID: monitor.ID + "-sample", TargetID: monitor.ID, HTML: "<p>artwork</p>", HasHTML: true, Result: "passed"}}
		data, _ := json.Marshal(sample)
		if _, err := r.db.Exec(ctx, `INSERT INTO embed_group_monitor_samples(id,monitor_id,kind,started_at,sample) VALUES($1,$2,'pelican',now(),$3)`, sample.ID, monitor.ID, data); err != nil {
			t.Fatal(err)
		}
	}
	m := monitors[0]
	health := newAdminGroupsService(&fakeGroupProbeProvider{}, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API}}, newFakeRepository())
	service := &EmbedMonitorService{repo: r, health: health}
	mux := http.NewServeMux()
	RegisterEmbedMonitorRoutes(mux, service)
	call := func(path, user, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		if user != "" {
			req = req.WithContext(authctx.WithUserID(req.Context(), user))
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	config, err := r.settings(ctx, m.UserID, m.WorkspaceID)
	if err != nil || config.VisibleMonitorIDs == nil || len(config.VisibleMonitorIDs) != 0 {
		t.Fatal("new monitors must be private by default", config, err)
	}
	config.Enabled = true
	config.VisibleMonitorIDs = []string{m.ID, m.ID}
	config, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, config, false)
	if err != nil || !reflect.DeepEqual(config.VisibleMonitorIDs, []string{m.ID}) {
		t.Fatal("selection not saved/deduplicated", config, err)
	}
	readBoard := func(want int) {
		t.Helper()
		response := call("/api/embed/group-quality", "", config.Token)
		var board EmbedMonitorBoard
		if err := json.Unmarshal(response.Body.Bytes(), &board); response.Code != 200 || err != nil || len(board.Cards) != want {
			t.Fatalf("public board: status=%d cards=%d err=%v", response.Code, len(board.Cards), err)
		}
		if want == 1 && board.Cards[0].Monitor.ID != m.ID {
			t.Fatal("wrong public monitor")
		}
	}
	readBoard(1)
	for _, monitor := range monitors {
		response := call("/api/embed/group-quality/samples/"+monitor.ID+"-sample", "", config.Token)
		want := 404
		if monitor.ID == m.ID {
			want = 200
		}
		if response.Code != want {
			t.Fatalf("public detail %s: got %d want %d", monitor.ID, response.Code, want)
		}
	}
	if response := call("/api/group-quality/samples/hidden-sample", m.UserID, ""); response.Code != 200 {
		t.Fatal("private history unavailable to administrator")
	}
	var adminBoard EmbedMonitorBoard
	response := call("/api/group-quality/board", m.UserID, "")
	if err := json.Unmarshal(response.Body.Bytes(), &adminBoard); err != nil || len(adminBoard.Cards) != 2 {
		t.Fatal("admin list lost hidden monitor", err)
	}
	// Foreign, deleted and malformed IDs cannot partially update title or publication.
	for _, ids := range [][]string{{"other-workspace"}, {"other-user"}, {"deleted"}, {""}} {
		bad := config
		bad.Title = "must not save"
		bad.VisibleMonitorIDs = ids
		if _, err := service.saveSettings(ctx, m.UserID, m.WorkspaceID, bad, false); err == nil {
			t.Fatal("invalid visibility accepted", ids)
		}
		saved, err := r.settings(ctx, m.UserID, m.WorkspaceID)
		if err != nil || saved.Title != config.Title || !reflect.DeepEqual(saved.VisibleMonitorIDs, config.VisibleMonitorIDs) {
			t.Fatal("invalid save changed configuration", saved, err)
		}
	}
	// Editing an item and rotating its board token must retain selection.
	if err := r.save(ctx, m); err != nil {
		t.Fatal(err)
	}
	oldToken := config.Token
	config, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, EmbedBoardSettings{}, true)
	if err != nil || config.Token == oldToken || !reflect.DeepEqual(config.VisibleMonitorIDs, []string{m.ID}) {
		t.Fatal("rotation changed publication", err)
	}
	if call("/api/embed/group-quality", "", oldToken).Code != 404 {
		t.Fatal("old token still works")
	}
	readBoard(1)
	// Empty means hide all, while missing input from an old client preserves selection.
	legacy := config
	legacy.VisibleMonitorIDs = nil
	config, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, legacy, false)
	if err != nil {
		t.Fatal(err)
	}
	readBoard(1)
	config.VisibleMonitorIDs = []string{}
	config, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, config, false)
	if err != nil {
		t.Fatal(err)
	}
	readBoard(0)
	if call("/api/embed/group-quality/samples/"+m.ID+"-sample", "", config.Token).Code != 404 {
		t.Fatal("hidden detail still publicly accessible")
	}
	config.VisibleMonitorIDs = []string{m.ID}
	config, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, config, false)
	if err != nil {
		t.Fatal(err)
	}
	readBoard(1)
	if response := call("/api/embed/group-quality/samples/"+m.ID+"-sample", "", config.Token); response.Code != 200 || !strings.Contains(response.Body.String(), "artwork") {
		t.Fatal("republished history missing")
	}
	config.Enabled = false
	if _, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, config, false); err != nil {
		t.Fatal(err)
	}
	if call("/api/embed/group-quality/samples/"+m.ID+"-sample", "", config.Token).Code != 404 {
		t.Fatal("disabled board detail exposed")
	}
	remove := func(user string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("DELETE", "/api/group-quality/monitors/"+m.ID, nil)
		if user != "" {
			req = req.WithContext(authctx.WithUserID(req.Context(), user))
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	if remove("").Code != 401 || remove("foreign").Code == 200 {
		t.Fatal("deletion authentication/scope missing")
	}
	if err := r.queue(ctx, m.UserID, m.WorkspaceID, m.ID, "candy"); err != nil {
		t.Fatal(err)
	}
	if response := remove(m.UserID); response.Code != 200 {
		t.Fatal("deletion failed", response.Body.String())
	}
	for _, table := range []string{"embed_group_monitor_jobs", "embed_group_monitor_samples"} {
		var count int
		if err := r.db.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE monitor_id=$1", m.ID).Scan(&count); err != nil || count != 0 {
			t.Fatal("deleted monitor left jobs or history", table, count, err)
		}
	}
	remaining, err := r.board(ctx, m.UserID, m.WorkspaceID, "Title", false)
	if err != nil || len(remaining.Cards) != 1 || remaining.Cards[0].Monitor.ID != "hidden" || len(remaining.Cards[0].History) != 1 {
		t.Fatal("deletion affected another monitor", err)
	}
	config, err = r.settings(ctx, m.UserID, m.WorkspaceID)
	if err != nil || len(config.VisibleMonitorIDs) != 0 {
		t.Fatal("deleted item remained in publish selection", err)
	}
}

func TestEmbedMonitorVisibilityMigrationPreservesExistingAndHidden(t *testing.T) {
	r := embedTestRepository(t)
	ctx := context.Background()
	m := embedTestMonitor()
	if err := r.save(ctx, m); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(ctx, `ALTER TABLE embed_group_monitors DROP COLUMN published`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../database/migrations/000037_embed_monitor_visibility.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	config, err := r.settings(ctx, m.UserID, m.WorkspaceID)
	if err != nil || !reflect.DeepEqual(config.VisibleMonitorIDs, []string{m.ID}) {
		t.Fatal("migration hid an existing monitor", err)
	}
	if _, err = r.db.Exec(ctx, `UPDATE embed_group_monitors SET published=false`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	m.ID = "created-after-upgrade"
	if err = r.save(ctx, m); err != nil {
		t.Fatal(err)
	}
	config, err = r.settings(ctx, m.UserID, m.WorkspaceID)
	if err != nil || len(config.VisibleMonitorIDs) != 0 {
		t.Fatal("rerun/new monitor published unexpectedly", err)
	}
}
