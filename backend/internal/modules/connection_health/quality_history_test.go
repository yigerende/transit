package connection_health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
	"transithub/backend/internal/shared/authctx"
)

func TestQualityHistoryScopedSortedAndAvailableOffline(t *testing.T) {
	svc, repo, _, _ := qualityTestService(t)
	const target = "sub2api:ws1:a"
	now := time.Now()
	later := now.Add(time.Minute)
	older := now.Add(-time.Minute)
	repo.history[qualityScopeKey("user", "ws1")] = []QualitySample{
		{ID: "older", TargetID: target, StartedAt: &older, CreatedAt: later, HTML: "<svg>older</svg>"},
		{ID: "newer", TargetID: target, StartedAt: &later, CreatedAt: later, Prompt: "saved question", Answer: strings.Repeat("a", 5000), HTML: "<svg>newer</svg>"},
		{ID: "legacy", TargetID: target, CreatedAt: now},
		{ID: "other-channel", TargetID: "sub2api:ws1:other", CreatedAt: later},
	}
	svc.mySites = qualityOfflineSites{}
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc)
	for _, tc := range []struct {
		user, target, id string
		status           int
	}{
		{"", target, "newer", http.StatusUnauthorized},
		{"user", target, "", http.StatusOK},
		{"user", target, "newer", http.StatusOK},
		{"user", target, "missing", http.StatusNotFound},
		{"user", target, "other-channel", http.StatusNotFound},
		{"user", "sub2api:foreign:a", "newer", http.StatusNotFound},
		{"other-user", target, "newer", http.StatusNotFound},
	} {
		path := "/api/connection-health/targets/" + tc.target + "/quality-history"
		if tc.id != "" {
			path += "/" + tc.id
		}
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if tc.user != "" {
			r = r.WithContext(authctx.WithUserID(r.Context(), tc.user))
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s %s: got %d want %d: %s", tc.user, path, w.Code, tc.status, w.Body.String())
		}
		if tc.status != http.StatusOK {
			continue
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private history is cacheable")
		}
		if tc.id == "" {
			var list []QualitySample
			if json.Unmarshal(w.Body.Bytes(), &list) != nil || len(list) != 3 || list[0].ID != "newer" || list[1].ID != "legacy" || list[2].ID != "older" {
				t.Fatal("history order or channel isolation incorrect")
			}
			if list[0].HTML != "" || list[0].Prompt != "" || !list[0].HasHTML || len(list[0].Answer) != 4000 {
				t.Fatal("summary contains full evidence")
			}
		} else {
			var sample QualitySample
			if json.Unmarshal(w.Body.Bytes(), &sample) != nil || sample.HTML != "<svg>newer</svg>" || sample.Prompt != "saved question" || len(sample.Answer) != 5000 {
				t.Fatal("detail lost evidence")
			}
		}
	}
}

func TestQualityPersistsPromptStartTimeAndPelicanHTML(t *testing.T) {
	svc, repo, _, _ := qualityTestService(t)
	enableManualQuality(t, svc)
	ctx := context.Background()
	const target = "sub2api:ws1:a"
	summary, err := svc.ProbeChannelQuality(ctx, "user", target, "one", "questions")
	if err != nil {
		t.Fatal(err)
	}
	custom, _ := svc.QualityHistoryDetail(ctx, "user", target, summary.ID)
	if custom.Prompt == "" || custom.Mode == "" || custom.StartedAt == nil || custom.StartedAt.After(custom.CreatedAt) || summary.Prompt != "" {
		t.Fatal("question evidence missing or leaked in summary")
	}
	svc.qualityRunner = manxueTestRunner(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, `{"id":"private-task-id","benchmark":"pelican","status":"running"}`)
		} else {
			fmt.Fprint(w, `{"id":"private-task-id","benchmark":"pelican","status":"succeeded","assessment":{"quality":"normal","reason":"good artwork"},"result":{"duration_ms":12000,"html":"<html><body><svg>pelican secret private-task-id</svg></body></html>"}}`)
		}
	})
	reader := svc.platformGroups.(fakePlatformGroupReader)
	reader.credByAccount["a"] = upstream.ProbeCredential{BaseURL: "https://gateway.example", Key: "secret"}
	svc.platformGroups = reader
	summary, err = svc.ProbeChannelQuality(ctx, "user", target, "two", "manxue_pelican")
	if err != nil {
		t.Fatal(err)
	}
	artwork, err := svc.QualityHistoryDetail(ctx, "user", target, summary.ID)
	if err != nil || !summary.HasHTML || summary.HTML != "" || artwork.HTML == "" || strings.Contains(artwork.HTML, "secret") || strings.Contains(artwork.HTML, "private-task-id") {
		t.Fatal("artwork not saved privately or summary too large")
	}
	states, _ := repo.ListQualityStates(ctx, "user", "ws1")
	if states[0].Latest.HTML != "" || !states[0].Latest.HasHTML {
		t.Fatal("state retains large HTML")
	}
	// Changing the saved method must not remove prior artwork and prompts.
	q, _ := svc.QualityConfiguration(ctx, "user")
	q.Model = "another model"
	_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
	artwork, _ = svc.QualityHistoryDetail(ctx, "user", target, summary.ID)
	if artwork.HTML == "" {
		t.Fatal("config change removed historical artwork")
	}
}

