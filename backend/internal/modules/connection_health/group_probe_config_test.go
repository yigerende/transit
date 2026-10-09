package connection_health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
	"transithub/backend/internal/shared/authctx"
)

func TestGroupProbeHTTPBlankKeyPreservesSavedKey(t *testing.T) {
	svc, repo, provider := groupConfigTestService(t)
	modelRequests := 0
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer group-secret" {
			t.Error("model discovery did not use the saved key")
		}
		modelRequests++
		_, _ = w.Write([]byte(`{"data":[{"id":"model"}]}`))
	}))
	defer modelServer.Close()
	session := svc.mySites.(fakeMySitesReader).session
	session.BaseURL = modelServer.URL
	svc.mySites = fakeMySitesReader{session: session}
	svc.modelDiscovery = NewModelDiscoveryRunner()
	mux := http.NewServeMux()
	RegisterRoutes(mux, svc)
	request := func(method, action, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/api/connection-health/admin-groups/42/"+action, strings.NewReader(body))
		req = req.WithContext(authctx.WithUserID(req.Context(), "user1"))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s %s returned %d: %s", method, action, w.Code, w.Body.String())
		}
		return w
	}
	request("PUT", "probe-config", `{"model":"model","enabled":true,"key":"group-secret"}`)
	for _, keyJSON := range []string{"", `,"key":null`, `,"key":""`, `,"key":"  "`} {
		for _, enabled := range []string{"true", "false"} {
			// Reopen and save with an empty password field, including older clients
			// that submit an empty string instead of omitting it.
			loaded := request("GET", "probe-config", "")
			var config GroupProbeConfig
			if err := json.Unmarshal(loaded.Body.Bytes(), &config); err != nil || !config.HasCustomKey {
				t.Fatal("reopened configuration lost its saved key indicator")
			}
			request("PUT", "probe-config", `{"model":"other-model","intervalSeconds":120,"enabled":`+enabled+keyJSON+`}`)
			request("POST", "prepare-probe", `{"useAutoKey":false`+keyJSON+`}`)
			stored, _ := repo.GetGroupProbeConfig(context.Background(), "user1", "ws1", "42")
			if stored.CustomKeyID != "key-7" || stored.IntervalSeconds != 120 || provider.calls != 0 {
				t.Fatal("resaving or fetching models discarded the saved key or created an automatic key")
			}
		}
	}
	if modelRequests != 8 {
		t.Fatalf("got %d model requests, want 8", modelRequests)
	}
	// Only the explicit button flag can switch the credential source.
	request("POST", "prepare-probe", `{"useAutoKey":true}`)
	stored, _ := repo.GetGroupProbeConfig(context.Background(), "user1", "ws1", "42")
	if provider.calls != 1 || stored.CustomKeyID != "key-7" {
		t.Fatal("previewing automatic mode must not alter the saved configuration")
	}
	request("PUT", "probe-config", `{"model":"model","enabled":true,"useAutoKey":true}`)
	stored, _ = repo.GetGroupProbeConfig(context.Background(), "user1", "ws1", "42")
	if provider.calls != 2 || stored.HasCustomKey || stored.CustomKeyID != "" {
		t.Fatal("explicit automatic selection did not persist")
	}
}

