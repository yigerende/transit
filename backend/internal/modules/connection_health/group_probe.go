package connection_health

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"transithub/backend/internal/modules/upstream"
)

const groupProbePrefix = "admin.connectionHealth.groupProbe."

type groupProbeKeyProvider interface {
	EnsureSub2APIGroupProbeKey(upstream.Session, string, string) (upstream.ProbeCredential, error)
}

type GroupProbePreparation struct {
	Models               []DiscoveredModel `json:"models"`
	ModelListUnavailable bool              `json:"modelListUnavailable"`
}

func groupProbeTargetID(adminAccountID, groupID string) string {
	return "group:" + adminAccountID + ":" + groupID
}

func (s *Service) withGroupProbeCredential(ctx context.Context, userID, groupID string, action func(string, upstream.AdminGroupInfo, upstream.ProbeCredential) error) error {
	adminAccountID, err := s.currentAdminAccountID(ctx, userID)
	if err != nil {
		return err
	}
	session, err := s.mySites.RequireSession(ctx, userID, adminAccountID)
	if err != nil {
		return err
	}
	if session.Platform != upstream.PlatformSub2API {
		return requestError(groupProbePrefix + "unsupported")
	}
	if strings.TrimSpace(session.AccessToken) == "" {
		return requestError(groupProbePrefix + "loginRequired")
	}
	if s.platformGroups == nil {
		return requestError(ErrorUnknown)
	}
	groups, err := s.platformGroups.FetchAdminAllGroups(session)
	if err != nil {
		return requestError(ErrorRequest)
	}
	var group *upstream.AdminGroupInfo
	for i := range groups {
		if groups[i].ID == groupID {
			group = &groups[i]
			break
		}
	}
	if group == nil {
		return requestError(ErrorNotFound)
	}
	provider, ok := s.platformGroups.(groupProbeKeyProvider)
	if !ok {
		return requestError(groupProbePrefix + "unsupported")
	}
	release, err := s.repo.AcquireTargetLease(ctx, groupProbeTargetID(adminAccountID, groupID))
	if err != nil {
		return err
	}
	defer release()
	digest := sha256.Sum256([]byte(userID + "|" + adminAccountID + "|" + groupID))
	keyName := fmt.Sprintf("monitor-group-%s-%x", groupID, digest[:6])
	credential, err := provider.EnsureSub2APIGroupProbeKey(session, groupID, keyName)
	if err != nil {
		return requestError(groupProbePrefix + "keyUnavailable")
	}
	// The gateway address is always taken from the authenticated workspace.
	credential.BaseURL = session.BaseURL
	return action(adminAccountID, *group, credential)
}

// Preparing is a POST because the first explicit click may create a remote key.
func (s *Service) PrepareGroupProbe(ctx context.Context, userID, groupID string) (GroupProbePreparation, error) {
	result := GroupProbePreparation{Models: []DiscoveredModel{}}
	err := s.withGroupProbeCredential(ctx, userID, groupID, func(_ string, _ upstream.AdminGroupInfo, cred upstream.ProbeCredential) error {
		models, err := s.modelDiscovery.ListModels(ctx, cred.BaseURL, cred.Key)
		if err != nil {
			result.ModelListUnavailable = true
			return nil
		}
		result.Models = models
		return nil
	})
	return result, err
}

func (s *Service) ProbeAdminGroup(ctx context.Context, userID, groupID, model string) (GroupProbeSample, error) {
	model = strings.TrimSpace(model)
	if model == "" || len(model) > 200 {
		return GroupProbeSample{}, requestError(groupProbePrefix + "modelRequired")
	}
	var sample GroupProbeSample
	err := s.withGroupProbeCredential(ctx, userID, groupID, func(adminAccountID string, group upstream.AdminGroupInfo, cred upstream.ProbeCredential) error {
		outcome := s.probeRunner.ProbeGroup(ctx, ProbeRequest{BaseURL: cred.BaseURL, UpstreamKey: cred.Key, ProviderFamily: group.Platform, ModelName: model})
		id, err := newID()
		if err != nil {
			return err
		}
		sample = GroupProbeSample{ID: id, TargetID: groupProbeTargetID(adminAccountID, groupID), ModelName: model, Result: string(outcome.Result), LatencyMs: &outcome.LatencyMs, CreatedAt: time.Now()}
		// Group results do not drive account state, policy budgets or remote actions.
		return s.repo.InsertEvent(ctx, ConnectionHealthEvent{ID: id, ConnectionID: sample.TargetID, UserID: userID, AdminAccountID: adminAccountID, AdminGroupID: groupID, OwnGroupName: group.Name, UpstreamGroupName: group.Name, ModelName: model, Result: sample.Result, LatencyMs: sample.LatencyMs, CreatedAt: sample.CreatedAt})
	})
	return sample, err
}
