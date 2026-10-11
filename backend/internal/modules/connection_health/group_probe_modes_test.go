package connection_health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

func TestGroupProbeModesUseGroupKeyAndPersistActualMode(t *testing.T) {
	for _, mode := range []string{ProbeModeLight, ProbeModeArithmetic, ProbeModeFirstToken} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer group-secret" || r.Header.Get("x-api-key") != "" {
					t.Error("did not use group gateway credential")
				}
				var body struct {
					Input  string `json:"input"`
					Stream bool   `json:"stream"`
					Model  string `json:"model"`
					Tokens int    `json:"max_output_tokens"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if body.Model != "group-model" {
					t.Error("wrong group model")
				}
				switch mode {
				case ProbeModeLight:
					if body.Input != "Reply OK." || body.Stream || body.Tokens != 32 {
						t.Error("changed existing lightweight request")
					}
					fmt.Fprint(w, `{"object":"response","status":"completed","output":[{"type":"message"}]}`)
				case ProbeModeArithmetic:
					m := regexp.MustCompile(`Q: (\d+) ([+-]) (\d+) = \?\nA:$`).FindStringSubmatch(body.Input)
					if len(m) != 4 || body.Stream || body.Tokens != 50 {
						t.Error("invalid arithmetic request")
						return
					}
					a, _ := strconv.Atoi(m[1])
					b, _ := strconv.Atoi(m[3])
					answer := a + b
					if m[2] == "-" {
						answer = a - b
					}
					fmt.Fprintf(w, `{"output":[{"type":"message","content":[{"type":"output_text","text":"%d"}]}]}`, answer)
				case ProbeModeFirstToken:
					if body.Input != "hi" || !body.Stream {
						t.Error("invalid streaming request")
					}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
				}
			}))
			defer server.Close()
			repo := newFakeRepository()
			provider := &fakeGroupProbeProvider{fakePlatformGroupReader: fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "42", Platform: ProviderOpenAI}}}}
			svc := newAdminGroupsService(provider, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API, AccessToken: "admin-jwt", BaseURL: server.URL}}, repo)
			svc.dispatcher = panicIfCalledRemoteActionRunner{}
			ctx := context.Background()
			key := "group-secret"
			config, err := svc.SaveGroupProbeConfiguration(ctx, "user1", "42", GroupProbeConfigInput{Model: "group-model", ProbeMode: mode, Enabled: true, Key: &key})
			if err != nil || config.ProbeMode != mode || calls != 0 {
				t.Fatalf("save: %+v %v", config, err)
			}
			// Older clients omitting mode and leaving the password blank preserve both.
			blank := ""
			config, err = svc.SaveGroupProbeConfiguration(ctx, "user1", "42", GroupProbeConfigInput{Model: "group-model", Enabled: true, Key: &blank})
			if err != nil || config.ProbeMode != mode || config.CustomKeyID != "key-7" || provider.calls != 0 {
				t.Fatal("resave lost method/key")
			}
			stale := config
			stale.ProbeMode = ProbeModeSub2API
			svc.runScheduledGroupProbe(ctx, stale)
			sample, err := svc.ProbeAdminGroup(ctx, "user1", "42", "group-model")
			if err != nil || sample.Result != "ok" || sample.ProbeMode != mode || !sample.Manual || calls != 2 {
				t.Fatalf("probe: %+v %v calls=%d", sample, err, calls)
			}
			if len(repo.events) != 2 || repo.events[0].ProbeMode != mode || repo.events[0].Manual || repo.events[1].ProbeMode != mode || !repo.events[1].Manual {
				t.Fatal("history lost actual mode/manual flag")
			}
			if len(repo.states) != 0 || len(repo.budgetClaims) != 0 || len(repo.targetActionStates) != 0 {
				t.Fatal("group test touched channel actions")
			}
		})
	}
}

func TestGroupProbeModeRejectsAccountTestsAndUnknownModes(t *testing.T) {
	svc, repo, provider := groupConfigTestService(t)
	for _, mode := range []string{ProbeModeSub2API, "unknown"} {
		if _, err := svc.SaveGroupProbeConfiguration(context.Background(), "user1", "42", GroupProbeConfigInput{Model: "model", ProbeMode: mode, Enabled: true}); err == nil {
			t.Fatal("accepted invalid group mode")
		}
		if outcome := NewRealProbeRunner().ProbeGroup(context.Background(), ProbeRequest{ProbeMode: mode}); outcome.Result != ResultUnsupported {
			t.Fatal("runner accepted invalid mode")
		}
	}
	if provider.calls != 0 || len(repo.groupProbeConfigs) != 0 {
		t.Fatal("invalid method created key/config")
	}
}

func TestGroupFirstTokenStopsBeforeFullReplyAndUsesProviderProtocol(t *testing.T) {
	for _, tc := range []struct{ family, path, answer string }{
		{ProviderOpenAI, "/v1/responses", `{"type":"response.output_text.delta","delta":"hi"}`},
		{ProviderAnthropic, "/v1/messages", `{"type":"content_block_delta","delta":{"type":"text_delta","text":"hi"}}`},
		{ProviderGemini, "/v1beta/models/model:streamGenerateContent", `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`},
		{"other", "/v1/chat/completions", `{"choices":[{"delta":{"content":"hi"}}]}`},
	} {
		t.Run(tc.family, func(t *testing.T) {
			disconnected := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path || r.Header.Get("Authorization") != "Bearer group-secret" {
					t.Error("wrong gateway request")
				}
				if tc.family == ProviderGemini && (r.URL.Query().Get("alt") != "sse" || r.Header.Get("x-goog-api-key") != "group-secret") {
					t.Error("wrong gemini stream")
				}
				if tc.family == ProviderAnthropic && r.Header.Get("x-api-key") != "group-secret" {
					t.Error("wrong anthropic auth")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, ": heartbeat\n\ndata: {\"type\":\"response.reasoning_text.delta\",\"delta\":\"thinking\"}\n\n")
				w.(http.Flusher).Flush()
				time.Sleep(30 * time.Millisecond)
				fmt.Fprintf(w, "data: %s\n\n", tc.answer)
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
					close(disconnected)
				case <-time.After(time.Second):
				}
			}))
			defer server.Close()
			out := NewRealProbeRunner().ProbeGroup(context.Background(), ProbeRequest{ProbeMode: ProbeModeFirstToken, ProviderFamily: tc.family, BaseURL: server.URL, ModelName: "model", UpstreamKey: "group-secret", MaxLatencyMs: 800})
			if out.Result != ResultOK || out.LatencyMs < 20 || out.LatencyMs >= 800 || out.Detail != "" {
				t.Fatalf("wrong first token outcome: %+v", out)
			}
			select {
			case <-disconnected:
			case <-time.After(time.Second):
				t.Fatal("did not close stream at first text")
			}
		})
	}
}

func TestGroupProbeModeFailuresAreNotSuccess(t *testing.T) {
	for _, tc := range []struct {
		mode, body string
		status     int
	}{
		{ProbeModeArithmetic, `{"output_text":"9999"}`, 200},
		{ProbeModeArithmetic, `{"error":{"message":"group-secret"}}`, 200},
		{ProbeModeFirstToken, "data: {\"type\":\"response.reasoning_text.delta\",\"delta\":\"thinking\"}\n\n", 200},
		{ProbeModeFirstToken, "data: {\"type\":\"error\",\"message\":\"group-secret\"}\n\n", 200},
		{ProbeModeFirstToken, "group-secret", 401},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tc.mode == ProbeModeFirstToken {
				w.Header().Set("Content-Type", "text/event-stream")
			}
			w.WriteHeader(tc.status)
			fmt.Fprint(w, tc.body)
		}))
		out := NewRealProbeRunner().ProbeGroup(context.Background(), ProbeRequest{ProbeMode: tc.mode, ProviderFamily: ProviderOpenAI, BaseURL: server.URL, UpstreamKey: "group-secret"})
		server.Close()
		if out.Result == ResultOK || strings.Contains(out.Detail, "group-secret") {
			t.Fatalf("false success or leaked key: %+v", out)
		}
	}
}
