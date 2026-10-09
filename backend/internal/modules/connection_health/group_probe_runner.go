package connection_health

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Group probes go through the connected gateway using its group-bound key.
// Choose the gateway protocol by the actual group's platform, not its channels.
func (r *RealProbeRunner) ProbeGroup(ctx context.Context, req ProbeRequest) ProbeOutcome {
	base := strings.TrimRight(req.BaseURL, "/")
	if strings.HasSuffix(base, "/v1") {
		base = strings.TrimSuffix(base, "/v1")
	}
	endpoint := base + "/v1/chat/completions"
	headers := map[string]string{"Authorization": "Bearer " + req.UpstreamKey}
	var payload any = map[string]any{"model": req.ModelName, "max_tokens": 32, "messages": []map[string]string{{"role": "user", "content": "Reply OK."}}}
	switch req.ProviderFamily {
	case ProviderOpenAI:
		endpoint = base + "/v1/responses"
		payload = map[string]any{"model": req.ModelName, "input": "Reply OK.", "max_output_tokens": 32, "stream": false}
	case ProviderAnthropic:
		endpoint = base + "/v1/messages"
		headers["x-api-key"] = req.UpstreamKey
		headers["anthropic-version"] = "2023-06-01"
	case ProviderGemini:
		endpoint = base + "/v1beta/models/" + url.PathEscape(strings.TrimPrefix(req.ModelName, "models/")) + ":generateContent"
		headers["x-goog-api-key"] = req.UpstreamKey
		payload = map[string]any{"contents": []any{map[string]any{"parts": []any{map[string]string{"text": "Reply OK."}}}}, "generationConfig": map[string]int{"maxOutputTokens": 32}}
	}
	httpReq, err := newJSONRequest(ctx, http.MethodPost, endpoint, payload, headers)
	if err != nil {
		return ProbeOutcome{Result: ResultInvalidResponse}
	}
	started := time.Now()
	// Group routing and reasoning models may take longer than channel probes.
	client := *r.client
	client.Timeout = 30 * time.Second
	resp, err := client.Do(httpReq)
	if err != nil {
		return ProbeOutcome{Result: classifyTransportError(err), LatencyMs: int(time.Since(started).Milliseconds())}
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	elapsed := int(time.Since(started).Milliseconds())
	if readErr != nil {
		return ProbeOutcome{Result: classifyTransportError(readErr), LatencyMs: elapsed}
	}
	outcome := classifyHTTPResponse(resp.StatusCode, body, req.UpstreamKey, elapsed)
	if outcome.Result != ResultOK {
		outcome.Detail = ""
		return outcome
	}
	// A 200 error envelope or a generic JSON health check is not a model reply.
	var result struct {
		Error      json.RawMessage   `json:"error"`
		Object     string            `json:"object"`
		Status     string            `json:"status"`
		Type       string            `json:"type"`
		Output     []json.RawMessage `json:"output"`
		Choices    []json.RawMessage `json:"choices"`
		Content    []json.RawMessage `json:"content"`
		Candidates []json.RawMessage `json:"candidates"`
	}
	valid := json.Unmarshal(body, &result) == nil && (len(result.Error) == 0 || string(result.Error) == "null")
	switch req.ProviderFamily {
	case ProviderOpenAI:
		valid = valid && result.Object == "response" && (result.Status == "completed" || result.Status == "incomplete") && len(result.Output) > 0
	case ProviderAnthropic:
		valid = valid && result.Type == "message" && len(result.Content) > 0
	case ProviderGemini:
		valid = valid && len(result.Candidates) > 0
	default:
		valid = valid && len(result.Choices) > 0
	}
	if !valid {
		return ProbeOutcome{Result: ResultInvalidResponse, LatencyMs: elapsed}
	}
	return ProbeOutcome{Result: ResultOK, LatencyMs: elapsed}
}
