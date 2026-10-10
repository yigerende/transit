package connection_health

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

func TestArithmeticProbeProtocolsAndWrongAnswers(t *testing.T) {
	challengePattern := regexp.MustCompile(`Q: (\d+) ([+-]) (\d+) = \?\nA:$`)
	for _, family := range []string{ProviderOpenAI, ProviderCustom, ProviderAnthropic, ProviderGemini} {
		for _, correct := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/correct=%v", family, correct), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					prompt, path, header, auth := "", "/v1/responses", "Authorization", "Bearer probe-secret"
					tokens := body["max_output_tokens"]
					switch family {
					case ProviderOpenAI:
						prompt, _ = body["input"].(string)
						if body["instructions"] == nil {
							t.Error("missing monitor instructions")
						}
					case ProviderCustom, ProviderAnthropic:
						prompt = body["messages"].([]any)[0].(map[string]any)["content"].(string)
						tokens, path = body["max_tokens"], "/v1/chat/completions"
						if family == ProviderAnthropic {
							path, header, auth = "/v1/messages", "x-api-key", "probe-secret"
						}
					case ProviderGemini:
						prompt = body["contents"].([]any)[0].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"].(string)
						tokens = body["generationConfig"].(map[string]any)["maxOutputTokens"]
						path, header, auth = "/v1beta/models/model:generateContent", "x-goog-api-key", "probe-secret"
					}
					if r.URL.Path != path || r.Header.Get(header) != auth || tokens != float64(50) || body["stream"] == true {
						t.Errorf("invalid request: %s tokens=%v", r.URL.Path, tokens)
					}
					m := challengePattern.FindStringSubmatch(prompt)
					if len(m) != 4 {
						t.Errorf("not a monitor challenge: %q", prompt)
						return
					}
					a, _ := strconv.Atoi(m[1])
					b, _ := strconv.Atoi(m[3])
					answer := a + b
					if m[2] == "-" {
						answer = a - b
					}
					if !correct {
						answer = 9999
					}
					switch family {
					case ProviderOpenAI:
						fmt.Fprintf(w, `{"output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":"%d"}]}]}`, answer)
					case ProviderCustom:
						fmt.Fprintf(w, `{"choices":[{"message":{"content":"%d"}}]}`, answer)
					case ProviderAnthropic:
						fmt.Fprintf(w, `{"content":[{"type":"text","text":"%d"}]}`, answer)
					case ProviderGemini:
						fmt.Fprintf(w, `{"candidates":[{"content":{"parts":[{"text":"%d"}]}}]}`, answer)
					}
				}))
				defer server.Close()
				out := NewRealProbeRunner().Probe(context.Background(), ProbeRequest{ProbeMode: ProbeModeArithmetic, BaseURL: server.URL + "/v1", UpstreamKey: "probe-secret", ProviderFamily: family, ModelName: "model", MaxTokens: 1})
				if (out.Result == ResultOK) != correct {
					t.Fatalf("unexpected result: %+v", out)
				}
			})
		}
	}
}

func TestProbeModeFallbackOnlyForUnsupportedEndpoint(t *testing.T) {
	for _, code := range []int{404, 405, 401, 429, 500} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path == "/v1/responses" {
					w.WriteHeader(code)
					return
				}
				if r.URL.Path != "/v1/chat/completions" {
					t.Error("wrong fallback path")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n")
			}))
			defer server.Close()
			out := NewRealProbeRunner().Probe(context.Background(), ProbeRequest{ProbeMode: ProbeModeFirstToken, BaseURL: server.URL})
			fallback := code == 404 || code == 405
			if (out.Result == ResultOK) != fallback || (calls.Load() == 2) != fallback {
				t.Fatalf("retry/result mismatch: %d %+v", calls.Load(), out)
			}
		})
	}
}

