package connection_health

import (
	"context"
	"strings"
)

// Quality is a separate reason to hold the shared upstream account inactive.
// It never rewrites health results or contributes to health latency samples.
func (s *Service) qualitySuspensionBlocked(ctx context.Context, user, workspace, target string, stored *TargetActionState) (bool, error) {
	if s.qualityRepo == nil {
		return false, nil
	}
	q, err := s.qualityRepo.GetQualitySettings(ctx, user, workspace)
	if err != nil {
		return false, err
	}
	if !q.AutoSuspendEnabled || q.Revision == "" {
		return false, nil
	}
	states, err := s.qualityRepo.ListQualityStates(ctx, user, workspace)
	if err != nil {
		return false, err
	}
	for _, state := range states {
		if state.TargetID == target && state.Revision == q.Revision {
			return state.Degraded, nil
		}
	}
	// A new question/model configuration must prove recovery before releasing an
	// existing hold. Turning off quality suspension explicitly removes this hold.
	return stored != nil && stored.QualitySuspended, nil
}

func (s *Service) reconcileQualityTarget(ctx context.Context, user, workspace, targetID string) error {
	release, err := s.repo.AcquireTargetLease(ctx, targetID)
	if err != nil {
		return err
	}
	defer release()
	inventory, err := s.loadAdminInventory(ctx, user, workspace, make(adminInventoryCache))
	if err != nil {
		return err
	}
	job := adminProbeJob{userID: user, adminAccountID: workspace, session: inventory.session}
	for _, group := range inventory.groups {
		if group.err != nil {
			return requestError(ErrorAccountsFetch)
		}
		for _, account := range group.accounts {
			if buildTargetID(string(inventory.session.Platform), workspace, account.ID) != targetID {
				continue
			}
			if job.target.TargetID == "" {
				job.target = AdminProbeTarget{TargetID: targetID, Platform: string(inventory.session.Platform), AdminGroupID: group.group.ID, AdminGroupName: group.group.Name, AccountID: account.ID, AccountName: account.Name, AccountStatus: account.Status, AccountWeight: cloneIntPointer(account.Weight), ProviderFamily: account.Platform, Models: splitModelList(account.Models)}
				job.account = account
			}
			job.groups = append(job.groups, group.group)
		}
	}
	if job.target.TargetID == "" {
		return requestError(ErrorProbeTargetNotFound)
	}
	job.models, err = s.currentScheduledModels(ctx, job)
	if err != nil {
		return err
	}
	if !hasRemoteActionModel(job.models) {
		return nil
	}
	action, actionErr := s.reconcileTargetRemoteAction(ctx, user, workspace, job.session, job.target, job.models)
	if action != "" || actionErr != nil {
		s.recordQualityAction(ctx, user, workspace, job.target, job.models, action, actionErr)
	}
	return actionErr
}

func (s *Service) recordQualityAction(ctx context.Context, user, workspace string, target AdminProbeTarget, specs []probeModelSpec, action string, actionErr error) {
	result := "quality_suspend"
	switch {
	case action == RemoteActionSub2APIStatusActive || strings.HasPrefix(action, "newapi_channel_weight_"):
		result = "quality_restore"
	case action == RemoteActionSkippedTargetConflict || action == RemoteActionSkippedTargetInitiallyDisabled || action == RemoteActionUnsupported:
		result = "quality_action_skipped"
	}
	errorKey := ""
	if actionErr != nil {
		result = "quality_action_failed"
		errorKey = qualityPrefix + "actionFailed"
	}
	policyID := ""
	for _, spec := range specs {
		if spec.policy.Enabled && policyRemoteActionEnabled(spec.policy) {
			policyID = spec.policy.ID
			target = targetForProbeSpec(target, spec)
			break
		}
	}
	s.recordTargetEvent(ctx, user, workspace, target, policyID, "*quality*", result, "", "", nil, errorKey, "", action, false)
}