func TestGroupProbeCustomKeyPersistsReferenceAndDoesNotCreateFallback(t *testing.T) {
	svc, repo, provider := groupConfigTestService(t)
	ctx := context.Background()
	// A supplied key also works with an admin API key session, without a user JWT.
	session := svc.mySites.(fakeMySitesReader).session
	session.AccessToken = ""
	session.AdminAPIKey = "admin-secret"
	svc.mySites = fakeMySitesReader{session: session}
	key := " group-secret "
	input := GroupProbeConfigInput{Model: "model", Enabled: true, Key: &key}
	c, err := svc.SaveGroupProbeConfiguration(ctx, "user1", "42", input)
	if err != nil || !c.HasCustomKey || c.CustomKeyID != "key-7" || provider.calls != 0 {
		t.Fatalf("custom save failed: %+v %v", c, err)
	}
	encoded, _ := json.Marshal(c)
	if strings.Contains(string(encoded), "group-secret") || strings.Contains(string(encoded), "key-7") {
		t.Fatal("key information leaked in response")
	}
	if stored := repo.groupProbeConfigs[configTestKey("user1", "ws1", "42")]; stored.CustomKeyID != "key-7" {
		t.Fatal("key reference was not persisted")
	}
	input.Key = nil
	c, err = svc.SaveGroupProbeConfiguration(ctx, "user1", "42", input)
	if err != nil || c.CustomKeyID != "key-7" {
		t.Fatal("saving interval discarded the saved key")
	}
	for _, blank := range []string{"", " \t "} {
		input.Key = &blank
		input.IntervalSeconds = 120
		c, err = svc.SaveGroupProbeConfiguration(ctx, "user1", "42", input)
		if err != nil || !c.HasCustomKey || c.CustomKeyID != "key-7" || provider.calls != 0 {
			t.Fatalf("blank key must retain the saved key without automatic creation: %+v %v", c, err)
		}
	}
	restarted := *svc
	restarted.runScheduledGroupProbe(ctx, c)
	if len(repo.events) != 1 || provider.calls != 0 {
		t.Fatal("saved custom key was not used for background probe")
	}
	bad := "wrong-key"
	input.Key = &bad
	_, err = svc.SaveGroupProbeConfiguration(ctx, "user1", "42", input)
	if err == nil || err.Error() != groupProbePrefix+"customKeyUnavailable" || provider.calls != 0 {
		t.Fatal("invalid supplied key must fail without auto-creating")
	}
	stored, _ := repo.GetGroupProbeConfig(ctx, "user1", "ws1", "42")
	if stored.CustomKeyID != "key-7" {
		t.Fatal("failed save destroyed working settings")
	}
	provider.err = errors.New("revoked key secret")
	past := time.Now().Add(-time.Second)
	stored.NextProbeAt = &past
	_ = repo.SaveGroupProbeConfig(ctx, *stored)
	restarted.runScheduledGroupProbe(ctx, *stored)
	stored, _ = repo.GetGroupProbeConfig(ctx, "user1", "ws1", "42")
	if stored.LastErrorKey != groupProbePrefix+"customKeyUnavailable" || len(repo.events) != 1 || provider.calls != 0 {
		t.Fatal("revoked key silently fell back or invented results")
	}
	provider.err = nil
	session.AccessToken = "jwt"
	svc.mySites = fakeMySitesReader{session: session}
	empty := ""
	input.Key = &empty
	input.UseAutoKey = true
	c, err = svc.SaveGroupProbeConfiguration(ctx, "user1", "42", input)
	if err != nil || c.HasCustomKey || c.CustomKeyID != "" || provider.calls != 1 {
		t.Fatal("explicit reset did not switch to automatic key")
	}
}

