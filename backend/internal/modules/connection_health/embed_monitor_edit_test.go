package connection_health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"transithub/backend/internal/modules/upstream"
)

func TestEmbedMonitorEditTarget(t *testing.T) {
	for _, field := range []string{"group", "model"} {
		t.Run(field, func(t *testing.T) {
			r := embedTestRepository(t)
			ctx := context.Background()
			m := embedTestMonitor()
			m.TargetRevision = "untrusted-client-revision"
			if err := r.save(ctx, &m); err != nil || m.TargetRevision != "" {
				t.Fatal("initial revision must remain compatible with legacy history", err)
			}
			if _, err := r.db.Exec(ctx, `UPDATE embed_group_monitors SET published=true WHERE id=$1`, m.ID); err != nil {
				t.Fatal(err)
			}
			makeSample := func(j EmbedJob) EmbedSample {
				return EmbedSample{Kind: j.Kind, QualitySample: QualitySample{ID: j.RunID, TargetID: m.ID, Model: m.Model, StartedAt: j.StartedAt, CreatedAt: time.Now(), Result: "passed", HTML: "old artwork"}}
			}
			oldIDs := []string{}
			for _, kind := range []string{"health", "candy", "pelican"} {
				if err := r.queue(ctx, m.UserID, m.WorkspaceID, m.ID, kind); err != nil {
					t.Fatal(err)
				}
				claimed, job, err := r.claim(ctx, kind)
				if err != nil {
					t.Fatal(err)
				}
				if err = r.complete(ctx, claimed, job, makeSample(job)); err != nil {
					t.Fatal(err)
				}
				oldIDs = append(oldIDs, job.RunID)
			}
			assertBoard := func(n int) {
				t.Helper()
				for _, public := range []bool{false, true} {
					b, err := r.board(ctx, m.UserID, m.WorkspaceID, "test", public)
					if err != nil || len(b.Cards) != 1 || len(b.Cards[0].History) != n {
						t.Fatalf("board history: %+v, %v", b, err)
					}
				}
			}
			assertBoard(3)
			m.Name = "renamed"
			m.Health.IntervalSeconds = 90
			m.TargetRevision = "spoof"
			if err := r.save(ctx, &m); err != nil || m.TargetRevision != "" {
				t.Fatal("ordinary save changed revision", err)
			}
			assertBoard(3)
			if err := r.queue(ctx, m.UserID, m.WorkspaceID, m.ID, "pelican"); err != nil {
				t.Fatal(err)
			}
			old, running, err := r.claim(ctx, "pelican")
			if err != nil {
				t.Fatal(err)
			}
			if err = r.queue(ctx, m.UserID, m.WorkspaceID, m.ID, "candy"); err != nil {
				t.Fatal(err)
			}
			if field == "group" {
				m.GroupID = "88"
			} else {
				m.Model = "new-model"
			}
			m.Health.Enabled = true
			if err = r.save(ctx, &m); err != nil || m.TargetRevision == "" {
				t.Fatal("target edit failed", err)
			}
			revision := m.TargetRevision
			assertBoard(0)
			var retained, active int
			if err = r.db.QueryRow(ctx, `SELECT count(*) FROM embed_group_monitor_samples WHERE monitor_id=$1`, m.ID).Scan(&retained); err != nil || retained != 3 {
				t.Fatal("old history deleted", err)
			}
			if err = r.db.QueryRow(ctx, `SELECT count(*) FROM embed_group_monitor_jobs WHERE monitor_id=$1 AND (requested OR run_id<>'' OR running_until IS NOT NULL OR started_at IS NOT NULL)`, m.ID).Scan(&active); err != nil || active != 0 {
				t.Fatal("old tasks not reset", err)
			}
			for _, id := range oldIDs {
				if _, err = r.detail(ctx, m.UserID, m.WorkspaceID, id); !errors.Is(err, pgx.ErrNoRows) {
					t.Fatal("old detail exposed", err)
				}
				if _, err = r.publicDetail(ctx, m.UserID, m.WorkspaceID, id); !errors.Is(err, pgx.ErrNoRows) {
					t.Fatal("old public artwork exposed", err)
				}
			}
			if err = r.complete(ctx, old, running, makeSample(running)); err != nil {
				t.Fatal(err)
			}
			assertBoard(0)
			current, job, err := r.claim(ctx, "health")
			if err != nil || current.GroupID != m.GroupID || current.Model != m.Model || current.TargetRevision != revision || job.Manual {
				t.Fatal("new automatic task did not use edited target", err)
			}
			if err = r.complete(ctx, current, job, makeSample(job)); err != nil {
				t.Fatal(err)
			}
			assertBoard(1)
			if sample, err := r.publicDetail(ctx, m.UserID, m.WorkspaceID, job.RunID); err != nil || sample.TargetRevision != revision {
				t.Fatal("new history missing", err)
			}
			// Simulate a claim with an old config snapshot but a current lease token.
			if err = r.queue(ctx, m.UserID, m.WorkspaceID, m.ID, "candy"); err != nil {
				t.Fatal(err)
			}
			_, race, err := r.claim(ctx, "candy")
			if err != nil {
				t.Fatal(err)
			}
			if err = r.complete(ctx, old, race, makeSample(race)); err != nil {
				t.Fatal(err)
			}
			assertBoard(1)
			var lease string
			if err = r.db.QueryRow(ctx, `SELECT run_id FROM embed_group_monitor_jobs WHERE monitor_id=$1 AND kind='candy'`, m.ID).Scan(&lease); err != nil || lease != "" {
				t.Fatal("stale snapshot left a running lease", err)
			}
			m.TargetRevision = "spoof-again"
			if err = r.save(ctx, &m); err != nil || m.TargetRevision != revision {
				t.Fatal("client could replace current revision", err)
			}
			assertBoard(1)
			foreign := m
			foreign.WorkspaceID = "foreign"
			foreign.GroupID = "stolen"
			if err = r.save(ctx, &foreign); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatal("cross-workspace edit allowed", err)
			}
			m.GroupID, m.Model = old.GroupID, old.Model
			if err = r.save(ctx, &m); err != nil || m.TargetRevision == revision || m.TargetRevision == "" {
				t.Fatal("returning to a target reused old statistics", err)
			}
			assertBoard(0)
		})
	}
}

func TestEmbedMonitorEditedGroupCredential(t *testing.T) {
	r := embedTestRepository(t)
	ctx := context.Background()
	m := embedTestMonitor()
	if err := r.save(ctx, &m); err != nil {
		t.Fatal(err)
	}
	provider := &fakeGroupProbeProvider{fakePlatformGroupReader: fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "42", Name: "Original", Platform: ProviderOpenAI}, {ID: "88", Name: "New group", Platform: ProviderOpenAI}}}}
	health := newAdminGroupsService(provider, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API, AccessToken: "jwt", BaseURL: "http://workspace.invalid"}}, newFakeRepository())
	s := &EmbedMonitorService{repo: r, health: health}
	m.GroupID, m.Model = "88", "new-model"
	saved, err := s.save(ctx, m.UserID, m.WorkspaceID, m)
	if err != nil || saved.GroupName != "New group" || saved.TargetRevision == "" {
		t.Fatal("save failed", err)
	}
	if _, _, err = s.credential(ctx, saved); err != nil || provider.groupID != "88" {
		t.Fatal("edited target used old group key", err)
	}
}
