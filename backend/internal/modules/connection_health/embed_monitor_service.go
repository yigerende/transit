package connection_health

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"transithub/backend/internal/modules/upstream"
)

type EmbedMonitorService struct {
	repo   *embedMonitorRepository
	health *Service
}

func NewEmbedMonitorService(db *pgxpool.Pool, health *Service) *EmbedMonitorService {
	return &EmbedMonitorService{repo: &embedMonitorRepository{db: db}, health: health}
}

func (s *EmbedMonitorService) groups(ctx context.Context, user, workspace string) ([]upstream.AdminGroupInfo, error) {
	session, err := s.health.mySites.RequireSession(ctx, user, workspace)
	if err != nil {
		return nil, requestError("请先登录当前工作区的 Sub2API 管理账号")
	}
	if session.Platform != upstream.PlatformSub2API {
		return nil, requestError("分组降智检测目前仅支持 Sub2API 工作区")
	}
	return s.health.platformGroups.FetchAdminAllGroups(session)
}

func (s *EmbedMonitorService) save(ctx context.Context, user, workspace string, m EmbedMonitor) (EmbedMonitor, error) {
	m.UserID, m.WorkspaceID = user, workspace
	if err := m.validate(); err != nil {
		return m, err
	}
	groups, err := s.groups(ctx, user, workspace)
	if err != nil {
		return m, err
	}
	found := false
	for _, group := range groups {
		if group.ID == m.GroupID {
			m.GroupName = group.Name
			found = true
			break
		}
	}
	if !found {
		return m, requestError("所选分组不存在或无权访问")
	}
	if m.ID == "" {
		m.ID, err = newID()
		if err != nil {
			return m, err
		}
	} else {
		if _, err = s.repo.monitor(ctx, user, workspace, m.ID); err != nil {
			return m, err
		}
	}
	err = s.repo.save(ctx, m)
	return m, err
}

// Uses a dedicated group-bound key name, separate from the original group
// health settings. Neither the credential nor the upstream key id is persisted.
func (s *EmbedMonitorService) credential(ctx context.Context, m EmbedMonitor) (upstream.ProbeCredential, string, error) {
	release, err := s.health.repo.AcquireTargetLease(ctx, "embed-group-key:"+m.UserID+":"+m.WorkspaceID+":"+m.GroupID)
	if err != nil {
		return upstream.ProbeCredential{}, "", err
	}
	defer release()
	session, err := s.health.mySites.RequireSession(ctx, m.UserID, m.WorkspaceID)
	if err != nil || session.Platform != upstream.PlatformSub2API {
		return upstream.ProbeCredential{}, "", requestError("credential_unavailable")
	}
	groups, err := s.health.platformGroups.FetchAdminAllGroups(session)
	if err != nil {
		return upstream.ProbeCredential{}, "", requestError("credential_unavailable")
	}
	var group *upstream.AdminGroupInfo
	for i := range groups {
		if groups[i].ID == m.GroupID {
			group = &groups[i]
			break
		}
	}
	if group == nil {
		return upstream.ProbeCredential{}, "", requestError("credential_unavailable")
	}
	provider, ok := s.health.platformGroups.(groupProbeKeyProvider)
	if !ok {
		return upstream.ProbeCredential{}, "", requestError("credential_unavailable")
	}
	digest := sha256.Sum256([]byte(m.UserID + "|" + m.WorkspaceID + "|" + m.GroupID))
	cred, err := provider.EnsureSub2APIGroupProbeKey(session, m.GroupID, fmt.Sprintf("embed-group-%s-%x", m.GroupID, digest[:6]))
	if err != nil {
		return upstream.ProbeCredential{}, "", requestError("credential_unavailable")
	}
	cred.BaseURL = session.BaseURL
	return cred, group.Platform, nil
}

