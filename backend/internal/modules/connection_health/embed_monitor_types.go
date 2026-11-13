package connection_health

import (
	"strings"
	"time"
)

// These settings belong solely to the embedded group monitor. They never feed
// channel automation, suspension, budgets, or the existing group probe history.
type EmbedCheck struct {
	Enabled         bool   `json:"enabled"`
	IntervalSeconds int    `json:"intervalSeconds"`
	TimeoutSeconds  int    `json:"timeoutSeconds"`
	ProbeMode       string `json:"probeMode,omitempty"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
	ServiceTier     string `json:"serviceTier,omitempty"`
}

type EmbedMonitor struct {
	ID             string     `json:"id"`
	UserID         string     `json:"-"`
	WorkspaceID    string     `json:"-"`
	GroupID        string     `json:"groupId"`
	GroupName      string     `json:"groupName"`
	Name           string     `json:"name"`
	Model          string     `json:"model"`
	QualityBaseURL string     `json:"qualityBaseUrl"`
	Enabled        bool       `json:"enabled"`
	Health         EmbedCheck `json:"health"`
	Candy          EmbedCheck `json:"candy"`
	Pelican        EmbedCheck `json:"pelican"`
}

func (m EmbedMonitor) check(kind string) EmbedCheck {
	switch kind {
	case "health":
		return m.Health
	case "candy":
		return m.Candy
	default:
		return m.Pelican
	}
}

func (m *EmbedMonitor) validate() error {
	m.Name, m.Model, m.GroupID = strings.TrimSpace(m.Name), strings.TrimSpace(m.Model), strings.TrimSpace(m.GroupID)
	m.QualityBaseURL = strings.TrimSpace(m.QualityBaseURL)
	if m.Name == "" || len(m.Name) > 120 || m.Model == "" || len(m.Model) > 200 || m.GroupID == "" || len(m.GroupID) > 120 {
		return requestError("请填写分组、展示名称和模型")
	}
	if m.QualityBaseURL != "" {
		if _, ok := manxueUpstreamBase(m.QualityBaseURL); !ok || len(m.QualityBaseURL) > 2048 {
			return requestError("API 检测地址须为公开 HTTPS 基础地址，不含参数或凭据")
		}
	}
	for _, kind := range []string{"health", "candy", "pelican"} {
		c := m.check(kind)
		if c.IntervalSeconds < 10 || c.IntervalSeconds > 86400 || c.TimeoutSeconds < 5 || c.TimeoutSeconds > 600 {
			return requestError("间隔须为 10–86400 秒，超时须为 5–600 秒")
		}
		if kind == "health" {
			if c.ProbeMode == "" || !validGroupProbeMode(c.ProbeMode) || c.TimeoutSeconds > 300 {
				return requestError("请选择有效的分组探活方式，探活超时不能超过 300 秒")
			}
		} else {
			q := defaultQualitySettings()
			q.DetectionMethod = qualityMethodManxue
			q.ManxueBenchmark = kind
			q.Model = m.Model
			q.ReasoningEffort = c.ReasoningEffort
			q.ManxueServiceTier = c.ServiceTier
			q.TimeoutSeconds = c.TimeoutSeconds
			if err := q.validate(); err != nil {
				return requestError("API 推理强度或服务档位不支持此检测方式")
			}
		}
	}
	return nil
}

type EmbedBoardSettings struct {
	Title             string   `json:"title"`
	Enabled           bool     `json:"enabled"`
	Token             string   `json:"token"`
	VisibleMonitorIDs []string `json:"visibleMonitorIds"`
}

type EmbedJob struct {
	Manual       bool       `json:"-"`
	MonitorID    string     `json:"monitorId"`
	Kind         string     `json:"kind"`
	Requested    bool       `json:"queued"`
	StartedAt    *time.Time `json:"startedAt"`
	RunningUntil *time.Time `json:"runningUntil"`
	NextRunAt    time.Time  `json:"nextRunAt"`
	RunID        string     `json:"-"`
}

type EmbedSample struct {
	QualitySample
	Kind      string `json:"kind"`
	ProbeMode string `json:"probeMode,omitempty"`
}

type EmbedMonitorCard struct {
	Monitor EmbedMonitor  `json:"monitor"`
	Jobs    []EmbedJob    `json:"jobs"`
	History []EmbedSample `json:"history"`
}

type EmbedMonitorBoard struct {
	Title     string             `json:"title"`
	Cards     []EmbedMonitorCard `json:"cards"`
	UpdatedAt time.Time          `json:"updatedAt"`
}
