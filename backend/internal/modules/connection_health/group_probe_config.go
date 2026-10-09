package connection_health

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const groupProbeConfigSchema = `CREATE TABLE IF NOT EXISTS connection_health_group_probe_configs (
	user_id text NOT NULL,
	admin_account_id text NOT NULL,
	group_id text NOT NULL,
	model text NOT NULL,
	custom_key_id text NOT NULL DEFAULT '',
	interval_seconds integer NOT NULL DEFAULT 60 CHECK (interval_seconds BETWEEN 10 AND 86400),
	enabled boolean NOT NULL DEFAULT true,
	next_probe_at timestamptz,
	last_probe_at timestamptz,
	last_error_key text NOT NULL DEFAULT '',
	updated_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (user_id, admin_account_id, group_id)
)`

type GroupProbeConfig struct {
	UserID          string     `json:"-"`
	AdminAccountID  string     `json:"-"`
	GroupID         string     `json:"groupId"`
	Model           string     `json:"model"`
	CustomKeyID     string     `json:"-"`
	HasCustomKey    bool       `json:"hasCustomKey"`
	IntervalSeconds int        `json:"intervalSeconds"`
	Enabled         bool       `json:"enabled"`
	NextProbeAt     *time.Time `json:"nextProbeAt"`
	LastProbeAt     *time.Time `json:"lastProbeAt"`
	LastErrorKey    string     `json:"lastErrorKey"`
}

type GroupProbeConfigInput struct {
	// Omitted keeps the saved key; empty explicitly switches to automatic creation.
	Key             *string `json:"key"`
	Model           string  `json:"model"`
	IntervalSeconds int     `json:"intervalSeconds"`
	Enabled         bool    `json:"enabled"`
}

const groupProbeConfigColumns = `user_id, admin_account_id, group_id, model, interval_seconds, enabled, next_probe_at, last_probe_at, last_error_key, custom_key_id`

func scanGroupProbeConfig(row pgx.Row) (GroupProbeConfig, error) {
	var c GroupProbeConfig
	err := row.Scan(&c.UserID, &c.AdminAccountID, &c.GroupID, &c.Model, &c.IntervalSeconds, &c.Enabled, &c.NextProbeAt, &c.LastProbeAt, &c.LastErrorKey, &c.CustomKeyID)
	c.HasCustomKey = c.CustomKeyID != ""
	return c, err
}

