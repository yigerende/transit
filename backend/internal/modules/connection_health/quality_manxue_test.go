package connection_health

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

type manxueTransportFunc func(*http.Request) (*http.Response, error)

func (f manxueTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Route the fixed production origin into an isolated server, never the public API.
func manxueTestRunner(t *testing.T, handler http.HandlerFunc) *manxueProbeRunner {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	u, _ := url.Parse(server.URL)
	return &manxueProbeRunner{pollInterval: time.Millisecond, client: &http.Client{Transport: manxueTransportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != "manxue.ai" || !strings.HasPrefix(r.URL.Path, "/api/v1/tests") {
			t.Errorf("request escaped the documented API: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" || r.URL.RawQuery != "" {
			t.Error("credential in header or URL")
		}
		copy := r.Clone(r.Context())
		copy.URL.Scheme, copy.URL.Host = u.Scheme, u.Host
		return server.Client().Transport.RoundTrip(copy)
	})}}
}

func TestManxueTaskVerdictsAndRequestContract(t *testing.T) {
	for _, tc := range []struct{ name, benchmark, response, verdict, errorKey string }{
		{"candy pass", "candy", `"candy":{"status":"passed","answer":"test-secret answer","duration_ms":17000}`, "passed", ""},
		{"candy wrong", "candy", `"candy":{"status":"incorrect","answer":"wrong","duration_ms":11000}`, "failed", ""},
		{"candy error", "candy", `"candy":{"status":"error","error":"test-secret"}`, "", qualityPrefix + "manxueFailed"},
		{"candy missing", "candy", `"candy":null`, "", qualityPrefix + "manxueUnknown"},
		{"pelican normal", "pelican", `"assessment":{"quality":"normal","reason":"test-secret normal"},"result":{"duration_ms":32000,"html":"<html>pelican test-secret test-id</html>"}`, "passed", ""},
		{"pelican degraded", "pelican", `"assessment":{"quality":"degraded","reason":"poor result"}`, "failed", ""},
		{"pelican unknown", "pelican", `"assessment":{"quality":"unknown","reason":"no verdict"}`, "", qualityPrefix + "manxueUnknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts, gets, deletes := 0, 0, 0
			runner := manxueTestRunner(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodPost:
					posts++
					var input map[string]any
					if json.NewDecoder(r.Body).Decode(&input) != nil {
						t.Error("invalid body")
					}
					if input["base_url"] != "https://gateway.example/custom/v1" || input["api_key"] != "test-secret" || input["model"] != "test-model" || input["benchmark"] != tc.benchmark || input["protocol"] != "responses" || input["reasoning_effort"] != "high" || input["service_tier"] != "priority" || len(input) != 7 {
						t.Errorf("wrong request: %+v", input)
					}
					if !manxueTaskID.MatchString(r.Header.Get("Idempotency-Key")) {
						t.Error("missing idempotency key")
					}
					w.WriteHeader(http.StatusAccepted)
					fmt.Fprintf(w, `{"id":"test-id","benchmark":%q,"status":"running"}`, tc.benchmark)
				case http.MethodGet:
					gets++
					if r.URL.Path != "/api/v1/tests/test-id" || r.ContentLength > 0 {
						t.Error("wrong poll request")
					}
					if gets == 1 {
						fmt.Fprintf(w, `{"id":"test-id","benchmark":%q,"status":"running"}`, tc.benchmark)
						return
					}
					fmt.Fprintf(w, `{"id":"test-id","benchmark":%q,"status":"succeeded",%s}`, tc.benchmark, tc.response)
				case http.MethodDelete:
					deletes++
					w.WriteHeader(http.StatusNoContent)
				}
			})
			q := defaultQualitySettings()
			q.DetectionMethod, q.ManxueBenchmark, q.Model, q.ReasoningEffort, q.ManxueServiceTier = qualityMethodManxue, tc.benchmark, "test-model", "high", "priority"
			out := runner.ProbeQuality(context.Background(), upstream.ProbeCredential{BaseURL: "https://gateway.example/custom/v1/", Key: "test-secret"}, ProviderOpenAI, q, QualityQuestion{})
			if out.Verdict != tc.verdict || out.ErrorKey != tc.errorKey || posts != 1 || gets != 2 || deletes != 0 {
				t.Fatalf("wrong result/calls: %+v %d/%d/%d", out, posts, gets, deletes)
			}
			raw, _ := json.Marshal(out)
			for _, forbidden := range []string{"test-secret", "test-id"} {
				if strings.Contains(string(raw), forbidden) {
					t.Fatal("sensitive or unused data stored")
				}
			}
			if tc.name == "candy pass" && out.DurationMS != 17000 {
				t.Fatal("reported duration lost")
			}
			if tc.name == "pelican normal" && !strings.Contains(out.HTML, "<html>pelican") {
				t.Fatal("pelican artwork was not retained")
			}
		})
	}
}