func (s *EmbedMonitorService) Start(ctx context.Context) {
	// Separate bounded pools keep long artwork tasks from starving health probes.
	// Each kind has two workers serving manual and scheduled jobs alike.
	for i := 0; i < 6; i++ {
		kind := []string{"health", "candy", "pelican"}[i/2]
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
				m, j, err := s.repo.claim(ctx, kind)
				if err != nil {
					if !errors.Is(err, pgx.ErrNoRows) && ctx.Err() == nil {
						log.Printf("[embed-monitor] claim failed: %v", err)
					}
					continue
				}
				s.run(ctx, m, j)
			}
		}()
	}
}

func (s *EmbedMonitorService) run(parent context.Context, m EmbedMonitor, j EmbedJob) {
	ctx, cancel := context.WithTimeout(parent, 650*time.Second)
	defer cancel()
	started := time.Now()
	if j.StartedAt != nil {
		started = *j.StartedAt
	}
	sample := EmbedSample{Kind: j.Kind, QualitySample: QualitySample{ID: j.RunID, TargetID: m.ID, Model: m.Model, StartedAt: &started, Manual: j.Manual, Result: "error", ErrorKey: "credential_unavailable", QuestionName: j.Kind}}
	if j.Kind == "health" {
		sample.ProbeMode = m.Health.ProbeMode
	}
	if j.Kind == "pelican" {
		sample.Model = manxuePelicanModel
	}
	if j.Kind != "health" {
		sample.DetectionMethod = qualityMethodManxue
		sample.Benchmark = j.Kind
	}
	defer func() {
		if recover() != nil {
			sample.Result = "error"
			sample.ErrorKey = "invalid_response"
			log.Printf("[embed-monitor] worker panic recovered")
		}
		sample.CreatedAt = time.Now()
		if sample.DurationMS == 0 {
			sample.DurationMS = int(sample.CreatedAt.Sub(started).Milliseconds())
		}
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer finishCancel()
		if err := s.repo.complete(finishCtx, m, j, sample); err != nil {
			log.Printf("[embed-monitor] completion failed: %v", err)
		}
	}()
	cred, provider, err := s.credential(ctx, m)
	if err != nil {
		return
	}
	c := m.check(j.Kind)
	if j.Kind == "health" {
		sample.ProbeMode = c.ProbeMode
		out := s.health.probeRunner.ProbeGroup(ctx, ProbeRequest{ProbeMode: c.ProbeMode, BaseURL: cred.BaseURL, UpstreamKey: cred.Key, ProviderFamily: provider, ModelName: m.Model, MaxLatencyMs: c.TimeoutSeconds * 1000})
		sample.DurationMS = out.LatencyMs
		sample.ErrorKey = string(out.Result)
		if out.Result == ResultOK {
			sample.Result = "passed"
			sample.ErrorKey = ""
			sample.ContentPassed = true
		}
		return
	}
	if m.QualityBaseURL != "" {
		cred.BaseURL = m.QualityBaseURL
	}
	q := defaultQualitySettings()
	q.DetectionMethod = qualityMethodManxue
	q.ManxueBenchmark = j.Kind
	q.Model = m.Model
	q.ReasoningEffort = c.ReasoningEffort
	q.ManxueServiceTier = c.ServiceTier
	q.TimeoutSeconds = c.TimeoutSeconds
	out := s.health.qualityRunner.ProbeQuality(ctx, cred, provider, q, QualityQuestion{})
	sample.ErrorKey = out.ErrorKey
	sample.ErrorDetail = out.ErrorDetail
	sample.Phase = out.Phase
	sample.InputTokens = out.InputTokens
	sample.OutputTokens = out.OutputTokens
	sample.UsageReported = out.UsageReported
	sample.ServiceTier = out.ServiceTier
	sample.DurationMS = out.DurationMS
	sample.Answer = out.Answer
	sample.Report = out.Report
	sample.HTML = out.HTML
	sample.HasHTML = out.HTML != ""
	sample.HTMLTooLarge = out.HTMLTooLarge
	if out.ErrorKey == "" && (out.Verdict == "passed" || out.Verdict == "failed") {
		sample.Result = out.Verdict
		sample.ContentPassed = out.Verdict == "passed"
	} else if sample.ErrorKey == "" {
		sample.ErrorKey = "invalid_response"
	}
}

