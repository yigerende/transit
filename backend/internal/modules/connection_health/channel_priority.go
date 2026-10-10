package connection_health

import (
	"context"
	"log"
	"net/http"

	"transithub/backend/internal/modules/upstream"
	"transithub/backend/internal/shared/authctx"
	"transithub/backend/internal/shared/httpjson"
)

const channelPrioritySchema = `CREATE TABLE IF NOT EXISTS connection_health_channel_priority (
 user_id text NOT NULL,
 admin_account_id text NOT NULL,
 target_id text NOT NULL,
 enabled boolean NOT NULL DEFAULT true,
 PRIMARY KEY (user_id, admin_account_id, target_id)
);`

type ChannelPriority struct {
	TargetID       string `json:"targetId"`
	Enabled        bool   `json:"enabled"`
	RestorePending bool   `json:"restorePending"`
	Priority       *int   `json:"priority"`
}

func (r *Repository) ListChannelPriorities(ctx context.Context, user, workspace string) (map[string]bool, error) {
	rows, err := r.db.Query(ctx, `SELECT target_id,enabled FROM connection_health_channel_priority WHERE user_id=$1 AND admin_account_id=$2`, user, workspace)
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

func (r *Repository) SetChannelPriority(ctx context.Context, user, workspace, target string, enabled bool) error {
	_, err := r.db.Exec(ctx, `INSERT INTO connection_health_channel_priority(user_id,admin_account_id,target_id,enabled) VALUES($1,$2,$3,$4)
 ON CONFLICT(user_id,admin_account_id,target_id) DO UPDATE SET enabled=EXCLUDED.enabled`, user, workspace, target, enabled)
	return err
}

func channelPriorityEnabled(settings map[string]bool, target string) bool {
	enabled, exists := settings[target]
	return !exists || enabled
}

// Priority writes and switch saves share a lease. Read upstream values and saved
// ownership only after acquiring it, so an older sync cannot undo a completed restore.
func (s *Service) priorityWorkspaceLease(ctx context.Context, user, workspace string) (func(), error) {
	return s.repo.AcquireTargetLease(ctx, "priority:"+user+":"+workspace)
}

// restoreChannelPriority releases only system-owned priority changes. An upstream
// manual edit is already the channel's own priority and must remain untouched.
// Failed writes retain the original snapshot so maintenance can retry restoration.
func (s *Service) restoreChannelPriority(ctx context.Context, session upstream.Session, stored PrioritySyncState, accountID string, current *int) (*int, error) {
	if current == nil {
		return nil, requestError(ErrorAccountsFetch)
	}
	if stored.PendingPriority != nil && *current == *stored.PendingPriority {
		stored.LastAppliedPriority = *stored.PendingPriority
		stored.PendingPriority = nil
	}
	if !stored.Conflict && *current == stored.LastAppliedPriority && *current != stored.OriginalPriority {
		if s.priorityActions == nil {
			return current, requestError(ErrorUnknown)
		}
		pending := stored.OriginalPriority
		stored.PendingPriority = &pending
		if err := s.repo.UpsertPrioritySyncState(ctx, stored); err != nil {
			return current, err
		}
		if err := s.priorityActions.UpdateAdminTargetPriority(session, accountID, pending); err != nil {
			return current, err
		}
		current = &pending
	}
	return current, s.repo.DeletePrioritySyncState(ctx, stored.UserID, stored.AdminAccountID, stored.TargetID)
}

func (s *Service) SetChannelPriority(ctx context.Context, user, targetID string, enabled bool) (ChannelPriority, error) {
	workspace, err := s.currentAdminAccountID(ctx, user)
	if err != nil {
		return ChannelPriority{}, err
	}
	release, err := s.priorityWorkspaceLease(ctx, user, workspace)
	if err != nil {
		return ChannelPriority{}, err
	}
	defer release()
	session, target, account, resolvedWorkspace, err := s.resolveManualTarget(ctx, user, targetID)
	if err != nil {
		return ChannelPriority{}, err
	}
	if resolvedWorkspace != workspace {
		return ChannelPriority{}, requestError(ErrorProbeTargetNotFound)
	}
	states, err := s.repo.ListPrioritySyncStates(ctx, user, workspace)
	if err != nil {
		return ChannelPriority{}, err
	}
	if err = s.repo.SetChannelPriority(ctx, user, workspace, target.TargetID, enabled); err != nil {
		return ChannelPriority{}, err
	}
	result := ChannelPriority{TargetID: target.TargetID, Enabled: enabled, Priority: cloneIntPointer(account.Priority)}
	if !enabled {
		for _, state := range states {
			if state.TargetID != target.TargetID {
				continue
			}
			result.Priority, err = s.restoreChannelPriority(ctx, session, state, target.AccountID, account.Priority)
			if err != nil {
				result.RestorePending = true
				log.Printf("[connection-health] channel priority switch restore pending target_id=%s err=%v", target.TargetID, err)
			}
			break
		}
	}
	return result, nil
}

func (h *Handler) setChannelPriority(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.service.SetChannelPriority(r.Context(), user, r.PathValue("id"), *input.Enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}