func configTestKey(user, workspace, group string) string { return user + "|" + workspace + "|" + group }
func (f *fakeRepository) GetGroupProbeConfig(_ context.Context, user, workspace, group string) (*GroupProbeConfig, error) {
	f.groupProbeMu.Lock()
	defer f.groupProbeMu.Unlock()
	c, ok := f.groupProbeConfigs[configTestKey(user, workspace, group)]
	if !ok {
		return nil, nil
	}
	return &c, nil
}
func (f *fakeRepository) ListGroupProbeConfigs(_ context.Context, user, workspace string) ([]GroupProbeConfig, error) {
	f.groupProbeMu.Lock()
	defer f.groupProbeMu.Unlock()
	out := []GroupProbeConfig{}
	for _, c := range f.groupProbeConfigs {
		if c.UserID == user && c.AdminAccountID == workspace {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeRepository) ListDueGroupProbeConfigs(_ context.Context, now time.Time) ([]GroupProbeConfig, error) {
	f.groupProbeMu.Lock()
	defer f.groupProbeMu.Unlock()
	out := []GroupProbeConfig{}
	for _, c := range f.groupProbeConfigs {
		if c.Enabled && c.NextProbeAt != nil && !c.NextProbeAt.After(now) {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeRepository) SaveGroupProbeConfig(_ context.Context, c GroupProbeConfig) error {
	f.groupProbeMu.Lock()
	defer f.groupProbeMu.Unlock()
	f.groupProbeConfigs[configTestKey(c.UserID, c.AdminAccountID, c.GroupID)] = c
	return nil
}
func (f *fakeRepository) CompleteGroupProbe(_ context.Context, c GroupProbeConfig, now time.Time, key string) error {
	f.groupProbeMu.Lock()
	defer f.groupProbeMu.Unlock()
	mapKey := configTestKey(c.UserID, c.AdminAccountID, c.GroupID)
	stored, ok := f.groupProbeConfigs[mapKey]
	if !ok || !stored.Enabled {
		return nil
	}
	stored.LastProbeAt = &now
	next := now.Add(time.Duration(stored.IntervalSeconds) * time.Second)
	stored.NextProbeAt = &next
	stored.LastErrorKey = key
	f.groupProbeConfigs[mapKey] = stored
	return nil
}

type completedGroupProbeRepo struct {
	*serializedGroupProbeRepo
	completed chan time.Time
}

func (r *completedGroupProbeRepo) CompleteGroupProbe(ctx context.Context, c GroupProbeConfig, now time.Time, key string) error {
	err := r.fakeRepository.CompleteGroupProbe(ctx, c, now, key)
	r.completed <- now
	return err
}

func TestGroupProbeSchedulerRepeatsWithoutBrowserRequests(t *testing.T) {
	svc, repo, _ := groupConfigTestService(t)
	c, err := svc.SaveGroupProbeConfiguration(context.Background(), "user1", "42", GroupProbeConfigInput{Model: "model", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	// Shorten only the test's stored interval so the real scheduler can repeat.
	c.IntervalSeconds = 1
	_ = repo.SaveGroupProbeConfig(context.Background(), c)
	observed := &completedGroupProbeRepo{serializedGroupProbeRepo: &serializedGroupProbeRepo{fakeRepository: repo}, completed: make(chan time.Time, 4)}
	svc.repo = observed
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartGroupProbeScheduler(ctx)
	var first time.Time
	for i := 0; i < 2; i++ {
		select {
		case now := <-observed.completed:
			if i == 0 {
				first = now
			} else if now.Sub(first) < time.Second {
				t.Fatal("scheduler ignored interval")
			}
		case <-time.After(6 * time.Second):
			t.Fatal("backend did not continue probing independently")
		}
	}
	// Pause under the same lease as normal settings saves, while the loop lives.
	release, _ := observed.AcquireTargetLease(ctx, "42")
	c.Enabled = false
	c.NextProbeAt = nil
	_ = repo.SaveGroupProbeConfig(ctx, c)
	if len(repo.events) != 2 {
		t.Fatal("unexpected number of scheduled probes")
	}
	release()
	select {
	case <-observed.completed:
		t.Fatal("paused task continued probing")
	case <-time.After(1500 * time.Millisecond):
	}
}

func groupConfigTestService(t *testing.T) (*Service, *fakeRepository, *fakeGroupProbeProvider) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer group-secret" {
			t.Errorf("unexpected group request")
		}
		_, _ = w.Write([]byte(`{"object":"response","status":"completed","output":[{"type":"message"}]}`))
	}))
	t.Cleanup(server.Close)
	repo := newFakeRepository()
	provider := &fakeGroupProbeProvider{fakePlatformGroupReader: fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "42", Name: "Group", Platform: ProviderOpenAI}}}}
	svc := newAdminGroupsService(provider, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API, AccessToken: "jwt", BaseURL: server.URL}}, repo)
	svc.dispatcher = panicIfCalledRemoteActionRunner{}
	return svc, repo, provider
}

func TestGroupProbeConfigDefaultsPersistsAndPausesWithoutUpstream(t *testing.T) {
	svc, repo, provider := groupConfigTestService(t)
	ctx := context.Background()
	c, err := svc.SaveGroupProbeConfiguration(ctx, "user1", "42", GroupProbeConfigInput{Model: " model ", Enabled: true})
	if err != nil || c.IntervalSeconds != 60 || c.Model != "model" || c.NextProbeAt == nil || c.NextProbeAt.After(time.Now()) {
		t.Fatalf("default/save: %+v %v", c, err)
	}
	if len(repo.events) != 0 {
		t.Fatal("saving must schedule, not synchronously probe")
	}
	loaded, err := svc.GroupProbeConfiguration(ctx, "user1", "42")
	if err != nil || loaded == nil || loaded.IntervalSeconds != 60 {
		t.Fatal("configuration was not persisted")
	}
	foreign, _ := svc.GroupProbeConfiguration(ctx, "other", "42")
	if foreign != nil {
		t.Fatal("configuration leaked across users")
	}
	provider.err = errors.New("upstream unavailable")
	calls := provider.calls
	c, err = svc.SaveGroupProbeConfiguration(ctx, "user1", "42", GroupProbeConfigInput{Model: "model", IntervalSeconds: 120, Enabled: false})
	if err != nil || c.Enabled || c.NextProbeAt != nil || c.IntervalSeconds != 120 || provider.calls != calls {
		t.Fatalf("pause must work offline: %+v %v", c, err)
	}
}

