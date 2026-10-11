package connection_health

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"transithub/backend/internal/shared/authctx"
	"transithub/backend/internal/shared/httpjson"
)

const channelQualitySuspensionSchema = `CREATE TABLE IF NOT EXISTS connection_health_channel_quality_suspension (
 user_id text NOT NULL,
 admin_account_id text NOT NULL,
 target_id text NOT NULL,
 enabled boolean NOT NULL DEFAULT false,
 PRIMARY KEY (user_id, admin_account_id, target_id)
);`

type ChannelQualitySuspension struct {
	TargetID       string `json:"targetId"`
	Enabled        bool   `json:"enabled"`
	RestorePending bool   `json:"restorePending"`
}

func (r *Repository) GetChannelQualitySuspension(ctx context.Context, user, workspace, target string) (bool, error) {
	var enabled bool
	err := r.db.QueryRow(ctx, `SELECT enabled FROM connection_health_channel_quality_suspension WHERE user_id=$1 AND admin_account_id=$2 AND target_id=$3`, user, workspace, target).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return enabled, err
}

func (r *Repository) ListChannelQualitySuspensions(ctx context.Context, user, workspace string) (map[string]bool, error) {
	rows, err := r.db.Query(ctx, `SELECT target_id,enabled FROM connection_health_channel_quality_suspension WHERE user_id=$1 AND admin_account_id=$2`, user, workspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]bool{}
	for rows.Next() {
		var target string
		var enabled bool
		if err := rows.Scan(&target, &enabled); err != nil {
			return nil, err
		}
		result[target] = enabled
	}
	return result, rows.Err()
}

func (r *Repository) SetChannelQualitySuspension(ctx context.Context, user, workspace, target string, enabled bool) error {
	_, err := r.db.Exec(ctx, `INSERT INTO connection_health_channel_quality_suspension(user_id,admin_account_id,target_id,enabled) VALUES($1,$2,$3,$4)
 ON CONFLICT(user_id,admin_account_id,target_id) DO UPDATE SET enabled=EXCLUDED.enabled`, user, workspace, target, enabled)
	return err
}

// The switch only controls quality-owned status changes, never quality testing.
func (s *Service) SetChannelQualitySuspension(ctx context.Context, user, targetID string, enabled bool) (ChannelQualitySuspension, error) {
	workspace, err := s.currentAdminAccountID(ctx, user)
	if err != nil {
		return ChannelQualitySuspension{}, err
	}
	release, err := s.channelActionLease(ctx, user, workspace, targetID)
	if err != nil {
		return ChannelQualitySuspension{}, err
	}
	defer release()
	_, target, _, resolvedWorkspace, err := s.resolveManualTarget(ctx, user, targetID)
	if err != nil {
		return ChannelQualitySuspension{}, err
	}
	if workspace != resolvedWorkspace {
		return ChannelQualitySuspension{}, requestError(ErrorProbeTargetNotFound)
	}
	if err = s.repo.SetChannelQualitySuspension(ctx, user, workspace, target.TargetID, enabled); err != nil {
		return ChannelQualitySuspension{}, err
	}
	result := ChannelQualitySuspension{TargetID: target.TargetID, Enabled: enabled}
	// Enabling takes effect on the next completed test, using the saved thresholds.
	if enabled {
		return result, nil
	}
	defer s.invalidateMonitorAccount(user, workspace, target.AccountID)
	stored, err := s.repo.GetTargetActionState(ctx, user, workspace, target.TargetID)
	if err != nil {
		result.RestorePending = true
		return result, nil
	}
	if stored == nil || !stored.QualitySuspended {
		return result, nil
	}
	// Read current group membership and policies under the action lease, without
	// waiting for a model request's probe lease. No model call is made here.
	job, err := s.manualProbeJob(ctx, user, targetID)
	if err == nil {
		var action string
		action, err = s.releaseChannelQualitySuspensionLocked(ctx, user, workspace, job.session, job.target, job.models, stored)
		if action != "" || err != nil {
			s.recordQualityAction(ctx, user, workspace, job.target, job.models, action, err)
		}
	}
	latest, readErr := s.repo.GetTargetActionState(ctx, user, workspace, target.TargetID)
	result.RestorePending = err != nil || readErr != nil || (latest != nil && latest.QualitySuspended)
	return result, nil
}

func (h *Handler) setChannelQualitySuspension(w http.ResponseWriter, r *http.Request) {
	user, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if httpjson.Decode(r, &input) != nil || input.Enabled == nil {
		httpjson.WriteError(w, http.StatusBadRequest, ErrorRequest)
		return
	}
	result, err := h.service.SetChannelQualitySuspension(r.Context(), user, r.PathValue("id"), *input.Enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}

// Preserve existing holds on upgrade; a saved opt-out always wins.
const channelQualitySuspensionBackfill = `INSERT INTO connection_health_channel_quality_suspension(user_id,admin_account_id,target_id,enabled)
 SELECT user_id,admin_account_id,target_id,true FROM connection_health_target_action_states WHERE quality_suspended
 ON CONFLICT(user_id,admin_account_id,target_id) DO NOTHING;`
