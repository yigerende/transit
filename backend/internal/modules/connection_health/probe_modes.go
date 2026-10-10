package connection_health

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"transithub/backend/internal/modules/upstream"
)

const (
	ProbeModeLight      = "real_model"
	ProbeModeArithmetic = "arithmetic"
	ProbeModeSub2API    = "sub2api_test"
	ProbeModeFirstToken = "first_token"
)

const probeModeHistorySchema = `ALTER TABLE IF EXISTS connection_health_events
    ADD COLUMN IF NOT EXISTS probe_mode text NOT NULL DEFAULT 'real_model';
`

func normalizeProbeMode(mode string) string {
	if mode = strings.TrimSpace(mode); mode == "" {
		return ProbeModeLight
	}
	return mode
}

func probeSpecsNeedCredentials(specs []probeModelSpec) bool {
	for _, spec := range specs {
		if normalizeProbeMode(spec.policy.ProbeMode) != ProbeModeSub2API {
			return true
		}
	}
	return false
}

func validProbeMode(mode string) bool {
	switch normalizeProbeMode(mode) {
	case ProbeModeLight, ProbeModeArithmetic, ProbeModeSub2API, ProbeModeFirstToken:
		return true
	}
	return false
}

var probeAnswerNumber = regexp.MustCompile(`-?\d+`)

func arithmeticProbeChallenge() (string, string) {
	a, b := rand.IntN(50)+1, rand.IntN(50)+1
	op, answer := "+", a+b
	if rand.IntN(2) == 0 {
		a, b = max(a, b), min(a, b)
		op, answer = "-", a-b
	}
	return fmt.Sprintf("Calculate and respond with ONLY the number, nothing else.\n\nQ: 3 + 5 = ?\nA: 8\n\nQ: 12 - 7 = ?\nA: 5\n\nQ: %d %s %d = ?\nA:", a, op, b), strconv.Itoa(answer)
}

func (r *RealProbeRunner) probeWithMode(ctx context.Context, req ProbeRequest) ProbeOutcome {
	mode := normalizeProbeMode(req.ProbeMode)
	if !validProbeMode(mode) {
		return ProbeOutcome{Result: ResultUnsupported, Detail: "Unknown probe method"}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(defaultInt(req.MaxLatencyMs, DefaultMaxLatencyMs))*time.Millisecond)
	defer cancel()
	prompt, expected := defaultString(strings.TrimSpace(req.ProbePrompt), defaultProbePrompt), ""
	if mode == ProbeModeArithmetic {
		prompt, expected = arithmeticProbeChallenge()
	}
	protocol := "responses"
	switch req.ProviderFamily {
	case ProviderAnthropic:
		protocol = "messages"
	case ProviderGemini:
		protocol = "gemini"
	case ProviderCustom:
		protocol = "chat"
	}
	if mode == ProbeModeSub2API {
		protocol = "sub2api"
	}
	started := time.Now()
	outcome := func(result ResultKey, detail string) ProbeOutcome {
		// Native test errors can contain unknown account credentials. Never persist
		// their upstream text; only fixed descriptions are returned for this mode.
		return ProbeOutcome{Result: result, LatencyMs: int(time.Since(started).Milliseconds()), Detail: redact(detail, req.UpstreamKey)}
	}
	client := *r.client
	// In particular, an admin x-api-key must never follow a redirect elsewhere.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for attempt := 0; attempt < 2; attempt++ {
		httpReq, err := buildModeProbeRequest(ctx, req, protocol, prompt)
		if err != nil {
			return outcome(ResultUnsupported, "Probe method is unavailable for this account")
		}
		resp, err := client.Do(httpReq)
		if err != nil {
			return outcome(classifyTransportError(err), "Probe request failed or timed out")
		}
		// Compatibility fallback stays inside the same deadline and uses the same
		// challenge. Never retry auth/rate-limit/server failures or a partial answer.
		if protocol == "responses" && attempt == 0 && (resp.StatusCode == 404 || resp.StatusCode == 405) {
			resp.Body.Close()
			protocol = "chat"
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			result := classifyHTTPResponse(resp.StatusCode, nil, "", 0).Result
			if mode == ProbeModeSub2API && (resp.StatusCode == 404 || resp.StatusCode == 405) {
				result = ResultUnsupported
			}
			return outcome(result, fmt.Sprintf("Probe endpoint returned HTTP %d", resp.StatusCode))
		}
		if mode == ProbeModeFirstToken || mode == ProbeModeSub2API {
			if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
				resp.Body.Close()
				return outcome(ResultInvalidResponse, "Expected an SSE response")
			}
			result, detail := readModeProbeStream(resp.Body, protocol)
			resp.Body.Close()
			if ctx.Err() != nil {
				return outcome(classifyTransportError(ctx.Err()), "Probe timed out")
			}
			return outcome(result, detail)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
		resp.Body.Close()
		if err != nil {
			return outcome(classifyTransportError(err), "Reading the probe response failed or timed out")
		}
		if len(body) > 1024*1024 {
			return outcome(ResultInvalidResponse, "Probe response was too large")
		}
		text := arithmeticProbeText(body, protocol)
		if text != "" {
			for _, number := range probeAnswerNumber.FindAllString(text, -1) {
				if number == expected {
					return outcome(ResultOK, "")
				}
			}
		}
		return outcome(ResultInvalidResponse, "Arithmetic challenge returned an incorrect or empty answer")
	}
	return outcome(ResultInvalidResponse, "No valid probe response")
}

