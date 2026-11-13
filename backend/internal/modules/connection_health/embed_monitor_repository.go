package connection_health

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type embedMonitorRepository struct{ db *pgxpool.Pool }

func embedToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}

func (r *embedMonitorRepository) settings(ctx context.Context, user, workspace string) (EmbedBoardSettings, error) {
	var out EmbedBoardSettings
	token, err := embedToken()
	if err != nil {
		return out, err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO embed_group_monitor_boards(user_id,admin_account_id,token) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, user, workspace, token)
	if err != nil {
		return out, err
	}
	err = r.db.QueryRow(ctx, `SELECT title,enabled,token,ARRAY(SELECT id FROM embed_group_monitors WHERE user_id=$1 AND admin_account_id=$2 AND published ORDER BY created_at,id),history_limit FROM embed_group_monitor_boards WHERE user_id=$1 AND admin_account_id=$2`, user, workspace).Scan(&out.Title, &out.Enabled, &out.Token, &out.VisibleMonitorIDs, &out.HistoryLimit)
	return out, err
}

func (r *embedMonitorRepository) publicScope(ctx context.Context, token string) (user, workspace, title string, err error) {
	if len(token) != 64 {
		return "", "", "", pgx.ErrNoRows
	}
	err = r.db.QueryRow(ctx, `SELECT user_id,admin_account_id,title FROM embed_group_monitor_boards WHERE token=$1 AND enabled`, token).Scan(&user, &workspace, &title)
	return
}

func (r *embedMonitorRepository) save(ctx context.Context, m EmbedMonitor) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize create/update per workspace, including the maximum target count.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "embed-monitor:"+m.UserID+":"+m.WorkspaceID); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM embed_group_monitors WHERE user_id=$1 AND admin_account_id=$2 AND id<>$3`, m.UserID, m.WorkspaceID, m.ID).Scan(&count); err != nil {
		return err
	}
	if count >= 50 {
		return requestError("每个工作区最多配置 50 个分组模型")
	}
	// Do not mix different groups/models in the same history after an edit.
	var old []byte
	err = tx.QueryRow(ctx, `SELECT config FROM embed_group_monitors WHERE id=$1 AND user_id=$2 AND admin_account_id=$3 FOR UPDATE`, m.ID, m.UserID, m.WorkspaceID).Scan(&old)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if len(old) > 0 {
		var previous EmbedMonitor
		if err = json.Unmarshal(old, &previous); err != nil {
			return err
		}
		if previous.Model != m.Model || previous.GroupID != m.GroupID {
			return requestError("分组和模型不可修改，请新建检测项以保留正确历史")
		}
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO embed_group_monitors(id,user_id,admin_account_id,config) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO UPDATE SET config=EXCLUDED.config WHERE embed_group_monitors.user_id=EXCLUDED.user_id AND embed_group_monitors.admin_account_id=EXCLUDED.admin_account_id`, m.ID, m.UserID, m.WorkspaceID, data)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	for _, kind := range []string{"health", "candy", "pelican"} {
		c := m.check(kind)
		_, err = tx.Exec(ctx, `INSERT INTO embed_group_monitor_jobs(monitor_id,kind,enabled,interval_seconds) VALUES($1,$2,$3,$4)
		ON CONFLICT(monitor_id,kind) DO UPDATE SET enabled=EXCLUDED.enabled,interval_seconds=EXCLUDED.interval_seconds,
		next_run_at=CASE WHEN EXCLUDED.enabled AND NOT embed_group_monitor_jobs.enabled THEN now() ELSE LEAST(embed_group_monitor_jobs.next_run_at,now()+EXCLUDED.interval_seconds*interval '1 second') END`, m.ID, kind, m.Enabled && c.Enabled, c.IntervalSeconds)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *embedMonitorRepository) monitor(ctx context.Context, user, workspace, id string) (EmbedMonitor, error) {
	var raw []byte
	m := EmbedMonitor{UserID: user, WorkspaceID: workspace}
	err := r.db.QueryRow(ctx, `SELECT config FROM embed_group_monitors WHERE user_id=$1 AND admin_account_id=$2 AND id=$3`, user, workspace, id).Scan(&raw)
	if err == nil {
		err = json.Unmarshal(raw, &m)
	}
	return m, err
}

