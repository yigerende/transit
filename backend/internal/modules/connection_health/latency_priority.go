package connection_health

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"transithub/backend/internal/modules/upstream"
)

const PriorityModeLatency = "latency"
const ErrorLatencyPriorityInvalid = "admin.connectionHealth.errors.latencyPriorityInvalid"

type LatencyPriorityBand struct {
	MinSeconds float64  `json:"minSeconds"`
	MaxSeconds *float64 `json:"maxSeconds"`
	Priority   int      `json:"priority"`
}

type LatencyPriorityConfig struct {
	ModelName            string                `json:"modelName"`
	SampleCount          int                   `json:"sampleCount"`
	MinSamples           int                   `json:"minSamples"`
	MaxAgeSeconds        int                   `json:"maxAgeSeconds"`
	Weights              []float64             `json:"weights"`
	HysteresisSeconds    float64               `json:"hysteresisSeconds"`
	Bands                []LatencyPriorityBand `json:"bands"`
	InsufficientPriority int                   `json:"insufficientPriority"`
	DegradedPriority     int                   `json:"degradedPriority"`
	SuspendedPriority    int                   `json:"suspendedPriority"`
}

func latencyFloat(v float64) *float64 { return &v }

func defaultLatencyPriorityConfig() *LatencyPriorityConfig {
	return &LatencyPriorityConfig{
		SampleCount: 3, MinSamples: 1, MaxAgeSeconds: 180, Weights: []float64{50, 30, 20}, HysteresisSeconds: .5,
		Bands:                []LatencyPriorityBand{{0, latencyFloat(5), 1}, {5, latencyFloat(10), 2}, {10, latencyFloat(15), 3}, {15, nil, 4}},
		InsufficientPriority: 50, DegradedPriority: 100, SuspendedPriority: 10000,
	}
}

func latencyConfig(policy Policy) LatencyPriorityConfig {
	if policy.LatencyPriority != nil {
		return *policy.LatencyPriority
	}
	return *defaultLatencyPriorityConfig()
}

