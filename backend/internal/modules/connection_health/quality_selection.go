package connection_health

import "context"

// Quality detection uses automation's saved channel selection, while its own
// global/group switches control execution independently of health probe settings.
type qualitySelection struct {
	disabledChannels map[string]bool
	groups           map[string]bool
	targets          map[string]bool
	excluded         map[string]map[string]bool
}

func newQualitySelection(policies []Policy, assignments []PolicyAssignment, groups []GroupPolicyAssignment, exclusions []GroupTargetExclusion) qualitySelection {
	selection := qualitySelection{groups: map[string]bool{}, targets: map[string]bool{}, excluded: map[string]map[string]bool{}}
	known := map[string]bool{}
	for _, p := range policies {
		known[p.ID] = true
	}
	for _, a := range assignments {
		if known[a.PolicyID] {
			selection.targets[a.TargetID] = true
		}
	}
	for _, a := range groups {
		if known[a.PolicyID] {
			selection.groups[a.AdminGroupID] = true
		}
	}
	for _, e := range exclusions {
		if selection.excluded[e.AdminGroupID] == nil {
			selection.excluded[e.AdminGroupID] = map[string]bool{}
		}
		selection.excluded[e.AdminGroupID][e.TargetID] = true
	}
	return selection
}

func (s qualitySelection) selected(group, target string) bool {
	// Explicitly unchecking a channel in this group also excludes it from quality
	// checks, even if a legacy per-channel health policy remains assigned.
	return !s.disabledChannels[target] && !s.excluded[group][target] && (s.groups[group] || s.targets[target])
}

func (s *Service) loadQualitySelection(ctx context.Context, user, workspace string) (qualitySelection, error) {
	policies, err := s.repo.ListPolicies(ctx, user, workspace)
	if err != nil {
		return qualitySelection{}, err
	}
	assignments, err := s.repo.ListPolicyAssignmentsByWorkspace(ctx, user, workspace)
	if err != nil {
		return qualitySelection{}, err
	}
	groups, err := s.repo.ListGroupPolicyAssignmentsByWorkspace(ctx, user, workspace)
	if err != nil {
		return qualitySelection{}, err
	}
	exclusions, err := s.repo.ListGroupTargetExclusionsByWorkspace(ctx, user, workspace)
	if err != nil {
		return qualitySelection{}, err
	}
	selection := newQualitySelection(policies, assignments, groups, exclusions)
	if s.qualityRepo != nil {
		channels, err := s.qualityRepo.ListQualityChannels(ctx, user, workspace)
		if err != nil {
			return qualitySelection{}, err
		}
		selection.disabledChannels = map[string]bool{}
		for _, channel := range channels {
			selection.disabledChannels[channel.TargetID] = !channel.Enabled
		}
	}
	return selection, nil
}
