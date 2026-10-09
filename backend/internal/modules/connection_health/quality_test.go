package connection_health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"transithub/backend/internal/modules/upstream"
)

func TestQualityAnswerMatchingAndVerdicts(t *testing.T) {
	for _, tc := range []struct {
		mode, want, answer string
		pass               bool
	}{
		{"answer", "7.5", "7.5", true}, {"answer", "7.5", "thinking\nFINAL_ANSWER=7.5", true}, {"answer", "7.5", "FINAL_ANSWER=7.5\nFINAL_ANSWER=7.5", false}, {"answer", "7.5", "7.5 degrees", false}, {"keyword", "hello", "well hello there", true}, {"regex", "^7[.]5$", "7.5", true}, {"regex", "[", "7.5", false},
	} {
		if got := qualityAnswerMatches(QualityQuestion{MatchMode: tc.mode, Answer: tc.want}, tc.answer); got != tc.pass {
			t.Errorf("match %s %q: %v", tc.mode, tc.answer, got)
		}
	}
	q := defaultQualitySettings()
	q.Enabled = true
	q.Revision = "revision"
	q.Mode = "content_time"
	question := q.Questions[0]
	now := time.Now()
	state := QualityState{}
	sample := QualitySample{TargetID: "target", Answer: "wrong", DurationMS: 100, CreatedAt: now}
	state = applyQualitySample(state, q, question, sample)
	if state.Status != "suspect" || state.Degraded || state.Failures != 1 || state.NextQuestionID != q.Questions[1].ID {
		t.Fatal("first incorrect answer or rotation")
	}
	state = applyQualitySample(state, q, question, sample)
	if !state.Degraded || state.Status != "degraded" {
		t.Fatal("threshold did not mark degraded")
	}
	beforeQuestion := state.NextQuestionID
	sample.ErrorKey = "rate_limited"
	state = applyQualitySample(state, q, question, sample)
	if state.Status != "error" || !state.Degraded || state.Failures != 2 || state.NextQuestionID != beforeQuestion || state.Latest.Result != "error" {
		t.Fatal("request failure changed evidence")
	}
	sample.ErrorKey = ""
	sample.Answer = question.Answer
	state = applyQualitySample(state, q, question, sample)
	if state.Status != "recovering" || !state.Degraded || state.Successes != 1 {
		t.Fatal("recovered too soon")
	}
	state = applyQualitySample(state, q, question, sample)
	if state.Status != "normal" || state.Degraded || state.Successes != 2 {
		t.Fatal("normal streak did not recover")
	}
	if state.NextProbeAt.Sub(now) != 300*time.Second {
		t.Fatal("normal interval not applied")
	}
	sample.DurationMS = question.MaxDurationMS
	state = applyQualitySample(state, q, question, sample)
	if state.Latest.Result != "failed" || state.Latest.TimePassed || state.NextProbeAt.Sub(now) != 60*time.Second {
		t.Fatal("duration boundary or retry interval incorrect")
	}
	q.Mode = "content"
	state = applyQualitySample(state, q, question, sample)
	if state.Latest.Result != "passed" {
		t.Fatal("content-only considered time")
	}
	q.Mode = "time"
	sample.Answer = "wrong"
	sample.DurationMS = 1
	state = applyQualitySample(state, q, question, sample)
	if state.Latest.Result != "passed" {
		t.Fatal("time-only considered content")
	}
}

func TestQualityConfigValidation(t *testing.T) {
	for _, edit := range []func(*QualitySettings){
		func(q *QualitySettings) { q.IntervalSeconds = 9 }, func(q *QualitySettings) { q.RetrySeconds = 86401 }, func(q *QualitySettings) { q.Concurrency = 33 }, func(q *QualitySettings) { q.TimeoutSeconds = 0 }, func(q *QualitySettings) { q.FailureLimit = 0 }, func(q *QualitySettings) { q.MaxTokens = 1 }, func(q *QualitySettings) { q.HistoryLimit = 1001 }, func(q *QualitySettings) { q.Questions[0].MatchMode = "regex"; q.Questions[0].Answer = "[" }, func(q *QualitySettings) { q.Questions[1].ID = q.Questions[0].ID }, func(q *QualitySettings) { q.Questions = nil; q.Enabled = true },
	} {
		q := defaultQualitySettings()
		edit(&q)
		if q.validate() == nil {
			t.Fatal("accepted invalid configuration")
		}
	}
	q := defaultQualitySettings()
	if q.Enabled || q.validate() != nil {
		t.Fatal("defaults should be valid and disabled")
	}
}

