package connection_health

import (
	"context"
	"errors"
	"log"
	"time"
)

// Keep group tasks independent of channel policy assignments and their 30s scan.
// A 1s scan supports the configured seconds without rounding to channel ticks.
func (s *Service) StartGroupProbeScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		done := make(chan GroupProbeConfig, globalProbeConcurrency)
		active := map[string]bool{}
		workspaceActive := map[string]int{}
		startDue := func() {
			defer func() {
				if recover() != nil {
					log.Printf("[group-probe] scheduler scan panic recovered")
				}
			}()
			if len(active) >= globalProbeConcurrency {
				return
			}
			configs, err := s.repo.ListDueGroupProbeConfigs(ctx, time.Now())
			if err != nil {
				log.Printf("[group-probe] list due tasks failed: %v", err)
				return
			}
			for _, c := range configs {
				key := c.UserID + "|" + groupProbeTargetID(c.AdminAccountID, c.GroupID)
				workspace := c.UserID + "|" + c.AdminAccountID
				if active[key] || workspaceActive[workspace] >= perSiteProbeConcurrency {
					continue
				}
				if len(active) >= globalProbeConcurrency {
					break
				}
				active[key] = true
				workspaceActive[workspace]++
				go func(c GroupProbeConfig) {
					defer func() { done <- c }()
					s.runScheduledGroupProbe(ctx, c)
				}(c)
			}
		}
		startDue()
		for {
			select {
			case <-ctx.Done():
				return
			case c := <-done:
				delete(active, c.UserID+"|"+groupProbeTargetID(c.AdminAccountID, c.GroupID))
				workspaceActive[c.UserID+"|"+c.AdminAccountID]--
			case <-ticker.C:
				startDue()
			}
		}
	}()
}

func (s *Service) runScheduledGroupProbe(ctx context.Context, candidate GroupProbeConfig) {
	defer func() {
		if recover() != nil {
			log.Printf("[group-probe] task panic recovered")
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	release, err := s.repo.AcquireTargetLease(ctx, groupProbeTargetID(candidate.AdminAccountID, candidate.GroupID))
	if err != nil {
		return
	}
	defer release()
	// Re-read under the cross-process lease: another worker may already have
	// completed this due task, or the user may have paused/edited it meanwhile.
	c, err := s.repo.GetGroupProbeConfig(ctx, candidate.UserID, candidate.AdminAccountID, candidate.GroupID)
	if err != nil || c == nil || !c.Enabled || c.NextProbeAt == nil || c.NextProbeAt.After(time.Now()) {
		return
	}
	errorKey := ErrorRequest
	defer func() {
		// Even credential errors back off by the chosen interval. A failure is
		// never a reason to hammer the upstream key endpoint every scheduler tick.
		finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer finishCancel()
		if err := s.repo.CompleteGroupProbe(finishCtx, *c, time.Now(), errorKey); err != nil {
			log.Printf("[group-probe] save task result failed: %v", err)
		}
	}()
	group, cred, _, err := s.resolveGroupProbeCredential(ctx, c.UserID, c.AdminAccountID, c.GroupID, nil, false)
	if err != nil {
		var requestErr requestError
		if errors.As(err, &requestErr) {
			errorKey = err.Error()
		}
		return
	}
	sample, err := s.probeGroupOnce(ctx, c.UserID, c.AdminAccountID, group, cred, c.Model, c.ProbeMode, false)
	if err != nil {
		return
	}
	errorKey = ""
	if sample.Result != string(ResultOK) {
		errorKey = sample.Result
	}
}
