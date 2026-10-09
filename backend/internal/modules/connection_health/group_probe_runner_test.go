package connection_health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGroupProbeUsesGatewayProtocol(t *testing.T) {
	for _, tc := range []struct{ family, path, response, field string }{
		{ProviderOpenAI, "/v1/responses", `{"object":"response","status":"completed","output":[{"type":"message"}]}`, "input"},
		{ProviderAnthropic, "/v1/messages", `{"type":"message","content":[{"type":"text","text":"OK"}]}`, "messages"},
		{ProviderGemini, "/v1beta/models/test-model:generateContent", `{"candidates":[{"content":{"parts":[{"text":"OK"}]}}]}`, "contents"},
		{"other", "/v1/chat/completions", `{"choices":[{"message":{"content":"OK"}}]}`, "messages"},
	} {
		t.Run(tc.family, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != tc.path || r.Header.Get("Authorization") != "Bearer group-key" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if tc.family == ProviderAnthropic && (r.Header.Get("x-api-key") != "group-key" || r.Header.Get("anthropic-version") == "") {
					t.Error("missing Anthropic auth/version")
				}
				if tc.family == ProviderGemini && r.Header.Get("x-goog-api-key") != "group-key" {
					t.Error("missing Gemini auth")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body[tc.field] == nil {
					t.Errorf("missing protocol payload %s", tc.field)
				}
				if tc.family != ProviderGemini && body["model"] != "test-model" {
					t.Error("wrong model")
				}
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			outcome := NewRealProbeRunner().ProbeGroup(context.Background(), ProbeRequest{BaseURL: server.URL + "/v1/", UpstreamKey: "group-key", ModelName: "test-model", ProviderFamily: tc.family})
			if outcome.Result != ResultOK {
				t.Fatalf("expected success, got %s", outcome.Result)
			}
		})
	}
}

func TestGroupProbeRejectsFalseSuccessAndSanitizesErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		result     ResultKey
	}{
		{"error-envelope", `{"error":{"message":"group-key"}}`, 200, ResultInvalidResponse},
		{"generic-health", `{"ok":true}`, 200, ResultInvalidResponse},
		{"wrong-protocol", `{"choices":[{}]}`, 200, ResultInvalidResponse},
		{"failed-response", `{"object":"response","status":"failed","output":[{}]}`, 200, ResultInvalidResponse},
		{"invalid-json", `not json group-key`, 200, ResultInvalidResponse},
		{"auth", `{"error":"group-key"}`, 401, ResultAuth},
		{"rate-limit", `{"error":"group-key"}`, 429, ResultRateLimited},
		{"server", `{"error":"group-key"}`, 503, ResultServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			outcome := NewRealProbeRunner().ProbeGroup(context.Background(), ProbeRequest{BaseURL: server.URL, UpstreamKey: "group-key", ProviderFamily: ProviderOpenAI, ModelName: "model"})
			if outcome.Result != tc.result || outcome.Detail != "" {
				t.Fatalf("incorrect result or leaked details: %s", outcome.Result)
			}
		})
	}
}
