package connection_health

import (
	"context"
	"strings"
	"time"
)

// ProbeChannelQuality bypasses the interval with the selected one-off detector,
// retaining health suspension and result history. Automatic opt-ins do not
// restrict an explicit manual check. It has no remote actions.
func (s *Service) ProbeChannelQuality(ctx context.Context, user, targetID, groupID, method string) (QualitySample, error) {
	if strings.TrimSpace(groupID) == "" {
		return QualitySample{}, requestError(ErrorRequest)
	}
	session, target, account, workspace, err := s.resolveManualTarget(ctx, user, targetID)
	if err != nil {
		return QualitySample{}, err
	}
	q, err := s.qualityRepo.GetQualitySettings(ctx, user, workspace)
	if err != nil {
		return QualitySample{}, err
	}
	if q.Revision == "" {
		return QualitySample{}, requestError(qualityPrefix + "configureManualFirst")
	}
	// Validate the chosen detector even when automatic detection is disabled.
	q.Enabled = true
	// One-off detector selection never changes the saved automatic configuration.
	switch method {
	case "questions":
		q.DetectionMethod = qualityMethodQuestions
		q.TimeoutSeconds = min(q.TimeoutSeconds, 300)
		if q.ReasoningEffort == "max" || q.ReasoningEffort == "ultra" {
			q.ReasoningEffort = "high"
		}
	case "manxue_candy", "manxue_pelican":
		q.DetectionMethod = qualityMethodManxue
		q.ManxueBenchmark = strings.TrimPrefix(method, "manxue_")
		// Keep one-off requests within the API's supported options for each benchmark.
		if q.ManxueBenchmark == "candy" {
			q.ManxueProtocol = "responses"
		} else if q.ReasoningEffort == "xhigh" || q.ReasoningEffort == "max" || q.ReasoningEffort == "ultra" {
			q.ReasoningEffort = "high"
		}
	default:
		return QualitySample{}, requestError(qualityPrefix + "invalidConfig")
	}
	if err := q.validate(); err != nil {
		return QualitySample{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(q.TimeoutSeconds+60)*time.Second)
	defer cancel()
	return s.runQualityCandidate(ctx, QualityScope{UserID: user, WorkspaceID: workspace}, session, q,
		qualityCandidate{account: account, groups: []string{groupID}, targetID: target.TargetID}, true)
}