func TestQualityResponseCompletionRequired(t *testing.T) {
	for _, tc := range []struct {
		protocol, body string
		valid          bool
	}{
		{"responses", `{"status":"completed","output":[{"type":"reasoning","content":[{"type":"output_text","text":"ignore"}]},{"type":"message","content":[{"type":"output_text","text":"7.5"}]}]}`, true},
		{"responses", `{"status":"incomplete","output":[{"type":"message","content":[{"type":"output_text","text":"7.5"}]}]}`, false},
		{"responses", `{"status":"completed","output":[]}`, false},
		{"chat", `{"choices":[{"finish_reason":"stop","message":{"content":"7.5"}}]}`, true},
		{"chat", `{"choices":[{"finish_reason":"length","message":{"content":"7.5"}}]}`, false},
		{"messages", `{"stop_reason":"end_turn","content":[{"type":"text","text":"7.5"}]}`, true},
		{"gemini", `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"thought":true,"text":"ignored"},{"text":"7.5"}]}}]}`, true},
		{"responses", "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"7.5\"}]}]}}\n\n", true},
		{"responses", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"7.5\"}\n\n", false},
		{"responses", `{"error":{"message":"secret"}}`, false},
	} {
		answer, ok := qualityResponseText([]byte(tc.body), tc.protocol)
		if ok != tc.valid || ok && answer != "7.5" {
			t.Errorf("%s completion: %q %v", tc.protocol, answer, ok)
		}
	}
}

func TestQualityRunnerReadsBodyAndUsesConfiguredPrompt(t *testing.T) {
	for _, scenario := range []string{"responses", "chat-fallback", "rate-limit", "incomplete", "echo-key"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer test-secret" {
					t.Error("missing channel credential")
				}
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["model"] != "test-model" {
					t.Error("wrong model")
				}
				if r.URL.Path == "/v1/responses" {
					if body["input"] != "question prompt" || body["max_output_tokens"] != float64(8192) {
						t.Error("ignored question/config")
					}
					if scenario == "chat-fallback" {
						w.WriteHeader(404)
						return
					}
				}
				if scenario == "rate-limit" {
					w.WriteHeader(429)
					_, _ = w.Write([]byte("test-secret"))
					return
				}
				w.(http.Flusher).Flush()
				time.Sleep(25 * time.Millisecond)
				if scenario == "incomplete" {
					_, _ = w.Write([]byte(`{"status":"incomplete","output":[]}`))
					return
				}
				if scenario == "chat-fallback" {
					_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"7.5"}}]}`))
					return
				}
				answer := "7.5"
				if scenario == "echo-key" {
					answer = "test-secret"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": answer}}}}})
			}))
			defer server.Close()
			q := defaultQualitySettings()
			q.Model = "test-model"
			question := q.Questions[0]
			question.Prompt = "question prompt"
			result := (&questionProbeRunner{client: server.Client()}).ProbeQuality(context.Background(), upstream.ProbeCredential{BaseURL: server.URL + "/v1", Key: "test-secret"}, ProviderOpenAI, q, question)
			if scenario == "rate-limit" {
				if result.ErrorKey != "rate_limited" {
					t.Fatal("rate limit misclassified")
				}
			} else if scenario == "incomplete" {
				if result.ErrorKey != "invalid_response" {
					t.Fatal("incomplete accepted")
				}
			} else if result.ErrorKey != "" || result.DurationMS < 20 {
				t.Fatal("must include body read time")
			}
			encoded, _ := json.Marshal(result)
			if strings.Contains(string(encoded), "test-secret") {
				t.Fatal("credential leaked into result")
			}
			if scenario == "chat-fallback" && calls != 2 {
				t.Fatal("fallback not used")
			}
		})
	}
}
