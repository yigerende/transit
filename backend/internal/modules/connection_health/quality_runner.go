package connection_health

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"transithub/backend/internal/modules/upstream"
)

type qualityProbeResult struct {
	Answer     string
	DurationMS int
	ErrorKey   string
}
type qualityProbeRunner interface {
	ProbeQuality(context.Context, upstream.ProbeCredential, string, QualitySettings, QualityQuestion) qualityProbeResult
}
type questionProbeRunner struct{ client *http.Client }

// This runner reads actual answer content; a 200 status alone is never a pass.
func (r *questionProbeRunner) ProbeQuality(ctx context.Context, cred upstream.ProbeCredential, provider string, q QualitySettings, question QualityQuestion) qualityProbeResult {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(q.TimeoutSeconds)*time.Second)
	defer cancel()
	base := strings.TrimSuffix(strings.TrimRight(cred.BaseURL, "/"), "/v1")
	endpoint := base + "/v1/responses"
	protocol := "responses"
	headers := map[string]string{"Authorization": "Bearer " + cred.Key}
	payload := map[string]any{"model": q.Model, "input": question.Prompt, "stream": false, "max_output_tokens": q.MaxTokens}
	if q.ReasoningEffort != "" {
		payload["reasoning"] = map[string]string{"effort": q.ReasoningEffort}
	}
	switch provider {
	case ProviderAnthropic:
		protocol = "messages"
		endpoint = base + "/v1/messages"
		headers["x-api-key"] = cred.Key
		headers["anthropic-version"] = "2023-06-01"
		payload = map[string]any{"model": q.Model, "max_tokens": q.MaxTokens, "messages": []map[string]string{{"role": "user", "content": question.Prompt}}}
	case ProviderGemini:
		protocol = "gemini"
		endpoint = base + "/v1beta/models/" + url.PathEscape(strings.TrimPrefix(q.Model, "models/")) + ":generateContent"
		headers["x-goog-api-key"] = cred.Key
		payload = map[string]any{"contents": []any{map[string]any{"parts": []any{map[string]string{"text": question.Prompt}}}}, "generationConfig": map[string]int{"maxOutputTokens": q.MaxTokens}}
	}
	started := time.Now()
	result := qualityProbeResult{}
	for attempt := 0; attempt < 2; attempt++ {
		req, err := newJSONRequest(ctx, http.MethodPost, endpoint, payload, headers)
		if err != nil {
			result.ErrorKey = "invalid_response"
			break
		}
		resp, err := r.client.Do(req)
		if err != nil {
			result.ErrorKey = string(classifyTransportError(err))
			break
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
		resp.Body.Close()
		if readErr != nil {
			result.ErrorKey = string(classifyTransportError(readErr))
			break
		}
		// Older OpenAI-compatible gateways expose only chat completions.
		if attempt == 0 && protocol == "responses" && (resp.StatusCode == 404 || resp.StatusCode == 405) {
			protocol = "chat"
			endpoint = base + "/v1/chat/completions"
			payload = map[string]any{"model": q.Model, "max_completion_tokens": q.MaxTokens, "stream": false, "messages": []map[string]string{{"role": "user", "content": question.Prompt}}}
			if q.ReasoningEffort != "" {
				payload["reasoning_effort"] = q.ReasoningEffort
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			result.ErrorKey = string(classifyHTTPResponse(resp.StatusCode, nil, cred.Key, 0).Result)
			break
		}
		if len(body) > 2*1024*1024 {
			result.ErrorKey = "invalid_response"
			break
		}
		answer, ok := qualityResponseText(body, protocol)
		if !ok || len(answer) > 65536 {
			result.ErrorKey = "invalid_response"
			break
		}
		result.Answer = redact(answer, cred.Key)
		break
	}
	result.DurationMS = int(time.Since(started).Milliseconds())
	return result
}

func qualityResponseText(body []byte, protocol string) (string, bool) {
	var root map[string]any
	if json.Unmarshal(body, &root) != nil {
		// Some Responses gateways return SSE even for stream=false. Only a full
		// response.completed event is accepted; partial deltas are not evidence.
		if protocol != "responses" {
			return "", false
		}
		scanner := bufio.NewScanner(bytes.NewReader(body))
		scanner.Buffer(make([]byte, 4096), 2*1024*1024)
		var complete map[string]any
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			var event map[string]any
			if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event) != nil {
				continue
			}
			if event["type"] == "response.failed" || event["type"] == "response.incomplete" || event["type"] == "error" {
				return "", false
			}
			if event["type"] == "response.completed" {
				complete, _ = event["response"].(map[string]any)
			}
		}
		if scanner.Err() != nil || complete == nil {
			return "", false
		}
		root = complete
	}
	if root["error"] != nil {
		return "", false
	}
	var text strings.Builder
	appendBlocks := func(raw any, kind string) {
		blocks, _ := raw.([]any)
		for _, block := range blocks {
			v, _ := block.(map[string]any)
			if v["type"] == kind {
				part, _ := v["text"].(string)
				text.WriteString(part)
			}
		}
	}
	switch protocol {
	case "responses":
		if root["status"] != "completed" {
			return "", false
		}
		output, _ := root["output"].([]any)
		for _, item := range output {
			v, _ := item.(map[string]any)
			if v["type"] == "message" {
				appendBlocks(v["content"], "output_text")
			}
		}
	case "chat":
		choices, _ := root["choices"].([]any)
		if len(choices) != 1 {
			return "", false
		}
		choice, _ := choices[0].(map[string]any)
		if choice["finish_reason"] != "stop" {
			return "", false
		}
		message, _ := choice["message"].(map[string]any)
		if content, ok := message["content"].(string); ok {
			text.WriteString(content)
		} else {
			appendBlocks(message["content"], "text")
		}
	case "messages":
		if root["stop_reason"] != "end_turn" && root["stop_reason"] != "stop_sequence" {
			return "", false
		}
		appendBlocks(root["content"], "text")
	case "gemini":
		candidates, _ := root["candidates"].([]any)
		if len(candidates) != 1 {
			return "", false
		}
		candidate, _ := candidates[0].(map[string]any)
		if candidate["finishReason"] != "STOP" {
			return "", false
		}
		content, _ := candidate["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		for _, part := range parts {
			v, _ := part.(map[string]any)
			if v["thought"] == true {
				continue
			}
			str, _ := v["text"].(string)
			text.WriteString(str)
		}
	default:
		return "", false
	}
	answer := strings.TrimSpace(text.String())
	return answer, answer != ""
}