func buildModeProbeRequest(ctx context.Context, req ProbeRequest, protocol, prompt string) (*http.Request, error) {
	if protocol == "sub2api" {
		session := req.AdminSession
		if session == nil || session.Platform != upstream.PlatformSub2API || !session.IsAuthenticated() || strings.TrimSpace(req.AccountID) == "" {
			return nil, errors.New("sub2api account required")
		}
		headers := map[string]string{"Accept": "text/event-stream"}
		if session.AdminAPIKey != "" {
			headers["x-api-key"] = session.AdminAPIKey
		} else {
			headers["Authorization"] = "Bearer " + session.AccessToken
		}
		return newJSONRequest(ctx, http.MethodPost, strings.TrimRight(session.BaseURL, "/")+"/api/v1/admin/accounts/"+url.PathEscape(req.AccountID)+"/test",
			map[string]string{"model_id": req.ModelName, "prompt": "hi", "mode": "default"}, headers)
	}
	base := strings.TrimSuffix(strings.TrimRight(req.BaseURL, "/"), "/v1")
	model := defaultString(req.ModelName, defaultModelForProvider(req.ProviderFamily))
	stream := normalizeProbeMode(req.ProbeMode) == ProbeModeFirstToken
	tokens := 50
	if stream {
		tokens = max(128, req.MaxTokens)
	}
	endpoint := base + "/v1/responses"
	headers := map[string]string{"Authorization": "Bearer " + req.UpstreamKey}
	var payload any
	switch protocol {
	case "responses":
		body := map[string]any{"model": model, "input": prompt, "max_output_tokens": tokens, "stream": stream}
		if !stream {
			body["instructions"] = "You are a channel health-check endpoint. Answer the arithmetic challenge exactly and briefly."
		}
		payload = body
	case "chat":
		endpoint = base + "/v1/chat/completions"
		payload = map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": prompt}}, "max_tokens": tokens, "stream": stream}
	case "messages":
		endpoint = base + "/v1/messages"
		headers["x-api-key"], headers["anthropic-version"] = req.UpstreamKey, "2023-06-01"
		payload = map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": prompt}}, "max_tokens": tokens, "stream": stream}
	case "gemini":
		endpoint = base + "/v1beta/models/" + url.PathEscape(strings.TrimPrefix(model, "models/")) + ":generateContent"
		if stream {
			endpoint = strings.TrimSuffix(endpoint, ":generateContent") + ":streamGenerateContent?alt=sse"
		}
		headers["x-goog-api-key"] = req.UpstreamKey
		payload = map[string]any{"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": prompt}}}}, "generationConfig": map[string]int{"maxOutputTokens": tokens}}
	default:
		return nil, errors.New("unknown protocol")
	}
	return newJSONRequest(ctx, http.MethodPost, endpoint, payload, headers)
}