func TestManxueTimeoutCancelRedirectAndMalformedTasks(t *testing.T) {
	for _, scenario := range []string{"timeout", "failed", "wrong-id", "wrong-benchmark", "missing-verdict", "malformed", "expired", "redirect"} {
		t.Run(scenario, func(t *testing.T) {
			posts, cancels, polls := 0, 0, 0
			runner := manxueTestRunner(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					cancels++
					w.WriteHeader(204)
					return
				}
				if r.Method == http.MethodPost {
					posts++
					if scenario == "redirect" {
						w.Header().Set("Location", "https://other.invalid/steal")
						w.WriteHeader(307)
						return
					}
					w.WriteHeader(202)
					io.WriteString(w, `{"id":"test-id","benchmark":"candy","status":"running"}`)
					return
				}
				polls++
				switch scenario {
				case "timeout":
					io.WriteString(w, `{"id":"test-id","benchmark":"candy","status":"running"}`)
				case "failed":
					io.WriteString(w, `{"id":"test-id","benchmark":"candy","status":"failed","error":"test-secret"}`)
				case "wrong-id":
					io.WriteString(w, `{"id":"other-id","benchmark":"candy","status":"succeeded","candy":{"status":"passed"}}`)
				case "wrong-benchmark":
					io.WriteString(w, `{"id":"test-id","benchmark":"pelican","status":"succeeded","candy":{"status":"passed"}}`)
				case "missing-verdict":
					io.WriteString(w, `{"id":"test-id","benchmark":"candy","status":"succeeded"}`)
				case "malformed":
					io.WriteString(w, `not-json test-secret`)
				case "expired":
					w.WriteHeader(404)
				}
			})
			q := defaultQualitySettings()
			q.DetectionMethod = qualityMethodManxue
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			out := runner.ProbeQuality(ctx, upstream.ProbeCredential{BaseURL: "https://gateway.example", Key: "test-secret"}, ProviderOpenAI, q, QualityQuestion{})
			if out.ErrorKey == "" || out.Verdict != "" || posts != 1 {
				t.Fatalf("bad task accepted: %+v", out)
			}
			if scenario == "timeout" && (cancels != 1 || out.ErrorKey != qualityPrefix+"manxueTimeout") {
				t.Fatal("timed out task was not cancelled")
			}
			if scenario == "redirect" && (polls != 0 || cancels != 0) {
				t.Fatal("redirect was followed")
			}
		})
	}
}

