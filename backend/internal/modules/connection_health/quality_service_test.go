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
	mu       sync.Mutex
	leases   map[string]bool
	configs  map[string]QualitySettings
	groups   map[string]map[string]bool
	channels map[string]map[string]bool
	states   map[string]map[string]QualityState
	history  map[string][]QualitySample
}

func (r *fakeQualityRepo) TryAcquireQualityLease(_ context.Context, user, workspace, target string) (func(), bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.leases == nil {
		r.leases = map[string]bool{}
	}
	key := user + "|" + workspace + "|" + target
	if r.leases[key] {
		return nil, false, nil
	}
	r.leases[key] = true
	return func() { r.mu.Lock(); delete(r.leases, key); r.mu.Unlock() }, true, nil
}

func newFakeQualityRepo() *fakeQualityRepo {
	return &fakeQualityRepo{configs: map[string]QualitySettings{}, channels: map[string]map[string]bool{}, groups: map[string]map[string]bool{}, states: map[string]map[string]QualityState{}, history: map[string][]QualitySample{}}
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
func (r *fakeQualityRepo) ListQualityChannels(_ context.Context, u, w string) ([]QualityChannel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []QualityChannel{}
	for id, enabled := range r.channels[qualityScopeKey(u, w)] {
		out = append(out, QualityChannel{TargetID: id, Enabled: enabled})
	}
	return out, nil
}
func (r *fakeQualityRepo) SetQualityChannel(_ context.Context, u, w, target string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := qualityScopeKey(u, w)
	if r.channels[key] == nil {
		r.channels[key] = map[string]bool{}
	}
	r.channels[key][target] = enabled
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
	if enabled, exists := r.channels[key][st.TargetID]; exists && !enabled {
		return false, nil
	}
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
	r.history[key] = append(r.history[key], st.Latest)
	st.Latest = qualitySampleSummary(st.Latest)
	r.states[key][st.TargetID] = st
	return true, nil
}
func (r *fakeQualityRepo) ListQualityHistory(_ context.Context, u, w string, targets []string, limit int) ([]QualitySample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []QualitySample{}
	for _, sample := range r.history[qualityScopeKey(u, w)] {
		for _, id := range targets {
			if sample.TargetID == id {
				out = append(out, qualitySampleSummary(sample))
			}
		}
	}
	return out, nil
}

func (r *fakeQualityRepo) GetQualitySample(_ context.Context, user, workspace, target, id string) (*QualitySample, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, sample := range r.history[qualityScopeKey(user, workspace)] {
		if sample.TargetID == target && sample.ID == id {
			return &sample, nil
		}
	}
	return nil, nil
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
	health.policies = []Policy{{ID: "selected-policy", UserID: "user", AdminAccountID: "ws1", Enabled: true}}
	health.groupAssignments = []GroupPolicyAssignment{
		{UserID: "user", AdminAccountID: "ws1", AdminGroupID: "one", PolicyID: "selected-policy"},
		{UserID: "user", AdminAccountID: "ws1", AdminGroupID: "two", PolicyID: "selected-policy"},
	}
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
	svc.runQualityCandidate(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, upstream.Session{Platform: upstream.PlatformSub2API}, config, qualityCandidate{account: upstream.AdminGroupAccountInfo{ID: "a"}, groups: []string{"one"}, targetID: "sub2api:ws1:a"}, true)
	if runner.calls != before {
		t.Fatal("moved channel was probed")
	}
}

func TestQualityUsesSavedAutomationChannelSelection(t *testing.T) {
	for _, scenario := range []string{"selected", "excluded", "no-assignment", "deleted-policy", "foreign-assignment", "foreign-exclusion", "explicit-assignment", "excluded-explicit", "selected-disabled-policy", "shared-selected", "shared-excluded"} {
		t.Run(scenario, func(t *testing.T) {
			svc, quality, health, runner := qualityTestService(t)
			ctx := context.Background()
			q := defaultQualitySettings()
			q.Enabled = true
			_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
			_, _ = svc.SetGroupQuality(ctx, "user", "one", true)
			exclusion := GroupTargetExclusion{UserID: "user", AdminAccountID: "ws1", AdminGroupID: "one", TargetID: "sub2api:ws1:a"}
			want := 1
			switch scenario {
			case "excluded":
				health.groupExclusions = []GroupTargetExclusion{exclusion}
				want = 0
			case "no-assignment":
				health.groupAssignments = nil
				want = 0
			case "deleted-policy":
				health.policies = nil
				want = 0
			case "foreign-assignment":
				for i := range health.groupAssignments {
					health.groupAssignments[i].AdminAccountID = "other"
				}
				want = 0
			case "foreign-exclusion":
				exclusion.UserID = "other"
				health.groupExclusions = []GroupTargetExclusion{exclusion}
			case "explicit-assignment", "excluded-explicit":
				health.groupAssignments = nil
				health.assignments = []PolicyAssignment{{UserID: "user", AdminAccountID: "ws1", TargetID: "sub2api:ws1:a", PolicyID: "selected-policy"}}
				if scenario == "excluded-explicit" {
					health.groupExclusions = []GroupTargetExclusion{exclusion}
					want = 0
				}
			case "selected-disabled-policy":
				// The quality switch controls execution; automation supplies the selection.
				health.policies[0].Enabled = false
			case "shared-selected", "shared-excluded":
				_, _ = svc.SetGroupQuality(ctx, "user", "two", true)
				health.groupExclusions = []GroupTargetExclusion{exclusion}
				if scenario == "shared-excluded" {
					exclusion.AdminGroupID = "two"
					health.groupExclusions = append(health.groupExclusions, exclusion)
					want = 0
				}
			}
			svc.runQualityScope(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
			if runner.calls != want || len(quality.history[qualityScopeKey("user", "ws1")]) != want {
				t.Fatalf("unselected channel executed: calls=%d want=%d", runner.calls, want)
			}
			groups, err := svc.AdminGroups(ctx, "user")
			if err != nil {
				t.Fatal(err)
			}
			for _, group := range groups {
				if group.ID == "one" {
					if len(group.Accounts) != 1 {
						t.Fatal("channel inventory changed")
					}
					selected := want == 1 && scenario != "shared-selected"
					if group.Accounts[0].QualitySelected != selected {
						t.Fatal("displayed selection differs from the scheduler")
					}
				}
			}
		})
	}
}

func TestQualityRechecksSelectionForQueuedAndInFlightChannels(t *testing.T) {
	for _, scenario := range []string{"queued", "in-flight"} {
		t.Run(scenario, func(t *testing.T) {
			svc, quality, health, runner := qualityTestService(t)
			ctx := context.Background()
			q := defaultQualitySettings()
			q.Enabled = true
			q.Concurrency = 1
			_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
			_, _ = svc.SetGroupQuality(ctx, "user", "one", true)
			target := "sub2api:ws1:a"
			if scenario == "queued" {
				reader := svc.platformGroups.(fakePlatformGroupReader)
				reader.accountsByGrp["one"] = append(reader.accountsByGrp["one"], upstream.AdminGroupAccountInfo{ID: "b"})
				reader.credByAccount["b"] = reader.credByAccount["a"]
				svc.platformGroups = reader
				target = "sub2api:ws1:b"
			}
			runner.hook = func() {
				health.groupExclusions = []GroupTargetExclusion{{UserID: "user", AdminAccountID: "ws1", AdminGroupID: "one", TargetID: target}}
			}
			svc.runQualityScope(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
			if runner.calls != 1 {
				t.Fatal("deselected queued channel was requested")
			}
			for _, sample := range quality.history[qualityScopeKey("user", "ws1")] {
				if sample.TargetID == target {
					t.Fatal("deselected channel appended an in-flight result")
				}
			}
		})
	}
}

type qualitySelectionErrorRepo struct{ healthRepository }

func (r qualitySelectionErrorRepo) ListGroupTargetExclusionsByWorkspace(context.Context, string, string) ([]GroupTargetExclusion, error) {
	return nil, errors.New("selection unavailable")
}

func TestQualityDoesNotProbeWhenChannelSelectionCannotBeRead(t *testing.T) {
	svc, quality, _, runner := qualityTestService(t)
	ctx := context.Background()
	q := defaultQualitySettings()
	q.Enabled = true
	_, _ = svc.SaveQualityConfiguration(ctx, "user", q)
	_, _ = svc.SetGroupQuality(ctx, "user", "one", true)
	svc.repo = qualitySelectionErrorRepo{svc.repo}
	svc.runQualityScope(ctx, QualityScope{UserID: "user", WorkspaceID: "ws1"}, make(chan struct{}, 32))
	if runner.calls != 0 || len(quality.history) != 0 {
		t.Fatal("unknown channel selection must not allow probing")
	}
}
