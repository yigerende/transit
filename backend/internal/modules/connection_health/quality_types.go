package connection_health

import (
	"regexp"
	"strings"
	"time"
)

const qualityPrefix = "admin.connectionHealth.quality."
const qualityMethodQuestions = "questions"
const qualityMethodManxue = "manxue"

type QualityQuestion struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Prompt        string `json:"prompt"`
	Answer        string `json:"answer"`
	MatchMode     string `json:"matchMode"`
	MaxDurationMS int    `json:"maxDurationMs"`
}

// Global within one connected workspace, never shared between users or sites.
// Deliberately contains no remote-action, weight or account-disable settings.
type QualitySettings struct {
	DetectionMethod   string            `json:"detectionMethod"`
	ManxueBenchmark   string            `json:"manxueBenchmark"`
	ManxueProtocol    string            `json:"manxueProtocol"`
	ManxueServiceTier string            `json:"manxueServiceTier"`
	Enabled           bool              `json:"enabled"`
	Revision          string            `json:"revision"`
	Model             string            `json:"model"`
	ReasoningEffort   string            `json:"reasoningEffort"`
	Mode              string            `json:"mode"`
	IntervalSeconds   int               `json:"intervalSeconds"`
	RetrySeconds      int               `json:"retrySeconds"`
	FailureLimit      int               `json:"failureLimit"`
	RecoveryLimit     int               `json:"recoveryLimit"`
	Concurrency       int               `json:"concurrency"`
	TimeoutSeconds    int               `json:"timeoutSeconds"`
	MaxTokens         int               `json:"maxTokens"`
	HistoryLimit      int               `json:"historyLimit"`
	Questions         []QualityQuestion `json:"questions"`
}

func defaultQualitySettings() QualitySettings {
	return QualitySettings{DetectionMethod: qualityMethodQuestions, ManxueBenchmark: "candy", ManxueProtocol: "responses", Model: "gpt-6-astra", ReasoningEffort: "xhigh", Mode: "content_time", IntervalSeconds: 300, RetrySeconds: 60, FailureLimit: 2, RecoveryLimit: 2, Concurrency: 4, TimeoutSeconds: 180, MaxTokens: 8192, HistoryLimit: 100, Questions: []QualityQuestion{
		{ID: "clock", Name: "时钟夹角", Enabled: true, Prompt: "连续走动的指针式时钟在3点15分时，时针与分针较小夹角是多少度？只回复数字，不带单位。", Answer: "7.5", MatchMode: "answer", MaxDurationMS: 20000},
		{ID: "percent", Name: "百分比变化", Enabled: true, Prompt: "一个数先增加20%，再在增加后的数值基础上减少20%，最终是原数的百分之多少？只回复数字，不带百分号。", Answer: "96", MatchMode: "answer", MaxDurationMS: 20000},
	}}
}

// Defaults for configurations saved before API detection was available.
func (q *QualitySettings) normalizeMethod() {
	if q.DetectionMethod == "" {
		q.DetectionMethod = qualityMethodQuestions
	}
	if q.ManxueBenchmark == "" {
		q.ManxueBenchmark = "candy"
	}
	if q.ManxueProtocol == "" {
		q.ManxueProtocol = "responses"
	}
}

func (q QualitySettings) activeQuestions() []QualityQuestion {
	if q.DetectionMethod == qualityMethodManxue {
		return []QualityQuestion{{ID: "manxue-" + q.ManxueBenchmark, Name: "Manxue AI · " + q.ManxueBenchmark, Enabled: true, MatchMode: "external"}}
	}
	out := []QualityQuestion{}
	for _, question := range q.Questions {
		if question.Enabled {
			out = append(out, question)
		}
	}
	return out
}

