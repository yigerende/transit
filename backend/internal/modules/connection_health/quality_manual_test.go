package connection_health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"transithub/backend/internal/shared/authctx"
)

func enableManualQuality(t *testing.T, svc *Service) QualitySettings {
	t.Helper()
	q := defaultQualitySettings()
	q.Enabled = true
	q, err := svc.SaveQualityConfiguration(context.Background(), "user", q)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range []string{"one", "two"} {
		if _, err := svc.SetGroupQuality(context.Background(), "user", group, true); err != nil {
			t.Fatal(err)
		}
	}
	return q
}

func TestManualQualityRunsBeforeDueAndRotatesSharedHistory(t *testing.T) {
	svc, repo, health, runner := qualityTestService(t)
	q := enableManualQuality(t, svc)
	ctx := context.Background()
	const target = "sub2api:ws1:a"
	for i, group := range []string{"one", "two"} {
		sample, err := svc.ProbeChannelQuality(ctx, "user", target, group, "questions")
		if err != nil || sample.Result != "passed" || sample.QuestionID != q.Questions[i].ID {
			t.Fatalf("check %d: %+v, %v", i, sample, err)
		}
	}
	history, _ := repo.ListQualityHistory(ctx, "user", "ws1", []string{target}, 100)
	states, _ := repo.ListQualityStates(ctx, "user", "ws1")
	if runner.calls != 2 || len(history) != 2 || history[0].ID == history[1].ID || !states[0].NextProbeAt.After(time.Now()) {
		t.Fatal("manual check did not append evidence and update next due time")
	}
	svc.runQualityScope(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
	if runner.calls != 2 || len(health.events) != 0 || len(health.states) != 0 || len(health.budgetClaims) != 0 || len(health.targetActionStates) != 0 {
		t.Fatal("scheduled duplicate or change to normal health/remote actions")
	}
}

func TestManualQualityRejectsDisabledUnselectedOrForeignTargets(t *testing.T) {
	for _, scenario := range []string{"global-off", "group-off", "channel-off", "excluded", "unassigned", "wrong-group", "empty-group", "foreign-workspace", "wrong-platform", "missing-channel"} {
		t.Run(scenario, func(t *testing.T) {
			svc, repo, health, runner := qualityTestService(t)
			q := enableManualQuality(t, svc)
			ctx := context.Background()
			target, group := "sub2api:ws1:a", "one"
			switch scenario {
			case "global-off":
				q.Enabled = false
				_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
			case "group-off":
				_, _ = svc.SetGroupQuality(ctx, "user", group, false)
			case "channel-off":
				_, _ = svc.SetChannelQuality(ctx, "user", target, false)
			case "excluded":
				health.groupExclusions = []GroupTargetExclusion{{UserID: "user", AdminAccountID: "ws1", AdminGroupID: group, TargetID: target}}
			case "unassigned":
				health.groupAssignments = nil
			case "wrong-group":
				group = "off"
			case "empty-group":
				group = " "
			case "foreign-workspace":
				target = "sub2api:other:a"
			case "wrong-platform":
				target = "newapi:ws1:a"
			case "missing-channel":
				target = "sub2api:ws1:missing"
			}
			if _, err := svc.ProbeChannelQuality(ctx, "user", target, group, "questions"); err == nil {
				t.Fatal("manual check bypassed a gate")
			}
			if runner.calls != 0 || len(repo.history) != 0 {
				t.Fatal("rejected check consumed a request or wrote history")
			}
		})
	}
}

func TestManualAndScheduledQualityShareChannelLease(t *testing.T) {
	for _, first := range []string{"manual", "scheduled"} {
		t.Run(first, func(t *testing.T) {
			svc, _, _, runner := qualityTestService(t)
			enableManualQuality(t, svc)
			ctx := context.Background()
			started, resume, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			runner.hook = func() { close(started); <-resume }
			defer func() { close(resume); <-done }()
			scope := QualityScope{UserID: "user", WorkspaceID: "ws1"}
			go func() {
				defer close(done)
				if first == "manual" {
					_, _ = svc.ProbeChannelQuality(ctx, "user", "sub2api:ws1:a", "one", "questions")
				} else {
					svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
				}
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("check did not start")
			}
			_, err := svc.ProbeChannelQuality(ctx, "user", "sub2api:ws1:a", "two", "questions")
			if err != requestError(qualityPrefix+"probeBusy") {
				t.Fatalf("shared channel duplicate was not rejected: %v", err)
			}
			if first == "manual" {
				svc.runQualityScope(ctx, scope, make(chan struct{}, 32))
			}
			runner.mu.Lock()
			calls := runner.calls
			runner.mu.Unlock()
			if calls != 1 {
				t.Fatal("duplicate billable check")
			}
		})
	}
}

func TestManualQualityDiscardsResultAfterSettingsChange(t *testing.T) {
	svc, repo, _, runner := qualityTestService(t)
	q := enableManualQuality(t, svc)
	runner.hook = func() {
		q.Model = "changed-model"
		_, _ = svc.SaveQualityConfiguration(context.Background(), "user", q)
	}
	if _, err := svc.ProbeChannelQuality(context.Background(), "user", "sub2api:ws1:a", "one", "questions"); err == nil || len(repo.history) != 0 {
		t.Fatal("stale result reported as saved")
	}
}

func TestManualQualityOverridesDetectorWithoutChangingSavedConfiguration(t *testing.T) {
	svc, repo, _, _ := qualityTestService(t)
	saved := enableManualQuality(t, svc)
	ctx := context.Background()
	const target = "sub2api:ws1:a"
	if _, err := svc.ProbeChannelQuality(ctx, "user", target, "one", "questions"); err != nil {
		t.Fatal(err)
	}
	runner := &manxueServiceRunner{}
	svc.qualityRunner = runner
	for _, benchmark := range []string{"candy", "pelican"} {
		sample, err := svc.ProbeChannelQuality(ctx, "user", target, "one", "manxue_"+benchmark)
		if err != nil || sample.DetectionMethod != qualityMethodManxue || sample.Benchmark != benchmark || sample.Result != "failed" || sample.Report == "" {
			t.Fatalf("manual API check lost selected method/result: %+v %v", sample, err)
		}
	}
	config, _ := repo.GetQualitySettings(ctx, "user", "ws1")
	states, _ := repo.ListQualityStates(ctx, "user", "ws1")
	if config.DetectionMethod != saved.DetectionMethod || config.ManxueBenchmark != saved.ManxueBenchmark || config.Revision != saved.Revision || states[0].NextQuestionID != saved.Questions[1].ID || runner.calls != 2 {
		t.Fatal("manual benchmarks changed saved config or rewound custom questions")
	}
	if _, err := svc.ProbeChannelQuality(ctx, "user", target, "one", "invalid"); err == nil {
		t.Fatal("invalid method accepted")
	}
	// Saved API mode may retain a question bank; the custom button must use it.
	config.DetectionMethod = qualityMethodManxue
	_, _ = svc.SaveQualityConfiguration(ctx, "user", config)
	svc.qualityRunner = &fakeQuestionRunner{}
	sample, err := svc.ProbeChannelQuality(ctx, "user", target, "one", "questions")
	if err != nil || sample.DetectionMethod != qualityMethodQuestions || sample.QuestionID != saved.Questions[0].ID {
		t.Fatal("custom override did not use saved questions")
	}
	config.Questions = nil
	_, _ = svc.SaveQualityConfiguration(ctx, "user", config)
	if _, err := svc.ProbeChannelQuality(ctx, "user", target, "one", "questions"); err == nil {
		t.Fatal("custom check accepted without a question bank")
	}
}

func TestManualQualityHTTPRequiresAuthAndReturnsSavedSample(t *testing.T) {
	svc, _, _, runner := qualityTestService(t)
	enableManualQuality(t, svc)
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc)
	for _, authenticated := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, "/api/connection-health/targets/sub2api:ws1:a/quality-probe", strings.NewReader(`{"groupId":"one","method":"questions"}`))
		if authenticated {
			req = req.WithContext(authctx.WithUserID(req.Context(), "user"))
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if !authenticated {
			if w.Code != http.StatusUnauthorized || runner.calls != 0 {
				t.Fatal("unauthenticated check accepted")
			}
			continue
		}
		var sample QualitySample
		if json.Unmarshal(w.Body.Bytes(), &sample) != nil || w.Code != http.StatusOK || sample.ID == "" || sample.Result != "passed" || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("invalid manual response: %s", w.Body.String())
		}
	}
}

func TestQualityPostgresLeaseIsSharedAndReleased(t *testing.T) {
	ctx, pool := qualityTestPool(t)
	repo, other := NewRepository(pool), NewRepository(pool)
	release, acquired, err := repo.TryAcquireQualityLease(ctx, "user", "ws", "target")
	if err != nil || !acquired {
		t.Fatalf("cannot acquire lease: %v", err)
	}
	defer release()
	if _, acquired, err := other.TryAcquireQualityLease(ctx, "user", "ws", "target"); err != nil || acquired {
		t.Fatalf("busy channel was not excluded: %v", err)
	}
	for _, target := range []string{"other-target", "target"} {
		if target == "target" {
			release()
		}
		unlock, acquired, err := other.TryAcquireQualityLease(ctx, "user", "ws", target)
		if err != nil || !acquired {
			t.Fatalf("independent/released channel blocked: %v", err)
		}
		unlock()
	}
}
