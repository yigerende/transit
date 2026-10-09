package connection_health

import (
	"context"
	"strings"
	"time"

	"transithub/backend/internal/modules/upstream"
)

// 手动探活不修改策略状态、不消耗策略预算、不触发远端降级/恢复。
// 调用方可以选择将真实结果写入历史，供渠道色条展示。

// ManualProbeResult 是一次探活单个模型的结果，绝不包含上游凭据。
type ManualProbeResult struct {
	ModelName   string    `json:"modelName"`
	Result      string    `json:"result"`
	Healthy     bool      `json:"healthy"`
	LatencyMs   *int      `json:"latencyMs"`
	ErrorKey    string    `json:"errorKey"`
	ErrorDetail string    `json:"errorDetail"`
	ProbedAt    time.Time `json:"probedAt"`
}

// ManualProbeTarget 对指定 models 逐一发起一次真实轻量探活，直接返回结果，不落任何库。
// models 必须非空（不像旧 ProbeConnection/ProbeTarget 那样把「空」当成「探活全部候选」，
// 手动一次性探活不存在候选池概念，必须由用户在弹窗里显式勾选）。
func (s *Service) ManualProbeTarget(ctx context.Context, userID string, targetID string, models []string) ([]ManualProbeResult, error) {
	return s.manualProbeTarget(ctx, userID, targetID, models, false)
}

func (s *Service) manualProbeTarget(ctx context.Context, userID string, targetID string, models []string, recordHistory bool) ([]ManualProbeResult, error) {
	requested := make([]string, 0, len(models))
	for _, m := range models {
		if trimmed := strings.TrimSpace(m); trimmed != "" {
			requested = append(requested, trimmed)
		}
	}
	if len(requested) == 0 {
		return nil, requestError(ErrorManualModelsRequired)
	}

	session, target, account, adminAccountID, err := s.resolveManualTarget(ctx, userID, targetID)
	if err != nil {
		return nil, err
	}
	cred, err := s.platformGroups.ResolveProbeCredential(session, account)
	if err != nil {
		return nil, requestError(reasonToErrorKey(upstream.ProbeCredentialReason(err)))
	}

	results := make([]ManualProbeResult, 0, len(requested))
	for _, modelName := range requested {
		outcome := s.probeRunner.Probe(ctx, ProbeRequest{
			BaseURL: cred.BaseURL, UpstreamKey: cred.Key, ProviderFamily: target.ProviderFamily,
			ModelName: modelName, MaxTokens: 1,
		})
		result := ManualProbeResult{
			ModelName: modelName, Result: string(outcome.Result), Healthy: outcome.Result == ResultOK,
			LatencyMs: intPtr(outcome.LatencyMs), ProbedAt: time.Now(),
		}
		if outcome.Result != ResultOK {
			result.ErrorKey = string(outcome.Result)
			result.ErrorDetail = outcome.Detail
		}
		results = append(results, result)
		if recordHistory {
			id, err := newID()
			if err != nil {
				return nil, err
			}
			if err := s.repo.InsertEvent(ctx, ConnectionHealthEvent{ID: id, ConnectionID: targetID, UserID: userID, AdminAccountID: adminAccountID, AdminGroupID: target.AdminGroupID, OwnGroupName: target.AdminGroupName, UpstreamGroupName: target.AdminGroupName, ModelName: modelName, Result: result.Result, LatencyMs: result.LatencyMs, CreatedAt: result.ProbedAt}); err != nil {
				return nil, err
			}
		}
	}
	return results, nil
}

func intPtr(v int) *int { return &v }