func (q *QualitySettings) validate() error {
	q.normalizeMethod()
	if q.DetectionMethod != qualityMethodQuestions && q.DetectionMethod != qualityMethodManxue {
		return requestError(qualityPrefix + "invalidConfig")
	}
	if q.ManxueBenchmark != "candy" && q.ManxueBenchmark != "pelican" || q.ManxueProtocol != "responses" && q.ManxueProtocol != "chat_completions" || q.ManxueServiceTier != "" && q.ManxueServiceTier != "priority" && q.ManxueServiceTier != "ultrafast" {
		return requestError(qualityPrefix + "manxueInvalidConfig")
	}
	q.Model = strings.TrimSpace(q.Model)
	if q.Model == "" || len(q.Model) > 200 {
		return requestError(qualityPrefix + "invalidModel")
	}
	if q.Mode != "content" && q.Mode != "time" && q.Mode != "content_time" {
		return requestError(qualityPrefix + "invalidConfig")
	}
	if q.ReasoningEffort != "" && q.ReasoningEffort != "low" && q.ReasoningEffort != "medium" && q.ReasoningEffort != "high" && q.ReasoningEffort != "xhigh" && !(q.DetectionMethod == qualityMethodManxue && q.ManxueBenchmark == "candy" && (q.ReasoningEffort == "max" || q.ReasoningEffort == "ultra")) {
		return requestError(qualityPrefix + "invalidConfig")
	}
	maxTimeout := 300
	if q.DetectionMethod == qualityMethodManxue {
		maxTimeout = 600
		if q.ManxueBenchmark == "candy" && q.ManxueProtocol != "responses" || q.ManxueBenchmark == "pelican" && (q.ReasoningEffort == "xhigh" || q.ReasoningEffort == "max" || q.ReasoningEffort == "ultra") {
			return requestError(qualityPrefix + "manxueInvalidConfig")
		}
	}
	if q.IntervalSeconds < 10 || q.IntervalSeconds > 86400 || q.RetrySeconds < 10 || q.RetrySeconds > 86400 || q.FailureLimit < 1 || q.FailureLimit > 20 || q.RecoveryLimit < 1 || q.RecoveryLimit > 20 || q.Concurrency < 1 || q.Concurrency > 32 || q.TimeoutSeconds < 5 || q.TimeoutSeconds > maxTimeout || q.HistoryLimit < 1 || q.HistoryLimit > 1000 || q.MaxTokens < 128 || q.MaxTokens > 32768 {
		return requestError(qualityPrefix + "invalidConfig")
	}
	if len(q.Questions) > 50 || (q.Enabled && len(q.activeQuestions()) == 0) {
		return requestError(qualityPrefix + "questionsRequired")
	}
	if q.DetectionMethod == qualityMethodManxue {
		return nil
	}
	seen := map[string]bool{}
	for i := range q.Questions {
		v := &q.Questions[i]
		v.Name = strings.TrimSpace(v.Name)
		if v.ID == "" || len(v.ID) > 80 || seen[v.ID] || v.Name == "" || len(v.Name) > 200 || strings.TrimSpace(v.Prompt) == "" || len(v.Prompt) > 15000 || len(v.Answer) > 2000 || (q.Mode != "time" && strings.TrimSpace(v.Answer) == "") || v.MaxDurationMS < 1 || v.MaxDurationMS > 300000 {
			return requestError(qualityPrefix + "invalidQuestion")
		}
		seen[v.ID] = true
		switch v.MatchMode {
		case "answer", "keyword":
		case "regex":
			if _, err := regexp.Compile(v.Answer); err != nil {
				return requestError(qualityPrefix + "invalidRegex")
			}
		default:
			return requestError(qualityPrefix + "invalidQuestion")
		}
	}
	return nil
}

type QualityScope struct {
	UserID, WorkspaceID string
	Settings            QualitySettings
}
type QualityGroup struct {
	GroupID       string `json:"groupId"`
	Enabled       bool   `json:"enabled"`
	GlobalEnabled bool   `json:"globalEnabled"`
	ErrorKey      string `json:"errorKey,omitempty"`
}

// Channel preference is shared across all groups in the connected workspace.
type QualityChannel struct {
	TargetID string `json:"targetId"`
	Enabled  bool   `json:"enabled"`
}