func TestGroupProbeConfigRejectsInvalidIntervalsAndForeignGroups(t *testing.T) {
	svc, repo, provider := groupConfigTestService(t)
	for _, interval := range []int{-1, 1, 9, 86401} {
		_, err := svc.SaveGroupProbeConfiguration(context.Background(), "user1", "42", GroupProbeConfigInput{Model: "model", IntervalSeconds: interval, Enabled: true})
		if err == nil || err.Error() != groupProbePrefix+"intervalInvalid" {
			t.Fatalf("accepted interval %d", interval)
		}
	}
	_, err := svc.SaveGroupProbeConfiguration(context.Background(), "user1", "foreign", GroupProbeConfigInput{Model: "model", Enabled: true})
	if err == nil || provider.calls != 0 || len(repo.groupProbeConfigs) != 0 {
		t.Fatal("invalid config reached credential creation/storage")
	}
}

func TestScheduledGroupProbeDueRetryPauseAndWorkspaceIsolation(t *testing.T) {
	svc, repo, provider := groupConfigTestService(t)
	ctx := context.Background()
	c, err := svc.SaveGroupProbeConfiguration(ctx, "user1", "42", GroupProbeConfigInput{Model: "model", IntervalSeconds: 60, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate switching the selected workspace and restarting the service.
	restarted := *svc
	restarted.accounts = fakeAdminAccountResolver{id: "other-workspace"}
	restarted.runScheduledGroupProbe(ctx, c)
	if len(repo.events) != 1 || repo.events[0].ConnectionID != "group:ws1:42" {
		t.Fatal("scheduled result went to current workspace instead of saved workspace")
	}
	stored, _ := repo.GetGroupProbeConfig(ctx, "user1", "ws1", "42")
	if stored.LastProbeAt == nil || stored.NextProbeAt.Sub(*stored.LastProbeAt) != 60*time.Second {
		t.Fatal("next run does not respect interval")
	}
	restarted.runScheduledGroupProbe(ctx, c)
	if len(repo.events) != 1 {
		t.Fatal("stale due candidate caused duplicate probe")
	}
	provider.err = errors.New("credential secret must not leak")
	past := time.Now().Add(-time.Second)
	stored.NextProbeAt = &past
	_ = repo.SaveGroupProbeConfig(ctx, *stored)
	restarted.runScheduledGroupProbe(ctx, c)
	stored, _ = repo.GetGroupProbeConfig(ctx, "user1", "ws1", "42")
	if stored.LastErrorKey != groupProbePrefix+"keyUnavailable" || !stored.NextProbeAt.After(time.Now()) || len(repo.events) != 1 {
		t.Fatal("credential failure must back off and not invent probe samples")
	}
	stored.Enabled = false
	stored.NextProbeAt = &past
	_ = repo.SaveGroupProbeConfig(ctx, *stored)
	calls := provider.calls
	restarted.runScheduledGroupProbe(ctx, c)
	if provider.calls != calls {
		t.Fatal("paused task contacted upstream")
	}
	if len(repo.states) != 0 || len(repo.budgetClaims) != 0 {
		t.Fatal("group schedule changed channel policy state")
	}
}

type serializedGroupProbeRepo struct {
	*fakeRepository
	lease sync.Mutex
}

func (r *serializedGroupProbeRepo) AcquireTargetLease(context.Context, string) (func(), error) {
	r.lease.Lock()
	return r.lease.Unlock, nil
}

func TestScheduledGroupProbeConcurrentWorkersRunOnlyOnce(t *testing.T) {
	svc, repo, _ := groupConfigTestService(t)
	c, err := svc.SaveGroupProbeConfiguration(context.Background(), "user1", "42", GroupProbeConfigInput{Model: "model", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	svc.repo = &serializedGroupProbeRepo{fakeRepository: repo}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); svc.runScheduledGroupProbe(context.Background(), c) }()
	}
	wg.Wait()
	if len(repo.events) != 1 {
		t.Fatalf("got %d probes from duplicate due candidates", len(repo.events))
	}
}