func TestFirstTokenWaitsForAnswerAndCancelsUpstream(t *testing.T) {
	for _, family := range []string{ProviderOpenAI, ProviderCustom, ProviderAnthropic, ProviderGemini} {
		t.Run(family, func(t *testing.T) {
			cancelled := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(cancelled)
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if family != ProviderGemini && body["stream"] != true {
					t.Error("stream must be enabled")
				}
				if family == ProviderGemini && (r.URL.Path != "/v1beta/models/model:streamGenerateContent" || r.URL.Query().Get("alt") != "sse") {
					t.Error("wrong Gemini streaming URL")
				}
				noise, answer := "", ""
				switch family {
				case ProviderOpenAI:
					noise, answer = `{"type":"response.reasoning_text.delta","delta":"thinking"}`, `{"type":"response.output_text.delta","delta":"H"}`
				case ProviderCustom:
					noise, answer = `{"choices":[{"delta":{"role":"assistant","reasoning_content":"thinking"}}]}`, `{"choices":[{"delta":{"content":"H"}}]}`
				case ProviderAnthropic:
					noise, answer = `{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"thinking"}}`, `{"type":"content_block_delta","delta":{"type":"text_delta","text":"H"}}`
				case ProviderGemini:
					noise, answer = `{"candidates":[{"content":{"parts":[{"thought":true,"text":"thinking"}]}}]}`, `{"candidates":[{"content":{"parts":[{"text":"H"}]}}]}`
				}
				w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
				fmt.Fprintf(w, ": heartbeat\r\n\r\ndata: %s\r\n\r\n", noise)
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
					t.Error("closed before answer")
					return
				case <-time.After(60 * time.Millisecond):
				}
				fmt.Fprintf(w, "data: %s\n\n", answer)
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
				case <-time.After(time.Second):
					t.Error("probe did not cancel after answer")
				}
			}))
			defer server.Close()
			out := NewRealProbeRunner().Probe(context.Background(), ProbeRequest{ProbeMode: ProbeModeFirstToken, ProviderFamily: family, BaseURL: server.URL, ModelName: "model", MaxLatencyMs: 800})
			<-cancelled
			if out.Result != ResultOK || out.LatencyMs < 50 || out.LatencyMs >= 800 {
				t.Fatalf("incorrect first-answer timing: %+v", out)
			}
		})
	}
}

func TestModeStreamRejectsMissingAnswersErrorsAndTruncation(t *testing.T) {
	for _, body := range []string{": heartbeat\n\n", "data: [DONE]\n\n", "data: invalid json\n\n", "data: {\"type\":\"response.failed\"}\n\n", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"  \"}\n\n", "data: " + strings.Repeat("x", 1024*1024)} {
		if result, _ := readModeProbeStream(strings.NewReader(body), "responses"); result == ResultOK {
			t.Fatal("invalid stream passed")
		}
	}
	result, _ := readModeProbeStream(strings.NewReader("event: response.output_text.delta\r\ndata: {\"delta\":\r\ndata: \"ok\"}\r\n"), "responses")
	if result != ResultOK {
		t.Fatal("valid multiline final event was lost")
	}
	for _, body := range []string{`{"error":{"message":"42"},"output_text":"42"}`, `{"output":[{"type":"reasoning","content":[{"type":"output_text","text":"42"}]}]}`, `{}`} {
		if arithmeticProbeText([]byte(body), "responses") != "" {
			t.Fatal("error/reasoning accepted as arithmetic answer")
		}
	}
}

func TestProbeModeDeadlineCoversBodyAndFallback(t *testing.T) {
	for _, mode := range []string{ProbeModeArithmetic, ProbeModeFirstToken, ProbeModeSub2API} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/responses" {
					time.Sleep(35 * time.Millisecond)
					w.WriteHeader(404)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer server.Close()
			out := NewRealProbeRunner().Probe(context.Background(), ProbeRequest{ProbeMode: mode, BaseURL: server.URL, MaxLatencyMs: 100, AdminSession: &upstream.Session{Platform: upstream.PlatformSub2API, BaseURL: server.URL, AdminAPIKey: "admin-secret"}, AccountID: "1"})
			if out.Result != ResultNetworkFluctuation || out.LatencyMs < 80 || out.LatencyMs > 400 {
				t.Fatalf("deadline failed: %+v", out)
			}
		})
	}
}