func TestManxueRateLimitAndIdempotency(t *testing.T) {
	posts, gets := 0, 0
	firstKey := ""
	runner := manxueTestRunner(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			if posts == 1 {
				firstKey = r.Header.Get("Idempotency-Key")
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(429)
				return
			}
			if firstKey == "" || firstKey != r.Header.Get("Idempotency-Key") {
				t.Error("retry changed idempotency key")
			}
			w.WriteHeader(202)
			io.WriteString(w, `{"id":"test-id","benchmark":"candy","status":"running"}`)
			return
		}
		gets++
		if gets == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(503)
			return
		}
		io.WriteString(w, `{"id":"test-id","benchmark":"candy","status":"succeeded","candy":{"status":"passed"}}`)
	})
	q := defaultQualitySettings()
	q.DetectionMethod = qualityMethodManxue
	start := time.Now()
	out := runner.ProbeQuality(context.Background(), upstream.ProbeCredential{BaseURL: "https://gateway.example", Key: "test-secret"}, ProviderCustom, q, QualityQuestion{})
	if out.ErrorKey != "" || out.Verdict != "passed" || posts != 2 || gets != 2 || time.Since(start) < 4*time.Second {
		t.Fatalf("rate limit ignored: %+v", out)
	}
	if manxueRetryDelay("15", 0, time.Now()) != 15*time.Second {
		t.Fatal("Retry-After ignored")
	}
}

func TestManxueSettingsCompatibilityAndIndependentVerdict(t *testing.T) {
	q := defaultQualitySettings()
	q.DetectionMethod, q.ManxueBenchmark, q.ManxueProtocol = "", "", ""
	if q.validate() != nil || q.DetectionMethod != qualityMethodQuestions {
		t.Fatal("legacy settings changed detection method")
	}
	q.DetectionMethod, q.Enabled, q.TimeoutSeconds, q.Questions = qualityMethodManxue, true, 600, nil
	if q.validate() != nil || len(q.activeQuestions()) != 1 {
		t.Fatal("API mode requires a local question")
	}
	q.ManxueBenchmark = "pelican"
	if q.validate() == nil {
		t.Fatal("pelican accepted xhigh")
	}
	q.ReasoningEffort = "high"
	q.ManxueProtocol = "chat_completions"
	if q.validate() != nil {
		t.Fatal("pelican chat mode rejected")
	}
	q.ManxueBenchmark = "candy"
	if q.validate() == nil {
		t.Fatal("candy accepted chat mode")
	}
	q.ManxueProtocol, q.ReasoningEffort = "responses", "ultra"
	if q.validate() != nil {
		t.Fatal("candy ultra rejected")
	}
	for _, mode := range []string{"content", "time", "content_time"} {
		q.Mode = mode
		state := QualityState{}
		for _, verdict := range []string{"failed", "failed", "", "passed", "passed"} {
			sample := QualitySample{Result: verdict, Answer: "unrelated", DurationMS: 599000, CreatedAt: time.Now()}
			state = applyQualitySample(state, q, q.activeQuestions()[0], sample)
			if verdict == "" && (!state.Degraded || state.Failures != 2 || state.Latest.Result != "error") {
				t.Fatal("unknown verdict changed evidence")
			}
			if verdict != "" && state.Latest.Result != verdict {
				t.Fatal("local mode overrode external verdict")
			}
		}
		if state.Degraded {
			t.Fatal("external recovery did not clear degradation")
		}
	}
}

func TestManxueURLValidation(t *testing.T) {
	for _, raw := range []string{"http://public.example", "https://localhost", "https://127.0.0.1", "https://10.1.2.3", "https://[::1]", "https://user:key@public.example", "https://public.example?key=x", "https://public.example#secret"} {
		if _, ok := manxueUpstreamBase(raw); ok {
			t.Errorf("accepted %q", raw)
		}
	}
	for raw, want := range map[string]string{"https://public.example/": "https://public.example/v1", "https://public.example/proxy/v1/": "https://public.example/proxy/v1", "https://public.example/custom": "https://public.example/custom"} {
		got, ok := manxueUpstreamBase(raw)
		if !ok || got != want {
			t.Errorf("lost custom path: %q", got)
		}
	}
}

type manxueServiceRunner struct{ calls int }

