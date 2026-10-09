package connection_health

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

const qualitySchema = `CREATE TABLE IF NOT EXISTS connection_health_quality_settings (
    user_id text NOT NULL,
    admin_account_id text NOT NULL,
    revision text NOT NULL,
    config jsonb NOT NULL,
    PRIMARY KEY (user_id, admin_account_id)
);
CREATE TABLE IF NOT EXISTS connection_health_quality_groups (
    user_id text NOT NULL,
    admin_account_id text NOT NULL,
    group_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, admin_account_id, group_id)
);
CREATE TABLE IF NOT EXISTS connection_health_quality_states (
    user_id text NOT NULL,
    admin_account_id text NOT NULL,
    target_id text NOT NULL,
    revision text NOT NULL,
    state jsonb NOT NULL,
    PRIMARY KEY (user_id, admin_account_id, target_id)
);
CREATE TABLE IF NOT EXISTS connection_health_quality_history (
    id text PRIMARY KEY,
    user_id text NOT NULL,
    admin_account_id text NOT NULL,
    target_id text NOT NULL,
    revision text NOT NULL,
    sample jsonb NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_quality_history_target ON connection_health_quality_history (user_id, admin_account_id, target_id, created_at DESC, id DESC);
`

type qualityRepository interface {
	GetQualitySettings(context.Context, string, string) (QualitySettings, error)
	SaveQualitySettings(context.Context, string, string, QualitySettings) error
	ListQualityScopes(context.Context) ([]QualityScope, error)
	ListQualityGroups(context.Context, string, string) ([]QualityGroup, error)
	SetQualityGroup(context.Context, string, string, string, bool) error
	ListQualityStates(context.Context, string, string) ([]QualityState, error)
	SaveQualityResult(context.Context, string, string, []string, QualitySettings, QualityState) (bool, error)
	ListQualityHistory(context.Context, string, string, []string, string, int) ([]QualitySample, error)
}

func (r *Repository) GetQualitySettings(ctx context.Context, user, workspace string) (QualitySettings, error) {
	var raw []byte
	err := r.db.QueryRow(ctx, `SELECT config FROM connection_health_quality_settings WHERE user_id=$1 AND admin_account_id=$2`, user, workspace).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultQualitySettings(), nil
	}
	if err != nil {
		return QualitySettings{}, err
	}
	var q QualitySettings
	err = json.Unmarshal(raw, &q)
	return q, err
}
func (r *Repository) SaveQualitySettings(ctx context.Context, user, workspace string, q QualitySettings) error {
	raw, err := json.Marshal(q)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO connection_health_quality_settings(user_id,admin_account_id,revision,config) VALUES($1,$2,$3,$4)
	 ON CONFLICT(user_id,admin_account_id) DO UPDATE SET revision=EXCLUDED.revision,config=EXCLUDED.config`, user, workspace, q.Revision, raw)
	return err
}
func (r *Repository) ListQualityScopes(ctx context.Context) ([]QualityScope, error) {
	rows, err := r.db.Query(ctx, `SELECT s.user_id,s.admin_account_id,s.config FROM connection_health_quality_settings s
	 WHERE s.config->>'enabled'='true' AND EXISTS(SELECT 1 FROM connection_health_quality_groups g WHERE g.user_id=s.user_id AND g.admin_account_id=s.admin_account_id AND g.enabled)
	 ORDER BY s.user_id,s.admin_account_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QualityScope{}
	for rows.Next() {
		var scope QualityScope
		var raw []byte
		if err = rows.Scan(&scope.UserID, &scope.WorkspaceID, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &scope.Settings); err != nil {
			return nil, err
		}
		out = append(out, scope)
	}
	return out, rows.Err()
}
func (r *Repository) ListQualityGroups(ctx context.Context, user, workspace string) ([]QualityGroup, error) {
	rows, err := r.db.Query(ctx, `SELECT group_id,enabled FROM connection_health_quality_groups WHERE user_id=$1 AND admin_account_id=$2`, user, workspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QualityGroup{}
	for rows.Next() {
		var g QualityGroup
		if err = rows.Scan(&g.GroupID, &g.Enabled); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
func (r *Repository) SetQualityGroup(ctx context.Context, user, workspace, group string, enabled bool) error {
	_, err := r.db.Exec(ctx, `INSERT INTO connection_health_quality_groups(user_id,admin_account_id,group_id,enabled) VALUES($1,$2,$3,$4)
	 ON CONFLICT(user_id,admin_account_id,group_id) DO UPDATE SET enabled=EXCLUDED.enabled`, user, workspace, group, enabled)
	return err
}
func (r *Repository) ListQualityStates(ctx context.Context, user, workspace string) ([]QualityState, error) {
	rows, err := r.db.Query(ctx, `SELECT state FROM connection_health_quality_states WHERE user_id=$1 AND admin_account_id=$2`, user, workspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QualityState{}
	for rows.Next() {
		var raw []byte
		var state QualityState
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &state); err != nil {
			return nil, err
		}
		out = append(out, state)
	}
	return out, rows.Err()
}

// Atomically guard against stale config, disabled groups and workspace deletion.
// Stores only quality evidence. Health policy state/events/actions are untouched.
func (r *Repository) SaveQualityResult(ctx context.Context, user, workspace string, groups []string, q QualitySettings, state QualityState) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var revision string
	err = tx.QueryRow(ctx, `SELECT revision FROM connection_health_quality_settings WHERE user_id=$1 AND admin_account_id=$2 AND config->>'enabled'='true' FOR SHARE`, user, workspace).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && revision != q.Revision {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var group string
	err = tx.QueryRow(ctx, `SELECT group_id FROM connection_health_quality_groups WHERE user_id=$1 AND admin_account_id=$2 AND group_id=ANY($3::text[]) AND enabled ORDER BY group_id LIMIT 1 FOR SHARE`, user, workspace, groups).Scan(&group)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_health_quality_states(user_id,admin_account_id,target_id,revision,state) VALUES($1,$2,$3,$4,$5)
	 ON CONFLICT(user_id,admin_account_id,target_id) DO UPDATE SET revision=EXCLUDED.revision,state=EXCLUDED.state`, user, workspace, state.TargetID, q.Revision, raw)
	if err != nil {
		return false, err
	}
	raw, err = json.Marshal(state.Latest)
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO connection_health_quality_history(id,user_id,admin_account_id,target_id,revision,sample,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, state.Latest.ID, user, workspace, state.TargetID, q.Revision, raw, state.Latest.CreatedAt)
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `DELETE FROM connection_health_quality_history WHERE user_id=$1 AND admin_account_id=$2 AND target_id=$3 AND id IN
	 (SELECT id FROM connection_health_quality_history WHERE user_id=$1 AND admin_account_id=$2 AND target_id=$3 ORDER BY created_at DESC,id DESC OFFSET $4)`, user, workspace, state.TargetID, q.HistoryLimit)
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
func (r *Repository) ListQualityHistory(ctx context.Context, user, workspace string, targets []string, revision string, limit int) ([]QualitySample, error) {
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `SELECT h.sample FROM unnest($3::text[]) AS target(id) CROSS JOIN LATERAL
	 (SELECT sample FROM connection_health_quality_history WHERE user_id=$1 AND admin_account_id=$2 AND target_id=target.id AND revision=$4 ORDER BY created_at DESC,id DESC LIMIT $5)h`, user, workspace, targets, revision, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QualitySample{}
	for rows.Next() {
		var raw []byte
		var s QualitySample
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