func (r *embedMonitorRepository) board(ctx context.Context, user, workspace, title string, public bool) (EmbedMonitorBoard, error) {
	out := EmbedMonitorBoard{Title: title, Cards: []EmbedMonitorCard{}, UpdatedAt: time.Now()}
	rows, err := r.db.Query(ctx, `SELECT config FROM embed_group_monitors WHERE user_id=$1 AND admin_account_id=$2 AND (NOT $3 OR published) ORDER BY created_at,id`, user, workspace, public)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var raw []byte
		var m EmbedMonitor
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return out, err
		}
		if err = json.Unmarshal(raw, &m); err != nil {
			rows.Close()
			return out, err
		}
		if public {
			m.QualityBaseURL = ""
		}
		out.Cards = append(out.Cards, EmbedMonitorCard{Monitor: m, Jobs: []EmbedJob{}, History: []EmbedSample{}})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for i := range out.Cards {
		card := &out.Cards[i]
		jobs, err := r.db.Query(ctx, `SELECT monitor_id,kind,requested,started_at,running_until,next_run_at FROM embed_group_monitor_jobs WHERE monitor_id=$1 ORDER BY kind`, card.Monitor.ID)
		if err != nil {
			return out, err
		}
		for jobs.Next() {
			var j EmbedJob
			if err = jobs.Scan(&j.MonitorID, &j.Kind, &j.Requested, &j.StartedAt, &j.RunningUntil, &j.NextRunAt); err != nil {
				jobs.Close()
				return out, err
			}
			card.Jobs = append(card.Jobs, j)
		}
		err = jobs.Err()
		jobs.Close()
		if err != nil {
			return out, err
		}
		// Strip large/generated and diagnostic text in SQL, not after loading it.
		history, err := r.db.Query(ctx, `SELECT sample - 'html' - 'answer' - 'report' - 'errorDetail' FROM (SELECT sample,started_at,id,row_number() OVER (PARTITION BY kind ORDER BY started_at DESC,id DESC) AS position FROM embed_group_monitor_samples WHERE monitor_id=$1) recent WHERE position<=60 ORDER BY started_at DESC,id DESC`, card.Monitor.ID)
		if err != nil {
			return out, err
		}
		for history.Next() {
			var raw []byte
			var sample EmbedSample
			if err = history.Scan(&raw); err != nil {
				history.Close()
				return out, err
			}
			if err = json.Unmarshal(raw, &sample); err != nil {
				history.Close()
				return out, err
			}
			card.History = append(card.History, sample)
		}
		err = history.Err()
		history.Close()
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func (r *embedMonitorRepository) detail(ctx context.Context, user, workspace, id string) (EmbedSample, error) {
	return r.readDetail(ctx, user, workspace, id, false)
}

func (r *embedMonitorRepository) publicDetail(ctx context.Context, user, workspace, id string) (EmbedSample, error) {
	return r.readDetail(ctx, user, workspace, id, true)
}

func (r *embedMonitorRepository) readDetail(ctx context.Context, user, workspace, id string, public bool) (EmbedSample, error) {
	var raw []byte
	var sample EmbedSample
	err := r.db.QueryRow(ctx, `SELECT s.sample FROM embed_group_monitor_samples s JOIN embed_group_monitors m ON m.id=s.monitor_id WHERE m.user_id=$1 AND m.admin_account_id=$2 AND s.id=$3 AND (NOT $4 OR m.published)`, user, workspace, id, public).Scan(&raw)
	if err == nil {
		err = json.Unmarshal(raw, &sample)
	}
	return sample, err
}

func (r *embedMonitorRepository) clearHistory(ctx context.Context, user, workspace string) (int64, error) {
	// Keep settings and scheduled/running jobs; subsequent completions are new history.
	tag, err := r.db.Exec(ctx, `DELETE FROM embed_group_monitor_samples s USING embed_group_monitors m WHERE s.monitor_id=m.id AND m.user_id=$1 AND m.admin_account_id=$2`, user, workspace)
	return tag.RowsAffected(), err
}

func (r *embedMonitorRepository) queue(ctx context.Context, user, workspace, id, kind string) error {
	if kind != "health" && kind != "candy" && kind != "pelican" {
		return requestError("未知检测方式")
	}
	tag, err := r.db.Exec(ctx, `UPDATE embed_group_monitor_jobs j SET requested=true WHERE monitor_id=$1 AND kind=$2 AND (running_until IS NULL OR running_until<now()) AND EXISTS(SELECT 1 FROM embed_group_monitors m WHERE m.id=j.monitor_id AND m.user_id=$3 AND m.admin_account_id=$4)`, id, kind, user, workspace)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return requestError("检测正在执行或检测项不存在")
	}
	return nil
}

