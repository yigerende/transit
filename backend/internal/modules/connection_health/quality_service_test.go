package connection_health

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
	"transithub/backend/internal/modules/upstream"
)

type fakeQualityRepo struct {
	mu      sync.Mutex
	configs map[string]QualitySettings
	groups  map[string]map[string]bool
	states  map[string]map[string]QualityState
	history map[string][]QualitySample
}

func newFakeQualityRepo() *fakeQualityRepo {
	return &fakeQualityRepo{configs: map[string]QualitySettings{}, groups: map[string]map[string]bool{}, states: map[string]map[string]QualityState{}, history: map[string][]QualitySample{}}
}
func qualityScopeKey(u, w string) string { return u + "|" + w }
func (r *fakeQualityRepo) GetQualitySettings(_ context.Context, u, w string) (QualitySettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	q, ok := r.configs[qualityScopeKey(u, w)]
	if !ok {
		return defaultQualitySettings(), nil
	}
	raw, _ := json.Marshal(q)
	_ = json.Unmarshal(raw, &q)
	return q, nil
}
func (r *fakeQualityRepo) SaveQualitySettings(_ context.Context, u, w string, q QualitySettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	raw, _ := json.Marshal(q)
	_ = json.Unmarshal(raw, &q)
	r.configs[qualityScopeKey(u, w)] = q
	return nil
}
func (r *fakeQualityRepo) ListQualityScopes(context.Context) ([]QualityScope, error) { return nil, nil }
func (r *fakeQualityRepo) ListQualityGroups(_ context.Context, u, w string) ([]QualityGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []QualityGroup{}
	for id, en := range r.groups[qualityScopeKey(u, w)] {
		out = append(out, QualityGroup{GroupID: id, Enabled: en})
	}
	return out, nil
}
func (r *fakeQualityRepo) SetQualityGroup(_ context.Context, u, w, g string, en bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := qualityScopeKey(u, w)
	if r.groups[key] == nil {
		r.groups[key] = map[string]bool{}
	}
	r.groups[key][g] = en
	return nil
}
func (r *fakeQualityRepo) ListQualityStates(_ context.Context, u, w string) ([]QualityState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []QualityState{}
	for _, st := range r.states[qualityScopeKey(u, w)] {
		out = append(out, st)
	}
	return out, nil
}
func (r *fakeQualityRepo) SaveQualityResult(_ context.Context, u, w string, groups []string, q QualitySettings, st QualityState) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := qualityScopeKey(u, w)
	current := r.configs[key]
	if !current.Enabled || current.Revision != q.Revision {
		return false, nil
	}
	allowed := false
	for _, g := range groups {
		if r.groups[key][g] {
			allowed = true
		}
	}
	if !allowed {
		return false, nil
	}
	if r.states[key] == nil {
		r.states[key] = map[string]QualityState{}
	}
	r.states[key][st.TargetID] = st
	r.history[key] = append(r.history[key], st.Latest)
	return true, nil
}
func (r *fakeQualityRepo) ListQualityHistory(_ context.Context, u, w string, targets []string, limit int) ([]QualitySample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []QualitySample{}
	for _, sample := range r.history[qualityScopeKey(u, w)] {
		for _, id := range targets {
			if sample.TargetID == id {
				out = append(out, sample)
			}
		}
	}
	return out, nil
}

type fakeQuestionRunner struct {
	mu                sync.Mutex
	calls             int
	active, maxActive int
	hook              func()
	delay             time.Duration
}

func (r *fakeQuestionRunner) ProbeQuality(_ context.Context, _ upstream.ProbeCredential, _ string, _ QualitySettings, q QualityQuestion) qualityProbeResult {
	r.mu.Lock()
	r.calls++
	r.active++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.mu.Unlock()
	if r.hook != nil {
		r.hook()
	}
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	return qualityProbeResult{Answer: q.Answer, DurationMS: 10}
}

func qualityTestService(t *testing.T) (*Service, *fakeQualityRepo, *fakeRepository, *fakeQuestionRunner) {
	t.Helper()
	health := newFakeRepository()
	quality := newFakeQualityRepo()
	runner := &fakeQuestionRunner{}
	account := upstream.AdminGroupAccountInfo{ID: "a", Name: "channel", Platform: ProviderOpenAI}
	reader := fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "one", Name: "one"}, {ID: "two", Name: "two"}, {ID: "off", Name: "off"}}, accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{"one": {account}, "two": {account}, "off": {{ID: "other"}}}, credByAccount: map[string]upstream.ProbeCredential{"a": {BaseURL: "http://unused.invalid", Key: "secret"}}}
	svc := newAdminGroupsService(reader, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API}}, health)
	svc.repo = &serializedGroupProbeRepo{fakeRepository: health}
	svc.qualityRepo = quality
	svc.qualityRunner = runner
	svc.dispatcher = panicIfCalledRemoteActionRunner{}
	return svc, quality, health, runner
}