func (r *manxueServiceRunner) ProbeQuality(_ context.Context, _ upstream.ProbeCredential, _ string, _ QualitySettings, _ QualityQuestion) qualityProbeResult {
	r.calls++
	return qualityProbeResult{Verdict: "failed", Report: "degraded fixture", DurationMS: 12000}
}

func TestManxueSchedulingUsesExistingChannelSelection(t *testing.T) {
	svc, repo, health, _ := qualityTestService(t)
	runner := &manxueServiceRunner{}
	svc.qualityRunner = runner
	ctx := context.Background()
	q := defaultQualitySettings()
	q.DetectionMethod = qualityMethodManxue
	q.Enabled = true
	q.Questions = nil
	q, err := svc.SaveQualityConfiguration(ctx, "user", q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetGroupQuality(ctx, "user", "one", true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetGroupQuality(ctx, "user", "two", true); err != nil {
		t.Fatal(err)
	}
	scope := QualityScope{UserID: "user", WorkspaceID: "ws1", Settings: q}
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	history, _ := repo.ListQualityHistory(ctx, "user", "ws1", []string{"sub2api:ws1:a"}, 100)
	if runner.calls != 1 || len(history) != 1 || history[0].Result != "failed" || history[0].DetectionMethod != qualityMethodManxue || history[0].Benchmark != "candy" || history[0].Report != "degraded fixture" {
		t.Fatal("shared channel not detected exactly once with external result")
	}
	_, _ = svc.SetChannelQuality(ctx, "user", "sub2api:ws1:a", false)
	q.Model = "another-model"
	q, _ = svc.SaveQualityConfiguration(ctx, "user", q)
	svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
	if runner.calls != 1 || len(health.states) != 0 || len(health.events) != 0 {
		t.Fatal("disabled channel ran or detection changed health")
	}
}

func TestManxuePostgresSettingsAndHistory(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	repo := NewRepository(pool)
	q := defaultQualitySettings()
	q.DetectionMethod = qualityMethodManxue
	q.ManxueBenchmark = "pelican"
	q.ManxueProtocol = "chat_completions"
	q.ManxueServiceTier = "ultrafast"
	q.ReasoningEffort = "high"
	q.Enabled = true
	q.Revision = "external"
	if err := repo.SaveQualitySettings(ctx, "u", "w", q); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewRepository(pool).GetQualitySettings(ctx, "u", "w")
	if err != nil || loaded.DetectionMethod != q.DetectionMethod || loaded.ManxueProtocol != q.ManxueProtocol || loaded.ManxueBenchmark != q.ManxueBenchmark || loaded.ManxueServiceTier != q.ManxueServiceTier {
		t.Fatal("API options lost on reload")
	}
	_ = repo.SetQualityGroup(ctx, "u", "w", "g", true)
	id, _ := newID()
	sample := QualitySample{ID: id, TargetID: "channel", DetectionMethod: qualityMethodManxue, Benchmark: "pelican", Report: "normal", Result: "passed", CreatedAt: time.Now()}
	state := QualityState{TargetID: "channel", Revision: q.Revision, Latest: sample}
	if ok, err := repo.SaveQualityResult(ctx, "u", "w", []string{"g"}, q, state); err != nil || !ok {
		t.Fatal(err)
	}
	history, err := repo.ListQualityHistory(ctx, "u", "w", []string{"channel"}, 100)
	if err != nil || len(history) != 1 || history[0].Report != "normal" || history[0].Benchmark != "pelican" {
		t.Fatal("external evidence lost")
	}
	_, err = pool.Exec(ctx, `UPDATE connection_health_quality_settings SET config = config - 'detectionMethod' - 'manxueBenchmark' - 'manxueProtocol'`)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := repo.GetQualitySettings(ctx, "u", "w")
	if err != nil || legacy.DetectionMethod != qualityMethodQuestions {
		t.Fatal("legacy config no longer defaults to local questions")
	}
}