func TestNativeProbeRequiresCompletionAndKeepsAdminAuthPrivate(t *testing.T) {
	for _, apiKey := range []bool{true, false} {
		for _, event := range []string{`{"type":"test_complete","success":true}`, `{"type":"test_complete","success":false}`, `{"type":"error","error":"admin-secret jwt-secret account-secret"}`, `{"type":"content","text":"hi"}`} {
			t.Run(fmt.Sprintf("apiKey=%v/%s", apiKey, event), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]string
					_ = json.NewDecoder(r.Body).Decode(&body)
					if r.Method != "POST" || r.URL.Path != "/api/v1/admin/accounts/123/test" || body["model_id"] != "model" || body["prompt"] != "hi" || body["mode"] != "default" {
						t.Error("invalid native request")
					}
					if apiKey {
						if r.Header.Get("x-api-key") != "admin-secret" || r.Header.Get("Authorization") != "" {
							t.Error("invalid admin API key auth")
						}
					} else if r.Header.Get("Authorization") != "Bearer jwt-secret" || r.Header.Get("x-api-key") != "" {
						t.Error("invalid JWT auth")
					}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"type\":\"test_start\"}\n\ndata: {\"type\":\"content\",\"text\":\"hi\"}\n\n")
					w.(http.Flusher).Flush()
					time.Sleep(20 * time.Millisecond)
					fmt.Fprintf(w, "data: %s\n\n", event)
				}))
				defer server.Close()
				session := upstream.Session{Platform: upstream.PlatformSub2API, BaseURL: server.URL, AccessToken: "jwt-secret"}
				if apiKey {
					session.AdminAPIKey = "admin-secret"
				}
				out := NewRealProbeRunner().Probe(context.Background(), ProbeRequest{ProbeMode: ProbeModeSub2API, AdminSession: &session, AccountID: "123", ModelName: "model"})
				if (out.Result == ResultOK) != strings.Contains(event, `"success":true`) || out.LatencyMs < 15 || strings.Contains(out.Detail, "secret") {
					t.Fatalf("wrong native result: %+v", out)
				}
			})
		}
	}
	for _, session := range []*upstream.Session{nil, {Platform: upstream.PlatformNewAPI, AccessToken: "key", UserID: "1"}, {Platform: upstream.PlatformSub2API}} {
		out := NewRealProbeRunner().Probe(context.Background(), ProbeRequest{ProbeMode: ProbeModeSub2API, AdminSession: session, AccountID: "123"})
		if out.Result != ResultUnsupported {
			t.Fatal("unsupported native session accepted")
		}
	}
}

func TestNativeProbeDoesNotFollowRedirectsOrAcceptJSON(t *testing.T) {
	var leaked atomic.Int32
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer redirect.Close()
	for _, code := range []int{200, 302, 404, 405} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", redirect.URL)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			fmt.Fprint(w, `{"success":true}`)
		}))
		out := NewRealProbeRunner().Probe(context.Background(), ProbeRequest{ProbeMode: ProbeModeSub2API, AdminSession: &upstream.Session{Platform: upstream.PlatformSub2API, BaseURL: server.URL, AdminAPIKey: "admin-secret"}, AccountID: "1"})
		server.Close()
		if out.Result == ResultOK || leaked.Load() != 0 {
			t.Fatalf("invalid native success/redirect: %+v", out)
		}
		if (code == 404 || code == 405) && out.Result != ResultUnsupported {
			t.Fatal("missing API must be unsupported")
		}
	}
}

