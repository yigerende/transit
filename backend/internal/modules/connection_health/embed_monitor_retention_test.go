package connection_health

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestEmbedMonitorConfigurableRetention(t *testing.T) {
	r := embedTestRepository(t)
	ctx := context.Background()
	m := embedTestMonitor()
	monitors := []EmbedMonitor{m, m, m, m}
	monitors[1].ID, monitors[1].Enabled = "disabled", false
	monitors[2].ID, monitors[2].WorkspaceID = "foreign-workspace", "ws2"
	monitors[3].ID, monitors[3].UserID = "foreign-user", "user2"
	for _, monitor := range monitors {
		if err := r.save(ctx, monitor); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"health", "candy", "pelican"} {
			_, err := r.db.Exec(ctx, `INSERT INTO embed_group_monitor_samples(id,monitor_id,kind,started_at,sample)
    SELECT $1||'-'||$2||'-'||i,$1,$2,now()-interval '1 hour'+i*interval '1 second',
    jsonb_build_object('id',$1||'-'||$2||'-'||i,'kind',$2::text,'result','passed','html','large artwork') FROM generate_series(1,70) i`, monitor.ID, kind)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	count := func(id, kind string) int {
		t.Helper()
		var n int
		if err := r.db.QueryRow(ctx, `SELECT count(*) FROM embed_group_monitor_samples WHERE monitor_id=$1 AND kind=$2`, id, kind).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	snapshot := func() string {
		t.Helper()
		var result string
		if err := r.db.QueryRow(ctx, `SELECT jsonb_build_array((SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM embed_group_monitors m),(SELECT jsonb_agg(to_jsonb(j) ORDER BY monitor_id,kind) FROM embed_group_monitor_jobs j))::text`).Scan(&result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := snapshot()
	service := &EmbedMonitorService{repo: r}
	config, err := r.settings(ctx, m.UserID, m.WorkspaceID)
	if err != nil || config.HistoryLimit == nil || *config.HistoryLimit != 60 {
		t.Fatal("default retention", config, err)
	}
	limit := 65
	config.HistoryLimit = &limit
	config, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, config, false)
	if err != nil || *config.HistoryLimit != 65 {
		t.Fatal("save retention", err)
	}
	for i, monitor := range monitors {
		for _, kind := range []string{"health", "candy", "pelican"} {
			want := 65
			if i >= 2 {
				want = 70
			}
			if got := count(monitor.ID, kind); got != want {
				t.Fatalf("scope/kind %s/%s: %d want %d", monitor.ID, kind, got, want)
			}
		}
	}
	if snapshot() != before {
		t.Fatal("retention changed monitor or scheduler configuration")
	}
	board, err := r.board(ctx, m.UserID, m.WorkspaceID, "Title", false)
	if err != nil || len(board.Cards) != 2 {
		t.Fatal(err)
	}
	for _, card := range board.Cards {
		if len(card.History) != 180 {
			t.Fatal("board must remain capped at 60 per kind", len(card.History))
		}
	}
	// Completion prunes the oldest sample with its HTML at the configured threshold.
	if err = r.queue(ctx, m.UserID, m.WorkspaceID, m.ID, "pelican"); err != nil {
		t.Fatal(err)
	}
	_, job, err := r.claim(ctx, "pelican")
	if err != nil {
		t.Fatal(err)
	}
	sample := EmbedSample{Kind: "pelican", QualitySample: QualitySample{ID: job.RunID, TargetID: m.ID, StartedAt: job.StartedAt, CreatedAt: time.Now(), Result: "passed", HTML: "new artwork"}}
	if err = r.complete(ctx, m, job, sample); err != nil {
		t.Fatal(err)
	}
	if count(m.ID, "pelican") != 65 {
		t.Fatal("completion exceeded retention")
	}
	if _, err = r.detail(ctx, m.UserID, m.WorkspaceID, m.ID+"-pelican-6"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("oldest artwork retained", err)
	}
	if _, err = r.detail(ctx, m.UserID, m.WorkspaceID, job.RunID); err != nil {
		t.Fatal("newest sample deleted", err)
	}
	for _, bad := range []int{0, -1, 1001} {
		invalid := config
		invalid.HistoryLimit = &bad
		invalid.Title = "must not save"
		if _, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, invalid, false); err == nil {
			t.Fatal("invalid limit accepted", bad)
		}
	}
	limit = 2
	config.HistoryLimit = &limit
	config, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, config, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{m.ID, "disabled"} {
		for _, kind := range []string{"health", "candy", "pelican"} {
			if count(id, kind) != 2 {
				t.Fatal("saving smaller limit did not immediately prune", id, kind)
			}
		}
	}
	if _, err = r.detail(ctx, m.UserID, m.WorkspaceID, m.ID+"-candy-68"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("old detail survived", err)
	}
	for _, i := range []int{69, 70} {
		if _, err = r.detail(ctx, m.UserID, m.WorkspaceID, fmt.Sprintf("%s-candy-%d", m.ID, i)); err != nil {
			t.Fatal("wrong sample order", err)
		}
	}
	legacy := config
	legacy.HistoryLimit = nil
	config, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, legacy, false)
	if err != nil || *config.HistoryLimit != 2 {
		t.Fatal("old client reset retention", err)
	}
	config, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, EmbedBoardSettings{}, true)
	if err != nil || *config.HistoryLimit != 2 {
		t.Fatal("rotation reset retention", err)
	}
	limit = 1000
	config.HistoryLimit = &limit
	if _, err = service.saveSettings(ctx, m.UserID, m.WorkspaceID, config, false); err != nil {
		t.Fatal(err)
	}
	if count(m.ID, "pelican") != 2 {
		t.Fatal("raising retention restored removed history")
	}
	if count("foreign-workspace", "pelican") != 70 || count("foreign-user", "pelican") != 70 {
		t.Fatal("foreign history altered")
	}
}