func validateLatencyPriority(c LatencyPriorityConfig) bool {
	finite := func(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
	if c.SampleCount < 1 || c.SampleCount > 20 || c.MinSamples < 1 || c.MinSamples > c.SampleCount || c.MaxAgeSeconds < 1 || c.MaxAgeSeconds > 86400 || len(c.Weights) != c.SampleCount || len(c.Bands) < 1 || len(c.Bands) > 20 || !finite(c.HysteresisSeconds) || c.HysteresisSeconds < 0 || c.HysteresisSeconds > 3600 {
		return false
	}
	for _, w := range c.Weights {
		if !finite(w) || w <= 0 || w > 1000000 {
			return false
		}
	}
	validPriority := func(p int) bool { return p >= 0 && p <= 2147483647 }
	if !validPriority(c.InsufficientPriority) || !validPriority(c.DegradedPriority) || !validPriority(c.SuspendedPriority) {
		return false
	}
	end := 0.0
	priorities := map[int]bool{}
	for i, band := range c.Bands {
		if !finite(band.MinSeconds) || band.MinSeconds != end || !validPriority(band.Priority) || priorities[band.Priority] {
			return false
		}
		priorities[band.Priority] = true
		if band.MaxSeconds == nil {
			return i == len(c.Bands)-1
		}
		if !finite(*band.MaxSeconds) || *band.MaxSeconds <= end || *band.MaxSeconds > 86400 {
			return false
		}
		end = *band.MaxSeconds
	}
	return false
}

// Each sample is an automatic successful probe of one model. Failed requests,
// manual probes, expired samples and other models never enter the average.
type PriorityProbeSample struct {
	ID        string    `json:"id"`
	TargetID  string    `json:"-"`
	ModelName string    `json:"-"`
	LatencyMs int       `json:"latencyMs"`
	CreatedAt time.Time `json:"createdAt"`
}

type LatencyPriorityDecision struct {
	PolicyID          string                `json:"policyId"`
	PolicyName        string                `json:"policyName"`
	ModelName         string                `json:"modelName"`
	AverageMs         *float64              `json:"averageMs"`
	SampleCount       int                   `json:"sampleCount"`
	RequiredSamples   int                   `json:"requiredSamples"`
	MaxAgeSeconds     int                   `json:"maxAgeSeconds"`
	Samples           []PriorityProbeSample `json:"samples"`
	Weights           []float64             `json:"weights"`
	BandIndex         *int                  `json:"bandIndex"`
	Priority          int                   `json:"priority"`
	Reason            string                `json:"reason"`
	SharedPolicyCount int                   `json:"sharedPolicyCount"`
}

func hasLatencyPriorityPolicy(policies []Policy) bool {
	for _, p := range policies {
		if p.Enabled && p.PriorityMode == PriorityModeLatency && normalizeStrategyMode(p.StrategyMode) == StrategyModeHealthProbe {
			return true
		}
	}
	return false
}

func latencyPriorityModel(p Policy) string {
	c := latencyConfig(p)
	if c.ModelName != "" {
		return c.ModelName
	}
	names := []string{}
	for _, m := range p.ModelTargets {
		if m.Enabled && strings.TrimSpace(m.ModelName) != "" {
			names = append(names, m.ModelName)
		}
	}
	sort.Strings(names)
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

func evaluateLatencyPriority(p Policy, states []ConnectionHealthState, samples []PriorityProbeSample, previous *PrioritySyncState, now time.Time) LatencyPriorityDecision {
	c := latencyConfig(p)
	d := LatencyPriorityDecision{PolicyID: p.ID, PolicyName: p.Name, ModelName: latencyPriorityModel(p), RequiredSamples: c.SampleCount, MaxAgeSeconds: c.MaxAgeSeconds, Samples: []PriorityProbeSample{}, Weights: append([]float64(nil), c.Weights...), Priority: c.InsufficientPriority, Reason: "insufficient", SharedPolicyCount: 1}
	cutoff := now.Add(-time.Duration(c.MaxAgeSeconds) * time.Second)
	for _, sample := range samples {
		if sample.ModelName == d.ModelName && sample.LatencyMs >= 0 && !sample.CreatedAt.Before(cutoff) && !sample.CreatedAt.After(now) {
			d.Samples = append(d.Samples, sample)
		}
	}
	sort.SliceStable(d.Samples, func(i, j int) bool {
		if d.Samples[i].CreatedAt.Equal(d.Samples[j].CreatedAt) {
			return d.Samples[i].ID > d.Samples[j].ID
		}
		return d.Samples[i].CreatedAt.After(d.Samples[j].CreatedAt)
	})
	if len(d.Samples) > c.SampleCount {
		d.Samples = d.Samples[:c.SampleCount]
	}
	d.SampleCount = len(d.Samples)
	if d.SampleCount > 0 {
		total, weights := 0.0, 0.0
		for i, sample := range d.Samples {
			total += float64(sample.LatencyMs) * c.Weights[i]
			weights += c.Weights[i]
		}
		average := total / weights
		d.AverageMs = &average
	}
	// Health thresholds remain authoritative, independently of the averaging window.
	degraded := false
	for _, st := range states {
		active := false
		for _, model := range p.ModelTargets {
			if model.Enabled && model.ModelName == st.ModelName {
				active = true
				break
			}
		}
		if !active || (!p.AutoDegradeEnabled && st.State != StateDisabled) {
			continue
		}
		st = stateWithoutSuspension(st, p)
		if st.State == StateSuspended || st.State == StateDisabled {
			d.Priority = c.SuspendedPriority
			d.Reason = "suspended"
			return d
		}
		if st.State == StateDegraded {
			degraded = true
		}
	}
	if degraded {
		d.Priority = c.DegradedPriority
		d.Reason = "degraded"
		return d
	}
	// Partial samples are visible but do not grant an unproven channel a fast tier.
	if d.SampleCount < c.MinSamples {
		return d
	}
	seconds := *d.AverageMs / 1000
	index := len(c.Bands) - 1
	for i, band := range c.Bands {
		if band.MaxSeconds == nil || seconds <= *band.MaxSeconds {
			index = i
			break
		}
	}
	if previous != nil && !previous.Conflict {
		for i, band := range c.Bands {
			if band.Priority != previous.LastAppliedPriority {
				continue
			}
			if index > i && band.MaxSeconds != nil && seconds <= *band.MaxSeconds+c.HysteresisSeconds {
				index = i
			}
			if index < i && seconds >= band.MinSeconds-c.HysteresisSeconds {
				index = i
			}
			break
		}
	}
	d.BandIndex = &index
	d.Priority = c.Bands[index].Priority
	d.Reason = "latency"
	return d
}

// Shared accounts get one deterministic write. Latency policies supersede legacy
// multiplier mode, and the least preferred configured value wins across policies.
func latencyDecisionForTarget(platform upstream.Platform, policies []Policy, states []ConnectionHealthState, samples []PriorityProbeSample, previous *PrioritySyncState, now time.Time) *LatencyPriorityDecision {
	var chosen *LatencyPriorityDecision
	count := 0
	for _, p := range policies {
		if !hasLatencyPriorityPolicy([]Policy{p}) {
			continue
		}
		count++
		d := evaluateLatencyPriority(p, states, samples, previous, now)
		worse := chosen == nil
		if chosen != nil {
			if platform == upstream.PlatformNewAPI {
				worse = d.Priority < chosen.Priority
			} else {
				worse = d.Priority > chosen.Priority
			}
			if d.Priority == chosen.Priority {
				worse = d.PolicyID < chosen.PolicyID
			}
		}
		if worse {
			chosen = &d
		}
	}
	if chosen != nil {
		chosen.SharedPolicyCount = count
	}
	return chosen
}

func (r *Repository) ListPriorityProbeSamples(ctx context.Context, userID, adminAccountID string, targetIDs []string, since time.Time) ([]PriorityProbeSample, error) {
	rows, err := r.db.Query(ctx, `SELECT id, connection_id, model_name, latency_ms, created_at FROM (
		SELECT id, connection_id, model_name, latency_ms, created_at,
		row_number() OVER (PARTITION BY connection_id, model_name ORDER BY created_at DESC,id DESC) AS rn
		FROM connection_health_events WHERE user_id=$1 AND admin_account_id=$2 AND connection_id=ANY($3)
		AND result='ok' AND policy_id<>'' AND latency_ms>=0 AND created_at >= $4
	) samples WHERE rn<=20 ORDER BY created_at DESC,id DESC`, userID, adminAccountID, targetIDs, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PriorityProbeSample{}
	for rows.Next() {
		var p PriorityProbeSample
		if err := rows.Scan(&p.ID, &p.TargetID, &p.ModelName, &p.LatencyMs, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) prioritySamples(ctx context.Context, userID, adminAccountID string, policies []Policy, targetIDs []string, now time.Time) (map[string][]PriorityProbeSample, error) {
	age := 0
	for _, p := range policies {
		if hasLatencyPriorityPolicy([]Policy{p}) {
			age = maxInt(age, latencyConfig(p).MaxAgeSeconds)
		}
	}
	result := map[string][]PriorityProbeSample{}
	if age == 0 || len(targetIDs) == 0 {
		return result, nil
	}
	samples, err := s.repo.ListPriorityProbeSamples(ctx, userID, adminAccountID, targetIDs, now.Add(-time.Duration(age)*time.Second))
	if err != nil {
		return nil, err
	}
	for _, sample := range samples {
		result[sample.TargetID] = append(result[sample.TargetID], sample)
	}
	return result, nil
}

func (s *Service) attachLatencyPriorities(ctx context.Context, userID, adminAccountID string, platform upstream.Platform, groups []AdminGroupHealth, policies []Policy, states []ConnectionHealthState, syncStates []PrioritySyncState) {
	policyByID := map[string]Policy{}
	for _, p := range policies {
		policyByID[p.ID] = p
	}
	policiesByTarget := map[string][]Policy{}
	for _, group := range groups {
		for _, account := range group.Accounts {
			for _, id := range account.AssignedPolicyIDs {
				if p, ok := policyByID[id]; ok {
					policiesByTarget[account.TargetID] = mergePoliciesByID(policiesByTarget[account.TargetID], []Policy{p})
				}
			}
		}
	}
	targetIDs := []string{}
	for id, ps := range policiesByTarget {
		if hasLatencyPriorityPolicy(ps) {
			targetIDs = append(targetIDs, id)
		}
	}
	now := time.Now()
	samples, err := s.prioritySamples(ctx, userID, adminAccountID, policies, targetIDs, now)
	statesByTarget := map[string][]ConnectionHealthState{}
	for _, state := range states {
		statesByTarget[state.ConnectionID] = append(statesByTarget[state.ConnectionID], state)
	}
	previous := map[string]PrioritySyncState{}
	for _, st := range syncStates {
		previous[st.TargetID] = st
	}
	for gi := range groups {
		for ai := range groups[gi].Accounts {
			a := &groups[gi].Accounts[ai]
			if !hasLatencyPriorityPolicy(policiesByTarget[a.TargetID]) {
				continue
			}
			if err != nil {
				a.LatencyPriorityError = true
				continue
			}
			var prior *PrioritySyncState
			if st, ok := previous[a.TargetID]; ok {
				prior = &st
			}
			a.LatencyPriority = latencyDecisionForTarget(platform, policiesByTarget[a.TargetID], statesByTarget[a.TargetID], samples[a.TargetID], prior, now)
		}
	}
}
