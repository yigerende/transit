package connection_health

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"transithub/backend/internal/modules/upstream"
)

func TestManxueFailureDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, response, detail, phase string }{
		{"failed", `"status":"failed","phase":"complete","error":"读取 API 响应失败，请稍后重试"`, "读取 API 响应失败，请稍后重试", "complete"},
		{"cancelled", `"status":"cancelled","phase":"generating","error":"任务已取消"`, "任务已取消", "generating"},
		{"candy", `"status":"succeeded","phase":"classifying","candy":{"status":"error","error":"答案读取失败"}`, "答案读取失败", "classifying"},
		{"legacy", `"status":"failed"`, "", ""},
		{"unexpected diagnostic", `"status":"failed","phase":"secret task-id","error":{"body":"secret task-id"}`, "", ""},
		{"sensitive", `"status":"failed","phase":"complete","error":"secret task-id"`, "*** ***", "complete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := manxueTestRunner(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					w.WriteHeader(http.StatusAccepted)
					fmt.Fprint(w, `{"id":"task-id","benchmark":"candy","status":"running"}`)
					return
				}
				fmt.Fprintf(w, `{"id":"task-id","benchmark":"candy",%s}`, tc.response)
			})
			q := defaultQualitySettings()
			q.DetectionMethod = qualityMethodManxue
			out := runner.ProbeQuality(context.Background(), upstream.ProbeCredential{BaseURL: "https://gateway.example", Key: "secret"}, ProviderOpenAI, q, QualityQuestion{})
			if out.ErrorKey != qualityPrefix+"manxueFailed" || out.ErrorDetail != tc.detail || out.Phase != tc.phase || out.Verdict != "" {
				t.Fatalf("unexpected diagnostics: %+v", out)
			}
			state := applyQualitySample(QualityState{Failures: 2, Successes: 1, Degraded: true}, q, QualityQuestion{}, QualitySample{ErrorKey: out.ErrorKey, ErrorDetail: out.ErrorDetail, Phase: out.Phase})
			if state.Failures != 2 || state.Successes != 1 || !state.Degraded || state.Latest.Result != "error" {
				t.Fatal("API error changed degradation evidence")
			}
		})
	}
}

func TestManxueDiagnosticRedactionAndBounds(t *testing.T) {
	key := `sk-secret+/<&"value`
	quoted, _ := json.Marshal(key)
	variants := []string{key, string(quoted[1 : len(quoted)-1]), url.QueryEscape(key), url.PathEscape(key), html.EscapeString(key), "private-task-id"}
	raw, _ := json.Marshal(strings.Join(variants, " | ") + "\x00\n" + strings.Repeat("中文", 1000))
	detail := manxueErrorDetail(raw, key, "private-task-id")
	for _, secret := range variants {
		if strings.Contains(detail, secret) {
			t.Fatal("diagnostic exposed a credential or private task ID")
		}
	}
	if !utf8.ValidString(detail) || utf8.RuneCountInString(detail) != 1001 || !strings.HasSuffix(detail, "…") || strings.ContainsRune(detail, 0) || strings.Count(detail, "***") != len(variants) {
		t.Fatal("diagnostic was not sanitized and bounded")
	}
	for _, raw := range []string{"", "null", "123", `{"message":"secret"}`, `["secret"]`} {
		if manxueErrorDetail(json.RawMessage(raw), key, "") != "" {
			t.Fatal("unexpected diagnostic shape was saved")
		}
	}
}

func TestQualityFailureDiagnosticsSurviveHistory(t *testing.T) {
	for _, postgres := range []bool{false, true} {
		t.Run(fmt.Sprintf("postgres=%v", postgres), func(t *testing.T) {
			svc, _, _, _ := qualityTestService(t)
			if postgres {
				_, pool := qualityTestPool(t)
				svc.qualityRepo = NewRepository(pool)
			}
			enableManualQuality(t, svc)
			svc.qualityRunner = manxueTestRunner(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					w.WriteHeader(http.StatusAccepted)
					fmt.Fprint(w, `{"id":"private-task-id","benchmark":"pelican","status":"running"}`)
					return
				}
				fmt.Fprint(w, `{"id":"private-task-id","benchmark":"pelican","status":"failed","phase":"complete","error":"读取 API 响应失败 secret private-task-id"}`)
			})
			reader := svc.platformGroups.(fakePlatformGroupReader)
			reader.credByAccount["a"] = upstream.ProbeCredential{BaseURL: "https://gateway.example", Key: "secret"}
			svc.platformGroups = reader
			ctx := context.Background()
			const target = "sub2api:ws1:a"
			summary, err := svc.ProbeChannelQuality(ctx, "user", target, "one", "manxue_pelican")
			if err != nil {
				t.Fatal(err)
			}
			detail, err := svc.QualityHistoryDetail(ctx, "user", target, summary.ID)
			if err != nil {
				t.Fatal(err)
			}
			states, err := svc.qualityRepo.ListQualityStates(ctx, "user", "ws1")
			if err != nil || len(states) != 1 || states[0].Failures != 0 || states[0].Degraded {
				t.Fatal("request error counted as degradation")
			}
			history, err := svc.qualityRepo.ListQualityHistory(ctx, "user", "ws1", []string{target}, 60)
			if err != nil || len(history) != 1 {
				t.Fatal("diagnostic history missing")
			}
			for _, sample := range []QualitySample{summary, detail, states[0].Latest, history[0]} {
				if sample.ErrorDetail != "读取 API 响应失败 *** ***" || sample.Phase != "complete" || sample.Result != "error" || sample.ErrorKey != qualityPrefix+"manxueFailed" {
					t.Fatalf("saved diagnostic missing: %+v", sample)
				}
			}
		})
	}
}
