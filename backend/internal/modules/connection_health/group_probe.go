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

type groupProbeCustomKeyProvider interface {
	ResolveSub2APIGroupProbeKey(upstream.Session, string, string, string) (upstream.ProbeCredential, string, error)
}

type GroupProbePreparation struct {
	Models               []DiscoveredModel `json:"models"`
	ModelListUnavailable bool              `json:"modelListUnavailable"`
}

func groupProbeTargetID(adminAccountID, groupID string) string {
	return "group:" + adminAccountID + ":" + groupID
}

func (s *Service) withGroupProbeCredential(ctx context.Context, userID, groupID string, key *string, action func(string, upstream.AdminGroupInfo, upstream.ProbeCredential) error) error {
	adminAccountID, err := s.currentAdminAccountID(ctx, userID)
	if err != nil {
		return err
	}
	release, err := s.repo.AcquireTargetLease(ctx, groupProbeTargetID(adminAccountID, groupID))
	if err != nil {
		return err
	}
	defer release()
	group, credential, _, err := s.resolveGroupProbeCredential(ctx, userID, adminAccountID, groupID, key)
	if err != nil {
		return err
	}
	return action(adminAccountID, group, credential)
}

// Caller holds the group target lease. An explicit workspace is required by
// background tasks; the user's currently selected workspace is irrelevant.
func (s *Service) resolveGroupProbeCredential(ctx context.Context, userID, adminAccountID, groupID string, key *string) (upstream.AdminGroupInfo, upstream.ProbeCredential, string, error) {
	fail := func(err error) (upstream.AdminGroupInfo, upstream.ProbeCredential, string, error) {
		return upstream.AdminGroupInfo{}, upstream.ProbeCredential{}, "", err
	}
	suppliedKey, keyID := "", ""
	if key != nil {
		suppliedKey = strings.TrimSpace(*key)
		if len(suppliedKey) > 4096 {
			return fail(requestError(groupProbePrefix + "customKeyUnavailable"))
		}
	} else {
		config, err := s.repo.GetGroupProbeConfig(ctx, userID, adminAccountID, groupID)
		if err != nil {
			return fail(err)
		}
		if config != nil {
			keyID = config.CustomKeyID
		}
	}
	session, err := s.mySites.RequireSession(ctx, userID, adminAccountID)
	if err != nil {
		return fail(requestError(groupProbePrefix + "loginRequired"))
	}
	if session.Platform != upstream.PlatformSub2API {
		return fail(requestError(groupProbePrefix + "unsupported"))
	}
	if suppliedKey == "" && keyID == "" && strings.TrimSpace(session.AccessToken) == "" {
		return fail(requestError(groupProbePrefix + "loginRequired"))
	}
	if s.platformGroups == nil {
		return fail(requestError(ErrorUnknown))
	}
	groups, err := s.platformGroups.FetchAdminAllGroups(session)
	if err != nil {
		return fail(requestError(ErrorRequest))
	}
	var group *upstream.AdminGroupInfo
	for i := range groups {
		if groups[i].ID == groupID {
			group = &groups[i]
			break
		}
	}
	if group == nil {
		return fail(requestError(ErrorNotFound))
	}
	if suppliedKey != "" || keyID != "" {
		provider, ok := s.platformGroups.(groupProbeCustomKeyProvider)
		if !ok {
			return fail(requestError(groupProbePrefix + "unsupported"))
		}
		cred, id, err := provider.ResolveSub2APIGroupProbeKey(session, groupID, suppliedKey, keyID)
		if err != nil {
			return fail(requestError(groupProbePrefix + "customKeyUnavailable"))
		}
		cred.BaseURL = session.BaseURL
		return *group, cred, id, nil
	}
	provider, ok := s.platformGroups.(groupProbeKeyProvider)
	if !ok {
		return fail(requestError(groupProbePrefix + "unsupported"))
	}
	digest := sha256.Sum256([]byte(userID + "|" + adminAccountID + "|" + groupID))
	keyName := fmt.Sprintf("monitor-group-%s-%x", groupID, digest[:6])
	credential, err := provider.EnsureSub2APIGroupProbeKey(session, groupID, keyName)
	if err != nil {
		return fail(requestError(groupProbePrefix + "keyUnavailable"))
	}
	// The gateway address is always taken from the authenticated workspace.
	credential.BaseURL = session.BaseURL
	return *group, credential, "", nil
}

// Preparing is a POST because the first explicit click may create a remote key.
func (s *Service) PrepareGroupProbe(ctx context.Context, userID, groupID string, key ...*string) (GroupProbePreparation, error) {
	result := GroupProbePreparation{Models: []DiscoveredModel{}}
	var selectedKey *string
	if len(key) > 0 {
		selectedKey = key[0]
	}
	err := s.withGroupProbeCredential(ctx, userID, groupID, selectedKey, func(_ string, _ upstream.AdminGroupInfo, cred upstream.ProbeCredential) error {
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
	err := s.withGroupProbeCredential(ctx, userID, groupID, nil, func(adminAccountID string, group upstream.AdminGroupInfo, cred upstream.ProbeCredential) error {
		var probeErr error
		sample, probeErr = s.probeGroupOnce(ctx, userID, adminAccountID, group, cred, model)
		return probeErr
	})
	return sample, err
}

func (s *Service) probeGroupOnce(ctx context.Context, userID, adminAccountID string, group upstream.AdminGroupInfo, cred upstream.ProbeCredential, model string) (GroupProbeSample, error) {
	outcome := s.probeRunner.ProbeGroup(ctx, ProbeRequest{BaseURL: cred.BaseURL, UpstreamKey: cred.Key, ProviderFamily: group.Platform, ModelName: model})
	id, err := newID()
	if err != nil {
		return GroupProbeSample{}, err
	}
	sample := GroupProbeSample{ID: id, TargetID: groupProbeTargetID(adminAccountID, group.ID), ModelName: model, Result: string(outcome.Result), LatencyMs: &outcome.LatencyMs, CreatedAt: time.Now()}
	// Group results do not drive account state, policy budgets or remote actions.
	err = s.repo.InsertEvent(ctx, ConnectionHealthEvent{ID: id, ConnectionID: sample.TargetID, UserID: userID, AdminAccountID: adminAccountID, AdminGroupID: group.ID, OwnGroupName: group.Name, UpstreamGroupName: group.Name, ModelName: model, Result: sample.Result, LatencyMs: sample.LatencyMs, CreatedAt: sample.CreatedAt})
	return sample, err
}