type QualitySample struct {
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	Prompt          string     `json:"prompt,omitempty"`
	Mode            string     `json:"mode,omitempty"`
	HTML            string     `json:"html,omitempty"`
	HasHTML         bool       `json:"hasHtml,omitempty"`
	HTMLTooLarge    bool       `json:"htmlTooLarge,omitempty"`
	DetectionMethod string     `json:"detectionMethod,omitempty"`
	Benchmark       string     `json:"benchmark,omitempty"`
	Report          string     `json:"report,omitempty"`
	ID              string     `json:"id"`
	TargetID        string     `json:"targetId"`
	Model           string     `json:"model"`
	QuestionID      string     `json:"questionId"`
	QuestionName    string     `json:"questionName"`
	Answer          string     `json:"answer"`
	ExpectedAnswer  string     `json:"expectedAnswer"`
	MatchMode       string     `json:"matchMode"`
	Result          string     `json:"result"` // passed, failed (valid answer), error (no verdict)
	ErrorKey        string     `json:"errorKey,omitempty"`
	ContentPassed   bool       `json:"contentPassed"`
	TimePassed      bool       `json:"timePassed"`
	DurationMS      int        `json:"durationMs"`
	MaxDurationMS   int        `json:"maxDurationMs"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// Group cards and history lists stay small; full evidence is fetched on demand.
func qualitySampleSummary(sample QualitySample) QualitySample {
	sample.HasHTML = sample.HasHTML || sample.HTML != ""
	sample.HTML = ""
	sample.Prompt = ""
	sample.Answer = truncate(sample.Answer, 4000)
	return sample
}

type QualityState struct {
	TargetID       string        `json:"targetId"`
	Revision       string        `json:"revision"`
	Status         string        `json:"status"`
	Degraded       bool          `json:"degraded"`
	Failures       int           `json:"failures"`
	Successes      int           `json:"successes"`
	NextQuestionID string        `json:"nextQuestionId"`
	NextProbeAt    time.Time     `json:"nextProbeAt"`
	Latest         QualitySample `json:"latest"`
}

var qualityFinalAnswerPattern = regexp.MustCompile(`(?m)^\s*FINAL_ANSWER\s*=\s*([^\r\n]+)\s*$`)

func qualityAnswerMatches(question QualityQuestion, answer string) bool {
	switch question.MatchMode {
	case "answer":
		matches := qualityFinalAnswerPattern.FindAllStringSubmatch(strings.TrimSpace(answer), -1)
		return len(matches) == 1 && strings.TrimSpace(matches[0][1]) == strings.TrimSpace(question.Answer) || len(matches) == 0 && strings.TrimSpace(answer) == strings.TrimSpace(question.Answer)
	case "keyword":
		return strings.Contains(answer, question.Answer)
	case "regex":
		re, err := regexp.Compile(question.Answer)
		return err == nil && re.MatchString(answer)
	}
	return false
}

func applyQualitySample(state QualityState, config QualitySettings, question QualityQuestion, sample QualitySample) QualityState {
	state.TargetID = sample.TargetID
	state.Revision = config.Revision
	interval := config.IntervalSeconds
	if config.DetectionMethod == qualityMethodManxue && sample.ErrorKey == "" && sample.Result != "passed" && sample.Result != "failed" {
		sample.ErrorKey = qualityPrefix + "manxueUnknown"
	}
	if sample.ErrorKey != "" {
		sample.Result = "error"
		state.Status = "error"
		interval = config.RetrySeconds
		// Request failures do not count as wrong answers, rotate questions or erase evidence.
	} else {
		sample.ContentPassed = qualityAnswerMatches(question, sample.Answer)
		sample.TimePassed = sample.DurationMS < question.MaxDurationMS
		passed := sample.ContentPassed
		if config.Mode == "time" {
			passed = sample.TimePassed
		} else if config.Mode == "content_time" {
			passed = passed && sample.TimePassed
		}
		if config.DetectionMethod == qualityMethodManxue {
			// The remote benchmark supplies the verdict. Local question thresholds
			// cannot turn an API failure into a pass, or a slow valid result into a failure.
			passed = sample.Result == "passed"
			sample.ContentPassed = passed
			sample.TimePassed = false
		}
		if passed {
			sample.Result = "passed"
			state.Failures = 0
			state.Successes++
			if state.Successes >= config.RecoveryLimit {
				state.Degraded = false
			}
			state.Status = "normal"
			if state.Degraded {
				state.Status = "recovering"
			}
		} else {
			sample.Result = "failed"
			state.Successes = 0
			state.Failures++
			interval = config.RetrySeconds
			if state.Failures >= config.FailureLimit {
				state.Degraded = true
			}
			state.Status = "suspect"
			if state.Degraded {
				state.Status = "degraded"
			}
		}
		active := config.activeQuestions()
		for i, v := range active {
			if v.ID == question.ID {
				state.NextQuestionID = active[(i+1)%len(active)].ID
				break
			}
		}
	}
	state.Latest = sample
	state.NextProbeAt = sample.CreatedAt.Add(time.Duration(interval) * time.Second)
	return state
}
