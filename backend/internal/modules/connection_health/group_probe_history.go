package connection_health

import (
	"context"
	"log"
	"sort"
	"time"
)

const groupProbeHistoryLimit = 100

// GroupProbeSample contains only the fields needed for a group's probe timeline.
type GroupProbeSample struct {
	ID        string    `json:"id"`
	TargetID  string    `json:"targetId"`
	ModelName string    `json:"modelName"`
	Result    string    `json:"result"`
	LatencyMs *int      `json:"latencyMs"`
	CreatedAt time.Time `json:"createdAt"`
}

// Read history independently for gateway groups and individual channels.
// Shared channels keep their own history in every group they belong to.
func (s *Service) attachGroupProbeHistory(ctx context.Context, userID, adminAccountID string, groups []AdminGroupHealth) {
	targetIDs := make([]string, 0)
	seen := make(map[string]bool)
	for i := range groups {
		groups[i].RecentProbes = []GroupProbeSample{}
		targetIDs = append(targetIDs, groupProbeTargetID(adminAccountID, groups[i].ID))
		for _, account := range groups[i].Accounts {
			if account.TargetID != "" && !seen[account.TargetID] {
				seen[account.TargetID] = true
				targetIDs = append(targetIDs, account.TargetID)
			}
		}
	}
	if len(targetIDs) == 0 {
		return
	}
	samples, err := s.repo.ListRecentProbesByTargets(ctx, userID, adminAccountID, targetIDs)
	if err != nil {
		log.Printf("[connection-health] group probe history failed: %v", err)
		for i := range groups {
			groups[i].ProbeHistoryError = "admin.connectionHealth.cards.historyUnavailable"
		}
		return
	}
	byTarget := make(map[string][]GroupProbeSample)
	for _, sample := range samples {
		byTarget[sample.TargetID] = append(byTarget[sample.TargetID], sample)
	}
	for i := range groups {
		group := &groups[i]
		for j := range group.Accounts {
			account := &group.Accounts[j]
			account.RecentProbes = append([]GroupProbeSample{}, byTarget[account.TargetID]...)
			sort.Slice(account.RecentProbes, func(a, b int) bool {
				return account.RecentProbes[a].CreatedAt.After(account.RecentProbes[b].CreatedAt)
			})
			if len(account.RecentProbes) > groupProbeHistoryLimit {
				account.RecentProbes = account.RecentProbes[:groupProbeHistoryLimit]
			}
		}
		group.RecentProbes = append([]GroupProbeSample{}, byTarget[groupProbeTargetID(adminAccountID, group.ID)]...)
		sort.Slice(group.RecentProbes, func(a, b int) bool {
			return group.RecentProbes[a].CreatedAt.After(group.RecentProbes[b].CreatedAt)
		})
		if len(group.RecentProbes) > 20 {
			group.RecentProbes = group.RecentProbes[:20]
		}
	}
}

func (r *Repository) ListRecentProbesByTargets(ctx context.Context, userID, adminAccountID string, targetIDs []string) ([]GroupProbeSample, error) {
	if len(targetIDs) == 0 {
		return []GroupProbeSample{}, nil
	}
	// LATERAL uses the existing (connection_id, created_at) index for each target
	// instead of sorting the workspace's full event history on every refresh.
	rows, err := r.db.Query(ctx, `
		SELECT event.id, target.target_id, event.model_name, event.result, event.latency_ms, event.created_at
		FROM unnest($3::text[]) AS target(target_id)
		CROSS JOIN LATERAL (
			SELECT id, model_name, result, latency_ms, created_at
			FROM connection_health_events
			WHERE user_id = $1 AND admin_account_id = $2
				AND connection_id = target.target_id AND result = ANY($4::text[])
			ORDER BY created_at DESC, id DESC LIMIT $5
		) AS event
	`, userID, adminAccountID, targetIDs, probeResultKeys(), groupProbeHistoryLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	samples := make([]GroupProbeSample, 0)
	for rows.Next() {
		var sample GroupProbeSample
		if err := rows.Scan(&sample.ID, &sample.TargetID, &sample.ModelName, &sample.Result, &sample.LatencyMs, &sample.CreatedAt); err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}
	return samples, rows.Err()
}