// Like Sub2API's channel monitor, arithmetic probes validate answer text rather
// than requiring a provider-specific finish reason. Reasoning is never an answer.
func arithmeticProbeText(body []byte, protocol string) string {
	type block struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Thought bool   `json:"thought"`
	}
	var response struct {
		Error      json.RawMessage `json:"error"`
		OutputText string          `json:"output_text"`
		Output     []struct {
			Type    string  `json:"type"`
			Content []block `json:"content"`
		} `json:"output"`
		Content []block `json:"content"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Candidates []struct {
			Content struct {
				Parts []block `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if json.Unmarshal(body, &response) != nil {
		// Some Responses gateways return a completed SSE response for stream=false.
		text, _ := qualityResponseText(body, protocol)
		return text
	}
	if len(response.Error) > 0 && string(response.Error) != "null" {
		return ""
	}
	parts := []string{}
	switch protocol {
	case "responses":
		if response.OutputText != "" {
			return response.OutputText
		}
		for _, output := range response.Output {
			if output.Type != "message" && output.Type != "" {
				continue
			}
			for _, b := range output.Content {
				if b.Type == "output_text" || b.Type == "" {
					parts = append(parts, b.Text)
				}
			}
		}
	case "chat":
		if len(response.Choices) > 0 {
			parts = append(parts, response.Choices[0].Message.Content)
		}
	case "messages":
		for _, b := range response.Content {
			if b.Type == "text" {
				parts = append(parts, b.Text)
			}
		}
	case "gemini":
		if len(response.Candidates) > 0 {
			for _, b := range response.Candidates[0].Content.Parts {
				if !b.Thought {
					parts = append(parts, b.Text)
				}
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// readModeProbeStream ignores headers, heartbeat events, reasoning and empty
// deltas. Direct probes stop at first answer text; native tests require completion.
func readModeProbeStream(body io.Reader, protocol string) (ResultKey, string) {
	result, detail := ResultInvalidResponse, "Stream ended without a valid answer"
	err := visitProbeSSE(body, func(event string, raw []byte) bool {
		var data struct {
			Type         string          `json:"type"`
			Error        json.RawMessage `json:"error"`
			Success      bool            `json:"success"`
			Text         string          `json:"text"`
			Delta        json.RawMessage `json:"delta"`
			ContentBlock struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content_block"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text    string `json:"text"`
						Thought bool   `json:"thought"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if json.Unmarshal(raw, &data) != nil {
			return false
		}
		if data.Type == "" {
			data.Type = event
		}
		if len(data.Error) > 0 && string(data.Error) != "null" && string(data.Error) != `""` || data.Type == "error" || data.Type == "response.failed" {
			result, detail = ResultServerError, "Upstream reported a probe error"
			return true
		}
		if protocol == "sub2api" {
			if data.Type == "test_complete" {
				if data.Success {
					result, detail = ResultOK, ""
				}
				return true
			}
			return false
		}
		text := ""
		switch protocol {
		case "responses":
			if data.Type == "response.output_text.delta" {
				_ = json.Unmarshal(data.Delta, &text)
			}
		case "chat":
			if len(data.Choices) > 0 {
				text = data.Choices[0].Delta.Content
			}
		case "messages":
			if data.Type == "content_block_delta" {
				var delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}
				if json.Unmarshal(data.Delta, &delta) == nil && delta.Type == "text_delta" {
					text = delta.Text
				}
			} else if data.Type == "content_block_start" && data.ContentBlock.Type == "text" {
				text = data.ContentBlock.Text
			}
		case "gemini":
			if len(data.Candidates) > 0 {
				for _, part := range data.Candidates[0].Content.Parts {
					if !part.Thought {
						text += part.Text
					}
				}
			}
		}
		if strings.TrimSpace(text) != "" {
			result, detail = ResultOK, ""
			return true
		}
		return false
	})
	if err != nil {
		return ResultInvalidResponse, "Invalid or interrupted probe stream"
	}
	return result, detail
}

// SSE supports comments, multiple data lines, CRLF, and a final event at EOF.
func visitProbeSSE(body io.Reader, visit func(string, []byte) bool) error {
	scanner := bufio.NewScanner(io.LimitReader(body, 4*1024*1024))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	event, data := "", ""
	dispatch := func() bool { return data != "" && visit(event, []byte(strings.TrimSuffix(data, "\n"))) }
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if dispatch() {
				return nil
			}
			event, data = "", ""
			continue
		}
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		}
		if strings.HasPrefix(line, "data:") {
			data += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ") + "\n"
			if len(data) > 1024*1024 {
				return errors.New("event too large")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	dispatch()
	return nil
}