// Atomic durable claim works across processes. The 12-minute lease exceeds the
// worker's hard 650-second deadline; crashed workers can safely be reclaimed.
func (r *embedMonitorRepository) claim(ctx context.Context, kinds ...string) (EmbedMonitor, EmbedJob, error) {
	var m EmbedMonitor
	var j EmbedJob
	var raw []byte
	kind := ""
	if len(kinds) > 0 {
		kind = kinds[0]
	}
	id, err := newID()
	if err != nil {
		return m, j, err
	}
	err = r.db.QueryRow(ctx, `WITH candidate AS (
	 SELECT monitor_id,kind,CASE WHEN run_id<>'' THEN manual ELSE requested END AS was_manual FROM embed_group_monitor_jobs WHERE ($2='' OR kind=$2) AND (requested OR run_id<>'' OR (enabled AND next_run_at<=now())) AND (running_until IS NULL OR running_until<now())
	 ORDER BY requested DESC,next_run_at,monitor_id,kind FOR UPDATE SKIP LOCKED LIMIT 1
	), claimed AS (
	 UPDATE embed_group_monitor_jobs j SET running_until=now()+interval '12 minutes',started_at=now(),run_id=$1,requested=false,manual=c.was_manual FROM candidate c WHERE j.monitor_id=c.monitor_id AND j.kind=c.kind
	 RETURNING j.monitor_id,j.kind,j.run_id,j.started_at,j.manual)
	 SELECT m.config,m.user_id,m.admin_account_id,c.monitor_id,c.kind,c.run_id,c.started_at,c.manual FROM claimed c JOIN embed_group_monitors m ON m.id=c.monitor_id`, id, kind).Scan(&raw, &m.UserID, &m.WorkspaceID, &j.MonitorID, &j.Kind, &j.RunID, &j.StartedAt, &j.Manual)
	if err == nil {
		err = json.Unmarshal(raw, &m)
	}
	return m, j, err
}

func (r *embedMonitorRepository) complete(ctx context.Context, m EmbedMonitor, j EmbedJob, sample EmbedSample) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize retention setting changes with completions, while allowing different
	// detection jobs in the same workspace to complete together.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(hashtextextended($1,0))`, "embed-monitor:"+m.UserID+":"+m.WorkspaceID); err != nil {
		return err
	}
	// A stale completion cannot overwrite a new worker or resurrect a deleted target.
	tag, err := tx.Exec(ctx, `UPDATE embed_group_monitor_jobs SET running_until=NULL,run_id='',next_run_at=GREATEST(now(),started_at+interval_seconds*interval '1 second') WHERE monitor_id=$1 AND kind=$2 AND run_id=$3`, m.ID, j.Kind, j.RunID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	raw, err := json.Marshal(sample)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO embed_group_monitor_samples(id,monitor_id,kind,started_at,sample) VALUES($1,$2,$3,$4,$5)`, sample.ID, m.ID, j.Kind, sample.StartedAt, raw)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM embed_group_monitor_samples WHERE monitor_id=$1 AND kind=$2 AND id IN (SELECT id FROM embed_group_monitor_samples WHERE monitor_id=$1 AND kind=$2 ORDER BY started_at DESC,id DESC OFFSET (SELECT COALESCE((SELECT history_limit FROM embed_group_monitor_boards WHERE user_id=$3 AND admin_account_id=$4),60)))`, m.ID, j.Kind, m.UserID, m.WorkspaceID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