func (r *Repository) GetGroupProbeConfig(ctx context.Context, userID, workspaceID, groupID string) (*GroupProbeConfig, error) {
	c, err := scanGroupProbeConfig(r.db.QueryRow(ctx, `SELECT `+groupProbeConfigColumns+` FROM connection_health_group_probe_configs WHERE user_id=$1 AND admin_account_id=$2 AND group_id=$3`, userID, workspaceID, groupID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) ListGroupProbeConfigs(ctx context.Context, userID, workspaceID string) ([]GroupProbeConfig, error) {
	rows, err := r.db.Query(ctx, `SELECT `+groupProbeConfigColumns+` FROM connection_health_group_probe_configs WHERE user_id=$1 AND admin_account_id=$2`, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGroupProbeConfigs(rows)
}

func (r *Repository) ListDueGroupProbeConfigs(ctx context.Context, now time.Time) ([]GroupProbeConfig, error) {
	rows, err := r.db.Query(ctx, `SELECT `+groupProbeConfigColumns+` FROM connection_health_group_probe_configs WHERE enabled AND next_probe_at <= $1 ORDER BY next_probe_at, user_id, admin_account_id, group_id LIMIT 100`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGroupProbeConfigs(rows)
}

func collectGroupProbeConfigs(rows pgx.Rows) ([]GroupProbeConfig, error) {
	configs := []GroupProbeConfig{}
	for rows.Next() {
		c, err := scanGroupProbeConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (r *Repository) SaveGroupProbeConfig(ctx context.Context, c GroupProbeConfig) error {
	_, err := r.db.Exec(ctx, `INSERT INTO connection_health_group_probe_configs (`+groupProbeConfigColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (user_id, admin_account_id, group_id) DO UPDATE SET
		model=EXCLUDED.model, interval_seconds=EXCLUDED.interval_seconds, enabled=EXCLUDED.enabled,
		next_probe_at=EXCLUDED.next_probe_at, last_probe_at=EXCLUDED.last_probe_at,
		last_error_key=EXCLUDED.last_error_key, custom_key_id=EXCLUDED.custom_key_id, updated_at=now()`,
		c.UserID, c.AdminAccountID, c.GroupID, c.Model, c.IntervalSeconds, c.Enabled, c.NextProbeAt, c.LastProbeAt, c.LastErrorKey, c.CustomKeyID)
	return err
}

func (r *Repository) CompleteGroupProbe(ctx context.Context, c GroupProbeConfig, completed time.Time, errorKey string) error {
	// UPDATE only: deleting a workspace while a request is in flight must not
	// recreate its task. The caller holds the same target lease as settings saves.
	_, err := r.db.Exec(ctx, `UPDATE connection_health_group_probe_configs SET
		last_probe_at=$4::timestamptz, next_probe_at=$4::timestamptz + make_interval(secs => interval_seconds), last_error_key=$5
		WHERE user_id=$1 AND admin_account_id=$2 AND group_id=$3 AND enabled`, c.UserID, c.AdminAccountID, c.GroupID, completed, errorKey)
	return err
}

func (s *Service) GroupProbeConfiguration(ctx context.Context, userID, groupID string) (*GroupProbeConfig, error) {
	workspaceID, err := s.currentAdminAccountID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetGroupProbeConfig(ctx, userID, workspaceID, groupID)
}

func (s *Service) SaveGroupProbeConfiguration(ctx context.Context, userID, groupID string, input GroupProbeConfigInput) (GroupProbeConfig, error) {
	input.Model = strings.TrimSpace(input.Model)
	if input.Model == "" || len(input.Model) > 200 {
		return GroupProbeConfig{}, requestError(groupProbePrefix + "modelRequired")
	}
	if input.IntervalSeconds == 0 {
		input.IntervalSeconds = 60
	}
	if input.IntervalSeconds < 10 || input.IntervalSeconds > 86400 {
		return GroupProbeConfig{}, requestError(groupProbePrefix + "intervalInvalid")
	}
	workspaceID, err := s.currentAdminAccountID(ctx, userID)
	if err != nil {
		return GroupProbeConfig{}, err
	}
	release, err := s.repo.AcquireTargetLease(ctx, groupProbeTargetID(workspaceID, groupID))
	if err != nil {
		return GroupProbeConfig{}, err
	}
	defer release()
	previous, err := s.repo.GetGroupProbeConfig(ctx, userID, workspaceID, groupID)
	if err != nil {
		return GroupProbeConfig{}, err
	}
	customKeyID := ""
	if previous != nil {
		customKeyID = previous.CustomKeyID
	}
	if !input.Enabled && previous == nil {
		return GroupProbeConfig{}, requestError(ErrorNotFound)
	}
	if input.Enabled || (input.Key != nil && strings.TrimSpace(*input.Key) != "") {
		// Validate the current workspace/group and prepare the dedicated key before
		// accepting an automatic task. Pausing works even if upstream is offline.
		_, _, customKeyID, err = s.resolveGroupProbeCredential(ctx, userID, workspaceID, groupID, input.Key)
		if err != nil {
			return GroupProbeConfig{}, err
		}
	} else if input.Key != nil {
		customKeyID = ""
	}
	c := GroupProbeConfig{UserID: userID, AdminAccountID: workspaceID, GroupID: groupID, Model: input.Model, IntervalSeconds: input.IntervalSeconds, Enabled: input.Enabled, CustomKeyID: customKeyID, HasCustomKey: customKeyID != ""}
	if previous != nil {
		c.LastProbeAt = previous.LastProbeAt
		c.LastErrorKey = previous.LastErrorKey
	}
	if c.Enabled {
		now := time.Now()
		c.NextProbeAt = &now
		c.LastErrorKey = ""
	}
	return c, s.repo.SaveGroupProbeConfig(ctx, c)
}
