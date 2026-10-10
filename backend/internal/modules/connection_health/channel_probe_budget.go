package connection_health

import (
	"context"
	"log"
	"time"
)

const channelProbeBudgetSchemaSQL = `
CREATE TABLE IF NOT EXISTS connection_health_channel_probe_budget_usage (
 user_id text NOT NULL,
 admin_account_id text NOT NULL DEFAULT '',
 policy_id text NOT NULL,
 target_id text NOT NULL,
 day_start timestamptz NOT NULL,
 used integer NOT NULL DEFAULT 0,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (user_id, admin_account_id, policy_id, target_id, day_start)
);
CREATE INDEX IF NOT EXISTS idx_connection_health_events_channel_budget
ON connection_health_events (user_id, admin_account_id, policy_id, connection_id, created_at) INCLUDE (result);`

type ProbeBudgetUsage struct {
	PolicyID string
	TargetID string
	Used     int
}

type ChannelProbeBudget struct {
	PolicyID   string    `json:"policyId"`
	PolicyName string    `json:"policyName"`
	Models     []string  `json:"models"`
	Used       int       `json:"used"`
	Limit      int       `json:"limit"`
	Exhausted  bool      `json:"exhausted"`
	ResetsAt   time.Time `json:"resetsAt"`
}

// One workspace read, including historical events before counters were created.
// Action-only events and unmanaged manual tests do not consume a policy quota.
func (r *Repository) ListProbeBudgetUsage(ctx context.Context, userID, adminAccountID string, dayStart time.Time) ([]ProbeBudgetUsage, error) {
	rows, err := r.db.Query(ctx, `
 SELECT policy_id, target_id, max(used) FROM (
   SELECT policy_id, connection_id AS target_id, count(*) AS used
   FROM connection_health_events
   WHERE user_id=$1 AND admin_account_id=$2 AND policy_id<>''
     AND created_at >= $3 AND created_at < $3 + interval '1 day' AND result=ANY($4)
   GROUP BY policy_id, connection_id
   UNION ALL
   SELECT policy_id, target_id, used FROM connection_health_channel_probe_budget_usage
   WHERE user_id=$1 AND admin_account_id=$2 AND day_start=$3
 ) usage GROUP BY policy_id, target_id`, userID, adminAccountID, dayStart, probeResultKeys())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ProbeBudgetUsage
	for rows.Next() {
		var item ProbeBudgetUsage
		if err := rows.Scan(&item.PolicyID, &item.TargetID, &item.Used); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) attachChannelProbeBudgets(ctx context.Context, userID, adminAccountID string, groups []AdminGroupHealth, policies map[string]Policy) {
	dayStart := probeBudgetDayStart(time.Now())
	usage, err := s.repo.ListProbeBudgetUsage(ctx, userID, adminAccountID, dayStart)
	if err != nil {
		log.Printf("[connection-health] channel probe budget read failed: %v", err)
	}
	counts := make(map[string]map[string]int)
	for _, item := range usage {
		if counts[item.TargetID] == nil {
			counts[item.TargetID] = make(map[string]int)
		}
		counts[item.TargetID][item.PolicyID] = item.Used
	}
	// Merge all actual memberships before choosing the same winning model policy
	// as the scheduler. A shared channel must show the same quota in every group.
	byTarget := make(map[string][]Policy)
	models := make(map[string][]string)
	suspensionEnabled := make(map[string]bool)
	for _, group := range groups {
		for _, account := range group.Accounts {
			suspensionEnabled[account.TargetID] = account.SuspensionEnabled
			models[account.TargetID] = append(models[account.TargetID], splitModelList(account.Models)...)
			for _, id := range account.AssignedPolicyIDs {
				if policy, ok := policies[id]; ok {
					byTarget[account.TargetID] = mergePoliciesByID(byTarget[account.TargetID], []Policy{policy})
				}
			}
		}
	}
	budgets := make(map[string][]ChannelProbeBudget)
	for target, effective := range byTarget {
		indexes := make(map[string]int)
		for _, spec := range candidateModelSpecs(models[target], channelSuspensionPolicies(effective, suspensionEnabled[target])) {
			id := spec.policy.ID
			index, exists := indexes[id]
			if !exists {
				index = len(budgets[target])
				indexes[id] = index
				used, limit := counts[target][id], probeBudgetLimit(spec.policy)
				budgets[target] = append(budgets[target], ChannelProbeBudget{
					PolicyID: id, PolicyName: spec.policy.Name, Used: used, Limit: limit,
					Exhausted: used >= limit, ResetsAt: dayStart.Add(24 * time.Hour),
				})
			}
			budgets[target][index].Models = append(budgets[target][index].Models, spec.modelName)
		}
	}
	for gi := range groups {
		for ai := range groups[gi].Accounts {
			account := &groups[gi].Accounts[ai]
			account.ProbeBudgetError = err != nil && len(budgets[account.TargetID]) > 0
			if err == nil {
				account.ProbeBudgets = budgets[account.TargetID]
			}
		}
	}
}