func TestSchedulerProbeMethodsSharedSelectionAndCredentialFailure(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		svc, repo, reader := cadenceFixture(1)
		svc.mySites = fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API, BaseURL: "https://admin.test", AdminAPIKey: "admin-test"}}
		repo.policies[0].ProbeMode = ProbeModeSub2API
		other := repo.policies[0]
		other.ID = "p2"
		other.ProbeMode = ProbeModeFirstToken
		if mixed {
			other.ModelTargets = []ModelTarget{{ModelName: "direct", Enabled: true}}
			reader.accountsByGrp["g1"][0].Models = "model-0,direct"
		}
		repo.policies = append(repo.policies, other)
		repo.groupAssignments = append(repo.groupAssignments, GroupPolicyAssignment{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", PolicyID: "p2"})
		reader.credErr = map[string]error{"0": fmt.Errorf("export unavailable")}
		requests := 0
		svc.probeRunner.client.Transport = latencyTestTransport(func(r *http.Request) (*http.Response, error) {
			requests++
			if r.URL.Path != "/api/v1/admin/accounts/0/test" {
				t.Error("unexpected direct request")
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"test_complete\",\"success\":true}\n\n"))}, nil
		})
		jobs := svc.collectAdminProbeJobsWithGroups(context.Background(), repo.policies, nil, repo.groupAssignments, nil)
		if len(jobs) != 1 {
			t.Fatalf("expected one shared job, got %d", len(jobs))
		}
		svc.runAdminProbeJob(context.Background(), jobs[0])
		wantCredentials := int32(0)
		if mixed {
			wantCredentials = 1
		}
		if requests != 1 || reader.credentials.Load() != wantCredentials {
			t.Fatalf("requests=%d exports=%d mixed=%v", requests, reader.credentials.Load(), mixed)
		}
		found := false
		for _, e := range repo.events {
			if e.ModelName == "model-0" {
				found = true
				if e.Result != "ok" || e.ProbeMode != ProbeModeSub2API {
					t.Fatalf("wrong native event %+v", e)
				}
			}
		}
		if !found {
			t.Fatal("missing native history")
		}
	}
}

func TestProbeModePolicySaveAndGroupEditPreserveSelection(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{ProbeModeLight, ProbeModeArithmetic, ProbeModeSub2API, ProbeModeFirstToken} {
		svc, repo, edit := groupPolicyEditFixture()
		edit.ProbeMode = mode
		saved, err := svc.SavePolicy(ctx, "user1", edit)
		if err != nil || saved.ProbeMode != mode {
			t.Fatalf("save mode: %+v %v", saved, err)
		}
		edit.ProbeMode = ""
		saved, err = svc.SavePolicy(ctx, "user1", edit)
		if err != nil || saved.ProbeMode != mode {
			t.Fatal("old client erased mode")
		}
		_, err = svc.SetAdminGroupPolicyConfiguration(ctx, "user1", "g1", AdminGroupPolicyConfigurationInput{EditPolicy: &edit})
		readBack, readErr := repo.GetPolicy(ctx, edit.ID, "user1", "ws1")
		if err != nil || readErr != nil || readBack.ProbeMode != mode {
			t.Fatal("group edit erased mode")
		}
	}
	if _, _, err := buildPolicyAndTargets("u", "w", "p", PolicyInput{Name: "invalid", ProbeMode: "unknown"}); err == nil {
		t.Fatal("invalid mode accepted")
	}
}

func TestSharedLatencyDecisionUsesWinningMethodWithoutMixingHistory(t *testing.T) {
	now := time.Now()
	p := latencyTestPolicy()
	p.ProbeMode = ProbeModeFirstToken
	other := latencyTestPolicy()
	other.ID = "p2"
	other.ProbeMode = ProbeModeArithmetic
	samples := []PriorityProbeSample{{ID: "first", ModelName: "model", ProbeMode: ProbeModeFirstToken, LatencyMs: 1000, CreatedAt: now}, {ID: "full", ModelName: "model", ProbeMode: ProbeModeArithmetic, LatencyMs: 15000, CreatedAt: now}, {ID: "old-default", ModelName: "model", LatencyMs: 20000, CreatedAt: now}}
	for _, policies := range [][]Policy{{p, other}, {other, p}} {
		d := latencyDecisionForTarget(upstream.PlatformSub2API, policies, nil, samples, nil, now)
		if d == nil || d.SharedPolicyCount != 2 || d.ProbeMode != ProbeModeFirstToken || d.SampleCount != 1 || d.AverageMs == nil || *d.AverageMs != 1000 {
			t.Fatalf("mixed or missing winning samples: %+v", d)
		}
	}
}
