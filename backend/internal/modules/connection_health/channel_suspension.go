package connection_health

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"transithub/backend/internal/shared/authctx"
	"transithub/backend/internal/shared/httpjson"
)

const channelSuspensionSchema = `CREATE TABLE IF NOT EXISTS connection_health_channel_suspension (
 user_id text NOT NULL,
 admin_account_id text NOT NULL,
 target_id text NOT NULL,
 enabled boolean NOT NULL DEFAULT true,
 PRIMARY KEY (user_id, admin_account_id, target_id)
);`

type ChannelSuspension struct {
	TargetID string `json:"targetId"`
	Enabled  bool   `json:"enabled"`
}

func (r *Repository) GetChannelSuspension(ctx context.Context, user, workspace, target string) (bool, error) {
	var enabled bool
	err := r.db.QueryRow(ctx, `SELECT enabled FROM connection_health_channel_suspension WHERE user_id=$1 AND admin_account_id=$2 AND target_id=$3`, user, workspace, target).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	return enabled, err
}

func (r *Repository) ListChannelSuspensions(ctx context.Context, user, workspace string) (map[string]bool, error) {
	rows, err := r.db.Query(ctx, `SELECT target_id,enabled FROM connection_health_channel_suspension WHERE user_id=$1 AND admin_account_id=$2`, user, workspace)
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

func (r *Repository) SetChannelSuspension(ctx context.Context, user, workspace, target string, enabled bool) error {
	_, err := r.db.Exec(ctx, `INSERT INTO connection_health_channel_suspension(user_id,admin_account_id,target_id,enabled) VALUES($1,$2,$3,$4)
 ON CONFLICT(user_id,admin_account_id,target_id) DO UPDATE SET enabled=EXCLUDED.enabled`, user, workspace, target, enabled)
	return err
}

// A channel opt-out is shared by all groups and only removes permission.
func channelSuspensionPolicies(policies []Policy, enabled bool) []Policy {
	result := append([]Policy(nil), policies...)
	if !enabled {
		for i := range result {
			result[i].AutoSuspendEnabled = false
		}
	}
	return result
}

func channelSuspensionEnabled(settings map[string]bool, target string) bool {
	enabled, exists := settings[target]
	return !exists || enabled
}

func (s *Service) currentTargetActionPermissions(ctx context.Context, user, workspace, target string, policy Policy) Policy {
	policy = s.currentActionPermissions(ctx, policy)
	enabled, err := s.repo.GetChannelSuspension(ctx, user, workspace, target)
	if err != nil || !enabled {
		policy.AutoSuspendEnabled = false
	}
	return policy
}

// Separate from the probe lease: saving a switch need not wait for a model response.
// Every automatic status write uses this lease and re-reads the preference under it.
func (s *Service) channelActionLease(ctx context.Context, user, workspace, target string) (func(), error) {
	return s.repo.AcquireTargetLease(ctx, "suspension:"+user+":"+workspace+":"+target)
}

func (s *Service) SetChannelSuspension(ctx context.Context, user, targetID string, enabled bool) (ChannelSuspension, error) {
	_, target, _, workspace, err := s.resolveManualTarget(ctx, user, targetID)
	if err != nil {
		return ChannelSuspension{}, err
	}
	release, err := s.channelActionLease(ctx, user, workspace, target.TargetID)
	if err != nil {
		return ChannelSuspension{}, err
	}
	defer release()
	if err = s.repo.SetChannelSuspension(ctx, user, workspace, target.TargetID, enabled); err != nil {
		return ChannelSuspension{}, err
	}
	return ChannelSuspension{TargetID: target.TargetID, Enabled: enabled}, nil
}

func (h *Handler) setChannelSuspension(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.service.SetChannelSuspension(r.Context(), user, r.PathValue("id"), *input.Enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}
