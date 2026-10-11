package connection_health

import (
	"context"
	"strings"

	"transithub/backend/internal/modules/upstream"
)

// Caller holds the action lease and supplies a fresh, complete inventory. An
// explicit opt-out releases the quality owner even without health samples, but
// never releases a health model still awaiting its own recovery threshold.
func (s *Service) releaseChannelQualitySuspensionLocked(ctx context.Context, user, workspace string, session upstream.Session, target AdminProbeTarget, specs []probeModelSpec, stored *TargetActionState) (string, error) {
	if stored == nil || !stored.QualitySuspended {
		return "", nil
	}
	specs = append([]probeModelSpec(nil), specs...)
	for i := range specs {
		// A failed policy read cannot be treated as revoked permission: that
		// would incorrectly release an independent health suspension.
		latest, err := s.repo.GetPolicy(ctx, specs[i].policy.ID, user, workspace)
		if err != nil {
			return "", err
		}
		if latest == nil || !policySupportsProbing(*latest) {
			specs[i].policy.Enabled = false
		} else {
			specs[i].policy = *latest
		}
	}
	enabled, err := s.repo.GetChannelSuspension(ctx, user, workspace, target.TargetID)
	if err != nil {
		return "", err
	}
	if enabled && hasRemoteActionModel(specs) {
		states, err := s.repo.ListStatesByConnection(ctx, target.TargetID)
		if err != nil {
			return "", err
		}
		for _, spec := range specs {
			if !spec.policy.Enabled || !policyRemoteActionEnabled(spec.policy) {
				continue
			}
			for _, state := range states {
				if state.ModelName == spec.modelName && state.State != StateHealthy {
					stored.QualitySuspended = false
					return "", s.repo.UpsertTargetActionState(ctx, *stored)
				}
			}
		}
	}
	currentStatus, currentWeight := normalizeTargetStatus(target.Platform, target.AccountStatus), normalizedTargetWeight(target)
	if stored.Conflict || targetActionCheckpointConflicted(target, stored, currentStatus, currentWeight) {
		stored.Conflict, stored.PendingStatus, stored.PendingWeight = true, "", nil
		return RemoteActionSkippedTargetConflict, s.repo.UpsertTargetActionState(ctx, *stored)
	}
	if targetStateEqual(target, currentStatus, currentWeight, stored.OriginalStatus, stored.OriginalWeight) {
		return "", s.repo.DeleteTargetActionState(ctx, user, workspace, target.TargetID)
	}
	stored.PendingStatus, stored.PendingWeight = stored.OriginalStatus, cloneIntPointer(stored.OriginalWeight)
	if err := s.repo.UpsertTargetActionState(ctx, *stored); err != nil {
		return "", err
	}
	action, err := s.dispatcher.ApplyTargetState(ctx, session, target, stored.OriginalWeight, stored.OriginalStatus)
	if err != nil {
		return action, err
	}
	s.invalidateMonitorAccount(user, workspace, target.AccountID)
	return action, s.repo.DeleteTargetActionState(ctx, user, workspace, target.TargetID)
}

// Quality is a separate reason to hold the shared upstream account inactive.
// It never rewrites health results or contributes to health latency samples.
func (s *Service) qualitySuspensionBlocked(ctx context.Context, user, workspace, target string, stored *TargetActionState) (bool, error) {
	if s.qualityRepo == nil {
		return false, nil
	}
	enabled, err := s.repo.GetChannelQualitySuspension(ctx, user, workspace, target)
	if err != nil || !enabled {
		return false, err
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