func TestQualitySettingsAndGroupOptIn(t *testing.T) {
	svc, repo, _, _ := qualityTestService(t)
	ctx := context.Background()
	if _, err := svc.SetGroupQuality(ctx, "user", "one", true); err == nil {
		t.Fatal("enabled group before global config")
	}
	q := defaultQualitySettings()
	q.Enabled = true
	saved, err := svc.SaveQualityConfiguration(ctx, "user", q)
	if err != nil || saved.Revision == "" {
		t.Fatal(err)
	}
	unchanged, err := svc.SaveQualityConfiguration(ctx, "user", saved)
	if err != nil || unchanged.Revision != saved.Revision {
		t.Fatal("identical save reset evidence")
	}
	saved.Model = "other-model"
	changed, err := svc.SaveQualityConfiguration(ctx, "user", saved)
	if err != nil || changed.Revision == saved.Revision {
		t.Fatal("changed settings kept old revision")
	}
	if _, err := svc.SetGroupQuality(ctx, "user", "missing", true); err == nil {
		t.Fatal("foreign group enabled")
	}
	if _, err := svc.SetGroupQuality(ctx, "user", "one", true); err != nil {
		t.Fatal(err)
	}
	svc.mySites = qualityOfflineSites{}
	if _, err := svc.SetGroupQuality(ctx, "user", "one", false); err != nil {
		t.Fatal("pause requires online upstream")
	}
	other, _ := repo.GetQualitySettings(ctx, "other-user", "ws1")
	if other.Enabled || other.Revision != "" {
		t.Fatal("configuration leaked across users")
	}
}

type qualityOfflineSites struct{ fakeMySitesReader }

func (qualityOfflineSites) RequireSession(context.Context, string, string) (upstream.Session, error) {
	return upstream.Session{}, errors.New("offline")
}

func TestQualityScheduleDeduplicatesAndNeverChangesHealth(t *testing.T) {
	svc, repo, health, runner := qualityTestService(t)
	ctx := context.Background()
	q := defaultQualitySettings()
	q.Enabled = true
	q, err := svc.SaveQualityConfiguration(ctx, "user", q)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []string{"one", "two"} {
		if _, err := svc.SetGroupQuality(ctx, "user", g, true); err != nil {
			t.Fatal(err)
		}
	}
	// Changing the selected workspace must not redirect saved background tasks.
	svc.accounts = fakeAdminAccountResolver{id: "elsewhere"}
	scope := QualityScope{UserID: "user", WorkspaceID: "ws1", Settings: q}
	tokens := make(chan struct{}, 32)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); svc.runQualityScope(ctx, scope, tokens) }()
	}
	wg.Wait()
	if runner.calls != 1 {
		t.Fatalf("shared channel / multi-instance duplicate: %d", runner.calls)
	}
	states, _ := repo.ListQualityStates(ctx, "user", "ws1")
	if len(states) != 1 || states[0].Latest.Result != "passed" || states[0].NextQuestionID != q.Questions[1].ID {
		t.Fatal("result or question rotation not persisted")
	}
	if len(health.events) != 0 || len(health.states) != 0 || len(health.budgetClaims) != 0 || len(health.targetActionStates) != 0 {
		t.Fatal("quality touched normal health machinery")
	}
	for _, g := range []string{"one", "two"} {
		_ = repo.SetQualityGroup(ctx, "user", "ws1", g, false)
	}
	svc.runQualityScope(ctx, scope, tokens)
	if runner.calls != 1 {
		t.Fatal("disabled groups still probed")
	}
	// Restart with the same repository and due state resumes the persisted task.
	_ = repo.SetQualityGroup(ctx, "user", "ws1", "one", true)
	repo.mu.Lock()
	st := states[0]
	st.NextProbeAt = time.Now().Add(-time.Second)
	repo.states[qualityScopeKey("user", "ws1")][st.TargetID] = st
	repo.mu.Unlock()
	restarted := *svc
	restarted.runQualityScope(ctx, scope, tokens)
	if runner.calls != 2 {
		t.Fatal("saved task did not resume")
	}
}

func TestQualityDiscardInFlightAfterDisableOrConfigChange(t *testing.T) {
	for _, change := range []string{"group", "config"} {
		t.Run(change, func(t *testing.T) {
			svc, repo, _, runner := qualityTestService(t)
			ctx := context.Background()
			q := defaultQualitySettings()
			q.Enabled = true
			q, _ = svc.SaveQualityConfiguration(ctx, "user", q)
			_, _ = svc.SetGroupQuality(ctx, "user", "one", true)
			runner.hook = func() {
				if change == "group" {
					_ = repo.SetQualityGroup(ctx, "user", "ws1", "one", false)
				} else {
					newConfig := q
					newConfig.Model = "changed"
					_, _ = svc.SaveQualityConfiguration(ctx, "user", newConfig)
				}
			}
			svc.runQualityScope(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
			states, _ := repo.ListQualityStates(ctx, "user", "ws1")
			if runner.calls != 1 || len(states) != 0 {
				t.Fatal("stale in-flight result became current evidence")
			}
		})
	}
}

func TestQualityConcurrencyAndMovedChannel(t *testing.T) {
	svc, _, _, runner := qualityTestService(t)
	ctx := context.Background()
	q := defaultQualitySettings()
	q.Enabled = true
	q.Concurrency = 2
	_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
	_, _ = svc.SetGroupQuality(ctx, "user", "one", true)
	reader := svc.platformGroups.(fakePlatformGroupReader)
	reader.accountsByGrp["one"] = []upstream.AdminGroupAccountInfo{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
	svc.platformGroups = reader
	runner.delay = 20 * time.Millisecond
	svc.runQualityScope(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
	if runner.calls != 4 || runner.maxActive > 2 || runner.maxActive < 2 {
		t.Fatalf("configured concurrency not respected: calls=%d max=%d", runner.calls, runner.maxActive)
	}
	before := runner.calls
	// A queued candidate whose channel has left its group cannot be probed.
	reader.accountsByGrp["one"] = nil
	svc.platformGroups = reader
	config, _ := svc.QualityConfiguration(ctx, "user")
	svc.runQualityCandidate(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, upstream.Session{Platform: upstream.PlatformSub2API}, config, qualityCandidate{account: upstream.AdminGroupAccountInfo{ID: "a"}, groups: []string{"one"}, targetID: "sub2api:ws1:a"}, QualityState{})
	if runner.calls != before {
		t.Fatal("moved channel was probed")
	}
}