func TestQualityPostgresArtworkDetailAndHistoryTrimming(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	repo := NewRepository(pool)
	q := defaultQualitySettings()
	q.Enabled, q.Revision, q.HistoryLimit = true, "v1", 1
	_ = repo.SaveQualitySettings(ctx, "u", "w", q)
	_ = repo.SetQualityGroup(ctx, "u", "w", "g", true)
	started := time.Now()
	sample := QualitySample{ID: "artwork-1", TargetID: "target", StartedAt: &started, CreatedAt: started, HTML: "<html>retained artwork</html>", Prompt: "original question", Answer: strings.Repeat("x", 10000), Result: "passed"}
	state := QualityState{TargetID: sample.TargetID, Revision: q.Revision, Latest: sample}
	if ok, err := repo.SaveQualityResult(ctx, "u", "w", []string{"g"}, q, state); err != nil || !ok {
		t.Fatal(err)
	}
	restarted := NewRepository(pool)
	detail, err := restarted.GetQualitySample(ctx, "u", "w", "target", sample.ID)
	if err != nil || detail == nil || detail.HTML != sample.HTML || !detail.HasHTML || detail.Prompt != sample.Prompt || detail.Answer != sample.Answer {
		t.Fatal("detail not persisted")
	}
	list, err := restarted.ListQualityHistory(ctx, "u", "w", []string{"target"}, 1000)
	if err != nil || len(list) != 1 || list[0].HTML != "" || list[0].Prompt != "" || !list[0].HasHTML || len(list[0].Answer) != 4000 {
		t.Fatal("history list includes heavy fields")
	}
	states, err := restarted.ListQualityStates(ctx, "u", "w")
	if err != nil || len(states) != 1 || states[0].Latest.HTML != "" || states[0].Latest.Prompt != "" || len(states[0].Latest.Answer) != 4000 {
		t.Fatal("state not summarized")
	}
	for _, scope := range [][3]string{{"other", "w", "target"}, {"u", "other", "target"}, {"u", "w", "other"}} {
		if result, err := restarted.GetQualitySample(ctx, scope[0], scope[1], scope[2], sample.ID); err != nil || result != nil {
			t.Fatal("history detail crossed scope")
		}
	}
	state.Latest.ID = "artwork-2"
	state.Latest.CreatedAt = started.Add(time.Minute)
	if ok, err := repo.SaveQualityResult(ctx, "u", "w", []string{"g"}, q, state); err != nil || !ok {
		t.Fatal(err)
	}
	if result, err := restarted.GetQualitySample(ctx, "u", "w", "target", sample.ID); err != nil || result != nil {
		t.Fatal("trimmed history left old artwork")
	}
}

func TestManxueArtworkSizeBoundAndUnknownVerdict(t *testing.T) {
	for _, size := range []int{30, maxQualityHTMLBytes + 1} {
		var task manxueTask
		raw, _ := json.Marshal(map[string]any{"id": "private-task", "result": map[string]any{"html": strings.Repeat("x", size)}, "assessment": map[string]string{"quality": "unknown"}})
		_ = json.Unmarshal(raw, &task)
		result := manxueVerdict(task, "pelican", "key")
		if result.ErrorKey != qualityPrefix+"manxueUnknown" || (size > maxQualityHTMLBytes && (result.HTML != "" || !result.HTMLTooLarge)) || (size < maxQualityHTMLBytes && len(result.HTML) != size) {
			t.Fatal("size limit or unknown verdict lost artwork")
		}
	}
}
