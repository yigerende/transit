package connection_health

import (
	"context"
	"sort"
	"time"
)

// History remains readable while a channel is paused or its upstream is offline.
// Ownership is enforced by the current workspace and every repository query.
func (s *Service) qualityHistoryWorkspace(ctx context.Context, user, target string) (string, error) {
	workspace, err := s.currentAdminAccountID(ctx, user)
	if err != nil {
		return "", err
	}
	parsed, ok := parseTargetID(target)
	if !ok || parsed.adminAccountID != workspace {
		return "", requestError(ErrorNotFound)
	}
	return workspace, nil
}

func qualitySampleTime(sample QualitySample) time.Time {
	if sample.StartedAt != nil && !sample.StartedAt.IsZero() {
		return *sample.StartedAt
	}
	return sample.CreatedAt
}

func (s *Service) QualityHistory(ctx context.Context, user, target string) ([]QualitySample, error) {
	workspace, err := s.qualityHistoryWorkspace(ctx, user, target)
	if err != nil {
		return nil, err
	}
	samples, err := s.qualityRepo.ListQualityHistory(ctx, user, workspace, []string{target}, 1000)
	if err != nil {
		return nil, err
	}
	if samples == nil {
		samples = []QualitySample{}
	}
	for i := range samples {
		samples[i] = qualitySampleSummary(samples[i])
	}
	sort.Slice(samples, func(i, j int) bool {
		left, right := qualitySampleTime(samples[i]), qualitySampleTime(samples[j])
		if left.Equal(right) {
			return samples[i].ID > samples[j].ID
		}
		return left.After(right)
	})
	return samples, nil
}

func (s *Service) QualityHistoryDetail(ctx context.Context, user, target, id string) (QualitySample, error) {
	workspace, err := s.qualityHistoryWorkspace(ctx, user, target)
	if err != nil {
		return QualitySample{}, err
	}
	sample, err := s.qualityRepo.GetQualitySample(ctx, user, workspace, target, id)
	if err != nil {
		return QualitySample{}, err
	}
	if sample == nil {
		return QualitySample{}, requestError(ErrorNotFound)
	}
	return *sample, nil
}
