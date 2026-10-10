package connection_health

import (
	"context"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"transithub/backend/internal/modules/upstream"
)

// https://manxue.ai/api documents an asynchronous API, not a model endpoint.
// The origin is fixed so channel credentials cannot be redirected elsewhere.
const manxueTestsURL = "https://manxue.ai/api/v1/tests"

var manxueTaskID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type manxueProbeRunner struct {
	client       *http.Client
	pollInterval time.Duration
}

type manxueTask struct {
	ID        string `json:"id"`
	Benchmark string `json:"benchmark"`
	Status    string `json:"status"`
	Candy     *struct {
		Status     string `json:"status"`
		Answer     string `json:"answer"`
		DurationMS int    `json:"duration_ms"`
	} `json:"candy"`
	Assessment *struct {
		Quality string `json:"quality"`
		Reason  string `json:"reason"`
	} `json:"assessment"`
	Result *struct {
		DurationMS int `json:"duration_ms"`
	} `json:"result"`
}

func manxueUpstreamBase(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return "", false
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast()) {
		return "", false
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" {
		u.Path = "/v1"
	}
	u.RawPath = strings.TrimRight(u.RawPath, "/")
	return u.String(), true
}

func manxueWait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func manxueRetryDelay(header string, attempt int, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 && seconds <= 86400 {
		return max(2*time.Second, time.Duration(seconds)*time.Second)
	}
	if date, err := http.ParseTime(header); err == nil && date.After(now) {
		return max(2*time.Second, date.Sub(now))
	}
	return time.Duration(min(30, 2<<min(attempt, 4)))*time.Second + time.Duration(rand.IntN(500))*time.Millisecond
}

// All errors are local keys. Never persist an upstream body, request key or task
// id (the latter grants access to a private report on the public API).
func (r *manxueProbeRunner) request(ctx context.Context, method, endpoint string, body any, idempotency string) (manxueTask, int, string, string) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	headers := map[string]string{}
	if idempotency != "" {
		headers["Idempotency-Key"] = idempotency
	}
	req, err := newJSONRequest(ctx, method, endpoint, body, headers)
	if method != http.MethodPost {
		req, err = http.NewRequestWithContext(ctx, method, endpoint, nil)
	}
	if err != nil {
		return manxueTask{}, 0, "", "invalid_response"
	}
	client := *r.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return manxueTask{}, 0, "", string(classifyTransportError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return manxueTask{}, resp.StatusCode, resp.Header.Get("Retry-After"), "rate_limited"
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		key := "invalid_response"
		if resp.StatusCode >= 500 {
			key = "server_error"
		}
		if resp.StatusCode == 404 {
			key = qualityPrefix + "manxueExpired"
		}
		return manxueTask{}, resp.StatusCode, "", key
	}
	if method == http.MethodDelete {
		return manxueTask{}, resp.StatusCode, "", ""
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil {
		return manxueTask{}, resp.StatusCode, "", string(classifyTransportError(err))
	}
	var task manxueTask
	if len(raw) > 4*1024*1024 || json.Unmarshal(raw, &task) != nil {
		return task, resp.StatusCode, "", "invalid_response"
	}
	return task, resp.StatusCode, "", ""
}