func (s *EmbedMonitorService) saveSettings(ctx context.Context, user, workspace string, in EmbedBoardSettings, rotate bool) (EmbedBoardSettings, error) {
	current, err := s.repo.settings(ctx, user, workspace)
	if err != nil {
		return current, err
	}
	if rotate {
		current.Token, err = embedToken()
		if err != nil {
			return current, err
		}
	} else {
		if in.HistoryLimit != nil && (*in.HistoryLimit < 1 || *in.HistoryLimit > 1000) {
			return current, requestError("每类历史保留条数须为 1–1000 的整数")
		}
		current.Title = strings.TrimSpace(in.Title)
		current.Enabled = in.Enabled
		if current.Title == "" || len(current.Title) > 120 {
			return current, requestError("展示标题须为 1–120 字符")
		}
	}
	// Changing display settings must not restore an old token when a concurrent
	// request rotates it. Likewise, rotation preserves the current publish flag.
	if rotate {
		err = s.repo.db.QueryRow(ctx, `UPDATE embed_group_monitor_boards SET token=$3 WHERE user_id=$1 AND admin_account_id=$2 RETURNING title,enabled,token`, user, workspace, current.Token).Scan(&current.Title, &current.Enabled, &current.Token)
	} else {
		tx, beginErr := s.repo.db.Begin(ctx)
		if beginErr != nil {
			return current, beginErr
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "embed-monitor:"+user+":"+workspace); err != nil {
			return current, err
		}
		// An omitted selection from an older client preserves visibility; [] hides all.
		if in.VisibleMonitorIDs != nil {
			if len(in.VisibleMonitorIDs) > 50 {
				return current, requestError("最多选择 50 个展示检测项")
			}
			ids := make([]string, 0, len(in.VisibleMonitorIDs))
			seen := map[string]bool{}
			for _, id := range in.VisibleMonitorIDs {
				if id == "" || len(id) > 120 {
					return current, requestError("请选择有效的检测项")
				}
				if !seen[id] {
					ids = append(ids, id)
					seen[id] = true
				}
			}
			var count int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM embed_group_monitors WHERE user_id=$1 AND admin_account_id=$2 AND id=ANY($3::text[])`, user, workspace, ids).Scan(&count); err != nil {
				return current, err
			}
			if count != len(ids) {
				return current, requestError("部分检测项已删除或不属于当前工作区，请重新打开嵌入设置")
			}
			if _, err = tx.Exec(ctx, `UPDATE embed_group_monitors SET published=(id=ANY($3::text[])) WHERE user_id=$1 AND admin_account_id=$2`, user, workspace, ids); err != nil {
				return current, err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE embed_group_monitor_boards SET title=$3,enabled=$4,history_limit=COALESCE($5::integer,history_limit) WHERE user_id=$1 AND admin_account_id=$2`, user, workspace, current.Title, current.Enabled, in.HistoryLimit); err != nil {
			return current, err
		}
		if in.HistoryLimit != nil {
			// Prune the whole workspace immediately, including disabled/private monitors.
			_, err = tx.Exec(ctx, `DELETE FROM embed_group_monitor_samples WHERE id IN (
			 SELECT id FROM (SELECT s.id,row_number() OVER (PARTITION BY s.monitor_id,s.kind ORDER BY s.started_at DESC,s.id DESC) AS position
			 FROM embed_group_monitor_samples s JOIN embed_group_monitors m ON m.id=s.monitor_id WHERE m.user_id=$1 AND m.admin_account_id=$2) ranked WHERE position>$3)`, user, workspace, *in.HistoryLimit)
			if err != nil {
				return current, err
			}
		}
		err = tx.Commit(ctx)
	}
	if err != nil {
		return current, err
	}
	return s.repo.settings(ctx, user, workspace)
}
