package connection_health

import (
	"context"
	"errors"
	"strings"
	"time"

	"transithub/backend/internal/modules/upstream"
)

type ManualProbeResult struct {
	ModelName   string    `json:"modelName"`
	ProbeMode   string    `json:"probeMode"`
	PolicyID    string    `json:"policyId,omitempty"`
	Result      string    `json:"result"`
	Healthy     bool      `json:"healthy"`
	LatencyMs   *int      `json:"latencyMs"`
	ErrorKey    string    `json:"errorKey"`
	ErrorDetail string    `json:"errorDetail"`
	ProbedAt    time.Time `json:"probedAt"`
}

// Read all actual memberships; a partial inventory cannot choose a safe winner.
func (s *Service) manualProbeJob(ctx context.Context, userID, targetID string) (adminProbeJob, error) {
	session, target, account, workspace, err := s.resolveManualTarget(ctx, userID, targetID)
	if err != nil {
		return adminProbeJob{}, err
	}
	job := adminProbeJob{userID: userID, adminAccountID: workspace, session: session, target: target, account: account}
	inventory, err := s.loadAdminInventory(ctx, userID, workspace, make(adminInventoryCache))
	if err != nil {
		return job, err
	}
	for _, group := range inventory.groups {
		if group.err != nil {
			return job, requestError(ErrorAccountsFetch)
		}
		for _, acc := range group.accounts {
			if acc.ID == target.AccountID {
				job.groups = append(job.groups, group.group)
				job.account = acc
				job.target.AccountStatus, job.target.AccountWeight = acc.Status, cloneIntPointer(acc.Weight)
				break
			}
		}
	}
	if len(job.groups) == 0 {
		return job, requestError(ErrorProbeTargetNotFound)
	}
	job.models, err = s.currentScheduledModels(ctx, job)
	return job, err
}

// Manual requests skip the interval but share the scheduler's policy engine.
func (s *Service) ManualProbeTarget(ctx context.Context, userID, targetID string, models []string) ([]ManualProbeResult, error) {
	return s.manualProbeTarget(ctx, userID, targetID, models, true)
}

// The history flag remains compatible with older clients; real probes always persist.
func (s *Service) manualProbeTarget(ctx context.Context, userID, targetID string, models []string, _ bool) (_ []ManualProbeResult, retErr error) {
	requested := []string{}
	seen := map[string]bool{}
	for _, name := range models {
		name = strings.TrimSpace(name)
		if name != "" && !seen[name] {
			requested = append(requested, name)
			seen[name] = true
		}
	}
	if len(requested) == 0 {
		return nil, requestError(ErrorManualModelsRequired)
	}
	if _, _, _, _, err := s.resolveManualTarget(ctx, userID, targetID); err != nil {
		return nil, err
	}
	release, err := s.repo.AcquireTargetLease(ctx, targetID)
	if err != nil {
		return nil, err
	}
	defer release()
	job, err := s.manualProbeJob(ctx, userID, targetID)
	if err != nil {
		return nil, err
	}
	byModel := map[string]probeModelSpec{}
	for _, spec := range job.models {
		byModel[spec.modelName] = spec
	}
	specs := make([]probeModelSpec, 0, len(requested))
	for _, name := range requested {
		spec, ok := byModel[name]
		if !ok {
			if len(job.models) > 0 {
				return nil, requestError(ErrorNoMatchingModels)
			}
			spec = probeModelSpec{modelName: name, providerFamily: job.target.ProviderFamily, maxProbeTokens: 1, policy: Policy{ProbeMode: ProbeModeLight}}
		}
		specs = append(specs, spec)
	}
	var cred upstream.ProbeCredential
	var credentialErr error
	if probeSpecsNeedCredentials(specs) {
		cred, credentialErr = s.platformGroups.ResolveProbeCredential(job.session, job.account)
		if credentialErr != nil && len(job.models) == 0 {
			return nil, requestError(reasonToErrorKey(upstream.ProbeCredentialReason(credentialErr)))
		}
	}
	results := make([]ManualProbeResult, 0, len(specs))
	probes := []targetProbeResult{}
	// Finish completed models even if a later model cannot run.
	defer func() {
		s.refreshProbeActionTarget(&job)
		retErr = errors.Join(retErr, s.finishTargetProbeBatch(ctx, userID, job.adminAccountID, job.session, job.target, job.models, probes))
	}()
	for _, spec := range specs {
		result := ManualProbeResult{ModelName: spec.modelName, ProbeMode: normalizeProbeMode(spec.policy.ProbeMode), PolicyID: spec.policy.ID, ProbedAt: time.Now()}
		if credentialErr != nil && result.ProbeMode != ProbeModeSub2API {
			result.Result, result.ErrorKey = string(ResultUnsupported), reasonToErrorKey(upstream.ProbeCredentialReason(credentialErr))
			results = append(results, result)
			continue
		}
		var outcome ProbeOutcome
		if spec.policy.ID != "" {
			probe, probeErr := s.probeTargetOnce(ctx, userID, job.adminAccountID, job.target, cred, spec, job.session)
			if probeErr != nil {
				return nil, probeErr
			}
			if probe == nil {
				result.Result, result.ErrorKey = string(ResultUnsupported), "admin.connectionHealth.errors.probeBudgetExhausted"
				results = append(results, result)
				continue
			}
			probe.manual = true
			probes = append(probes, *probe)
			outcome, result.ProbedAt = probe.outcome, *probe.state.LastProbeAt
		} else {
			outcome = s.probeRunner.Probe(ctx, ProbeRequest{BaseURL: cred.BaseURL, UpstreamKey: cred.Key, ProviderFamily: job.target.ProviderFamily, ModelName: spec.modelName, MaxTokens: 1})
			result.ProbedAt = time.Now()
			id, idErr := newID()
			if idErr != nil {
				return nil, idErr
			}
			errorKey := ""
			if outcome.Result != ResultOK {
				errorKey = string(outcome.Result)
			}
			if err := s.repo.InsertEvent(ctx, ConnectionHealthEvent{ID: id, Manual: true, ProbeMode: ProbeModeLight, ConnectionID: targetID, UserID: userID, AdminAccountID: job.adminAccountID, AdminGroupID: job.target.AdminGroupID, OwnGroupName: job.target.AdminGroupName, UpstreamGroupName: job.target.AdminGroupName, ModelName: spec.modelName, Result: string(outcome.Result), LatencyMs: intPtr(outcome.LatencyMs), ErrorKey: errorKey, ErrorDetail: outcome.Detail, CreatedAt: result.ProbedAt}); err != nil {
				return nil, err
			}
		}
		result.Result, result.Healthy, result.LatencyMs = string(outcome.Result), outcome.Result == ResultOK, intPtr(outcome.LatencyMs)
		if !result.Healthy {
			result.ErrorKey, result.ErrorDetail = result.Result, outcome.Detail
		}
		results = append(results, result)
	}
	return results, nil
}

func intPtr(v int) *int { return &v }
