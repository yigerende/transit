package connection_health

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"transithub/backend/internal/modules/my_sites"
	"transithub/backend/internal/modules/upstream"
)

type latencyTestTransport func(*http.Request) (*http.Response, error)

func (f latencyTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func latencyCheckingRunner(t *testing.T, wantMs int) *RealProbeRunner {
	t.Helper()
	runner := NewRealProbeRunner()
	if runner.client.Timeout != 0 {
		t.Fatal("a shared client timeout would cap longer policy deadlines")
	}
	runner.client.Transport = latencyTestTransport(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		remaining := time.Until(deadline)
		want := time.Duration(wantMs) * time.Millisecond
		if !ok || remaining > want || remaining < want-time.Second {
			t.Errorf("request deadline = %s, want approximately %s", remaining, want)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"choices":[]}`))}, nil
	})
	return runner
}

func TestProbeLatencyDefaultAndCustomDeadline(t *testing.T) {
	for _, milliseconds := range []int{0, 15000, 20000, 45000, 120000} {
		want := milliseconds
		if want == 0 {
			want = 20000
		}
		runner := latencyCheckingRunner(t, want)
		result := runner.Probe(context.Background(), ProbeRequest{BaseURL: "http://probe.test", MaxLatencyMs: milliseconds})
		if result.Result != ResultOK {
			t.Fatalf("custom deadline %d failed: %+v", milliseconds, result)
		}
	}
}

func TestProbeLatencyIncludesBodyAndTimesOutWhileReading(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
			return
		case <-time.After(100 * time.Millisecond):
			_, _ = io.WriteString(w, `{"choices":[]}`)
		}
	}))
	defer server.Close()
	runner := NewRealProbeRunner()
	// Concurrent requests on the same runner must keep separate policy deadlines.
	results := make(chan ProbeOutcome, 2)
	go func() {
		results <- runner.Probe(context.Background(), ProbeRequest{BaseURL: server.URL, MaxLatencyMs: 30})
	}()
	go func() {
		results <- runner.Probe(context.Background(), ProbeRequest{BaseURL: server.URL, MaxLatencyMs: 2000})
	}()
	seen := make(map[ResultKey]ProbeOutcome)
	for i := 0; i < 2; i++ {
		outcome := <-results
		seen[outcome.Result] = outcome
	}
	if success, ok := seen[ResultOK]; !ok || success.LatencyMs < 90 {
		t.Fatalf("latency must include the response body: %+v", seen)
	}
	if timeout, ok := seen[ResultNetworkFluctuation]; !ok || timeout.LatencyMs < 25 {
		t.Fatalf("body timeout must be a transport failure with elapsed time: %+v", seen)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result := runner.Probe(ctx, ProbeRequest{BaseURL: server.URL, MaxLatencyMs: 45000}); result.Result != ResultNetworkFluctuation {
		t.Fatal("longer policy deadline must still respect parent cancellation")
	}
}

func TestPolicyLatencyReachesBothProbePaths(t *testing.T) {
	repo := newFakeRepository()
	policy := probePolicy()
	policy.MaxLatencyMs = 45000
	repo.policies = []Policy{policy}
	service := &Service{
		repo: repo, probeRunner: latencyCheckingRunner(t, 45000), dispatcher: noopRemoteActionRunner{},
		sites: fakeSiteLookup{site: &upstream.Site{ID: "site", BaseURL: "http://probe.test"}},
	}
	ctx := context.Background()
	if _, err := service.probeOnce(ctx, my_sites.RealConnection{ID: "connection", UpstreamSiteID: "site", UserID: "user1", WorkspaceAdminAccountID: "ws1"}, policy, policy.ModelTargets[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.probeTargetOnce(ctx, "user1", "ws1", AdminProbeTarget{TargetID: "newapi:ws1:100"}, upstream.ProbeCredential{BaseURL: "http://probe.test"}, probeModelSpec{modelName: "gpt-4o", policy: policy}); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyLatencyDefaultsAndPreservesOldClientUpdates(t *testing.T) {
	ctx := context.Background()
	service, repo, edit := groupPolicyEditFixture()
	created, err := service.SavePolicy(ctx, "user1", PolicyInput{Name: "new"})
	if err != nil || created.MaxLatencyMs != 20000 {
		t.Fatalf("new policy must default to 20000ms: %+v %v", created, err)
	}
	repo.policies[0].MaxLatencyMs = 45000
	saved, err := service.SavePolicy(ctx, "user1", edit)
	if err != nil || saved.MaxLatencyMs != 45000 {
		t.Fatalf("old client update reset latency: %+v %v", saved, err)
	}
	if _, err := service.SetAdminGroupPolicyConfiguration(ctx, "user1", "g1", AdminGroupPolicyConfigurationInput{EditPolicy: &edit}); err != nil {
		t.Fatal(err)
	}
	if repo.policies[0].MaxLatencyMs != 45000 {
		t.Fatal("group editor reset latency on an omitted field")
	}
	edit.MaxLatencyMs = intPtr(60000)
	if _, err := service.SetAdminGroupPolicyConfiguration(ctx, "user1", "g1", AdminGroupPolicyConfigurationInput{EditPolicy: &edit}); err != nil {
		t.Fatal(err)
	}
	if repo.policies[0].MaxLatencyMs != 60000 {
		t.Fatal("group editor lost custom latency")
	}
	for _, invalid := range []int{0, -1, 2147483648} {
		edit.MaxLatencyMs = &invalid
		if _, err := service.SavePolicy(ctx, "user1", edit); err == nil || err.Error() != ErrorMaxLatencyInvalid {
			t.Fatalf("invalid latency %d was not rejected: %v", invalid, err)
		}
		if repo.policies[0].MaxLatencyMs != 60000 {
			t.Fatal("invalid input changed the saved policy")
		}
	}
}