func (r *manxueProbeRunner) ProbeQuality(ctx context.Context, cred upstream.ProbeCredential, provider string, q QualitySettings, _ QualityQuestion) (out qualityProbeResult) {
	q.normalizeMethod()
	if provider != "" && provider != ProviderOpenAI && provider != ProviderCustom {
		return qualityProbeResult{ErrorKey: qualityPrefix + "manxueUnsupported"}
	}
	base, ok := manxueUpstreamBase(cred.BaseURL)
	if !ok {
		return qualityProbeResult{ErrorKey: qualityPrefix + "manxuePublicURL"}
	}
	if strings.TrimSpace(cred.Key) == "" {
		return qualityProbeResult{ErrorKey: "credential_unavailable"}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(q.TimeoutSeconds)*time.Second)
	defer cancel()
	started := time.Now()
	defer func() {
		if out.DurationMS == 0 {
			out.DurationMS = int(time.Since(started).Milliseconds())
		}
	}()
	idempotency, err := newID()
	if err != nil {
		return qualityProbeResult{ErrorKey: "invalid_response"}
	}
	payload := map[string]any{"benchmark": q.ManxueBenchmark, "base_url": base, "api_key": cred.Key, "model": q.Model, "protocol": q.ManxueProtocol}
	if q.ReasoningEffort != "" {
		payload["reasoning_effort"] = q.ReasoningEffort
	}
	if q.ManxueServiceTier != "" {
		payload["service_tier"] = q.ManxueServiceTier
	}
	var task manxueTask
	for attempt := 0; ; attempt++ {
		var status int
		var retry, errorKey string
		task, status, retry, errorKey = r.request(ctx, http.MethodPost, manxueTestsURL, payload, idempotency)
		if (status == 429 || status == 503) && attempt < 2 && manxueWait(ctx, manxueRetryDelay(retry, attempt, time.Now())) {
			continue
		}
		if errorKey != "" {
			return qualityProbeResult{ErrorKey: errorKey}
		}
		if status != http.StatusAccepted || !manxueTaskID.MatchString(task.ID) {
			return qualityProbeResult{ErrorKey: "invalid_response"}
		}
		break
	}
	endpoint := manxueTestsURL + "/" + task.ID
	finished := false
	defer func() {
		if !finished {
			// Cancel only the task created by this run when timing out or stopping.
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, _, _, _ = r.request(cleanup, http.MethodDelete, endpoint, nil, "")
		}
	}()
	interval := r.pollInterval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	backoff := 0
	for {
		if task.Benchmark != q.ManxueBenchmark {
			return qualityProbeResult{ErrorKey: "invalid_response"}
		}
		switch task.Status {
		case "succeeded":
			finished = true
			return manxueVerdict(task, q.ManxueBenchmark, cred.Key)
		case "failed", "cancelled":
			finished = true
			return qualityProbeResult{ErrorKey: qualityPrefix + "manxueFailed"}
		case "queued", "pending", "running":
		default:
			return qualityProbeResult{ErrorKey: "invalid_response"}
		}
		if !manxueWait(ctx, interval) {
			return qualityProbeResult{ErrorKey: qualityPrefix + "manxueTimeout"}
		}
		var status int
		var retry, errorKey string
		next, status, retry, errorKey := r.request(ctx, http.MethodGet, endpoint, nil, "")
		if status == 429 || status == 503 {
			interval = manxueRetryDelay(retry, backoff, time.Now())
			backoff++
			continue
		}
		if errorKey != "" {
			if status == 404 {
				finished = true
			}
			if ctx.Err() != nil {
				errorKey = qualityPrefix + "manxueTimeout"
			}
			return qualityProbeResult{ErrorKey: errorKey}
		}
		if status != http.StatusOK || next.ID != task.ID {
			return qualityProbeResult{ErrorKey: "invalid_response"}
		}
		task = next
		interval, backoff = r.pollInterval, 0
		if interval <= 0 {
			interval = 3 * time.Second
		}
	}
}

func manxueVerdict(task manxueTask, benchmark, key string) qualityProbeResult {
	out := qualityProbeResult{}
	if benchmark == "candy" && task.Candy != nil {
		out.DurationMS = task.Candy.DurationMS
		out.Answer = truncate(redact(task.Candy.Answer, key), 4000)
		switch task.Candy.Status {
		case "passed":
			out.Verdict = "passed"
		case "incorrect":
			out.Verdict = "failed"
		case "error":
			out.ErrorKey = qualityPrefix + "manxueFailed"
		default:
			out.ErrorKey = qualityPrefix + "manxueUnknown"
		}
	} else if benchmark == "pelican" && task.Assessment != nil {
		if task.Result != nil {
			out.DurationMS = task.Result.DurationMS
		}
		out.Report = truncate(redact(task.Assessment.Reason, key), 2000)
		switch task.Assessment.Quality {
		case "normal":
			out.Verdict = "passed"
		case "degraded", "suspicious":
			out.Verdict = "failed"
		default:
			out.ErrorKey = qualityPrefix + "manxueUnknown"
		}
	} else {
		out.ErrorKey = qualityPrefix + "manxueUnknown"
	}
	if out.DurationMS < 0 {
		return qualityProbeResult{ErrorKey: "invalid_response"}
	}
	return out
}
