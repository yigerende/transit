package connection_health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"transithub/backend/internal/modules/upstream"
)

type fakeGroupProbeProvider struct {
	fakePlatformGroupReader
	calls   int
	name    string
	groupID string
	err     error
}

func (f *fakeGroupProbeProvider) ResolveSub2APIGroupProbeKey(_ upstream.Session, groupID, key, id string) (upstream.ProbeCredential, string, error) {
	if groupID != "42" || (key != "group-secret" && (key != "" || id != "key-7")) {
		return upstream.ProbeCredential{}, "", errors.New("invalid secret")
	}
	return upstream.ProbeCredential{Key: "group-secret"}, "key-7", f.err
}

func (f *fakeGroupProbeProvider) EnsureSub2APIGroupProbeKey(_ upstream.Session, groupID, name string) (upstream.ProbeCredential, error) {
	f.calls++
	f.name, f.groupID = name, groupID
	return upstream.ProbeCredential{BaseURL: "http://wrong-gateway.invalid", Key: "group-secret"}, f.err
}

func TestGroupProbePreparationAndHistoryAreIndependent(t *testing.T) {
	modelRequests, probes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer group-secret" {
			t.Error("missing dedicated group key")
		}
		switch r.URL.Path {
		case "/v1/models":
			modelRequests++
			_, _ = w.Write([]byte(`{"data":[{"id":"group-model"}]}`))
		case "/v1/responses":
			probes++
			_, _ = w.Write([]byte(`{"object":"response","status":"completed","output":[{"type":"message"}]}`))
		default:
			t.Errorf("unexpected gateway endpoint: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	repo := newFakeRepository()
	provider := &fakeGroupProbeProvider{fakePlatformGroupReader: fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "42", Name: "Group", Platform: ProviderOpenAI}}}}
	svc := newAdminGroupsService(provider, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API, AccessToken: "jwt", BaseURL: server.URL}}, repo)
	svc.modelDiscovery = NewModelDiscoveryRunner()
	svc.dispatcher = panicIfCalledRemoteActionRunner{}
	preparation, err := svc.PrepareGroupProbe(context.Background(), "user1", "42")
	if err != nil || len(preparation.Models) != 1 || preparation.Models[0].ID != "group-model" || preparation.ModelListUnavailable {
		t.Fatalf("prepare failed: %+v %v", preparation, err)
	}
	if probes != 0 || len(repo.events) != 0 {
		t.Fatal("preparing key must not send billable model probe or save fake results")
	}
	keyName := provider.name
	sample, err := svc.ProbeAdminGroup(context.Background(), "user1", "42", " group-model ")
	if err != nil || sample.Result != string(ResultOK) || sample.TargetID != "group:ws1:42" || sample.ModelName != "group-model" {
		t.Fatalf("probe failed: %+v %v", sample, err)
	}
	if provider.name != keyName || provider.groupID != "42" || modelRequests != 1 || probes != 1 {
		t.Fatal("incorrect group key identity or request count")
	}
	if len(repo.events) != 1 || repo.events[0].ConnectionID != "group:ws1:42" || repo.events[0].AdminGroupID != "42" || repo.events[0].UserID != "user1" || repo.events[0].AdminAccountID != "ws1" {
		t.Fatal("history not scoped to current group/workspace/user")
	}
	if len(repo.states) != 0 || len(repo.budgetClaims) != 0 || len(repo.targetActionStates) != 0 {
		t.Fatal("group probe touched channel policy machinery")
	}
	encoded, _ := json.Marshal([]any{sample, preparation, repo.events})
	if strings.Contains(string(encoded), "group-secret") {
		t.Fatal("key leaked into result/history")
	}
}

func TestGroupProbeValidatesBeforeCreatingKey(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		platform                    upstream.Platform
		token, groupID, model, want string
	}{
		{"unsupported", upstream.PlatformNewAPI, "jwt", "42", "model", groupProbePrefix + "unsupported"},
		{"admin-key-only", upstream.PlatformSub2API, "", "42", "model", groupProbePrefix + "loginRequired"},
		{"foreign-group", upstream.PlatformSub2API, "jwt", "43", "model", ErrorNotFound},
		{"empty-model", upstream.PlatformSub2API, "jwt", "42", " ", groupProbePrefix + "modelRequired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &fakeGroupProbeProvider{fakePlatformGroupReader: fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "42"}}}}
			svc := newAdminGroupsService(provider, fakeMySitesReader{session: upstream.Session{Platform: tc.platform, AccessToken: tc.token}}, newFakeRepository())
			_, err := svc.ProbeAdminGroup(context.Background(), "user1", tc.groupID, tc.model)
			if err == nil || err.Error() != tc.want || provider.calls != 0 {
				t.Fatalf("validation failed: err=%v calls=%d", err, provider.calls)
			}
		})
	}
}

func TestGroupProbeKeyErrorsAreSanitizedAndModelDiscoveryCanFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) }))
	defer server.Close()
	provider := &fakeGroupProbeProvider{fakePlatformGroupReader: fakePlatformGroupReader{groups: []upstream.AdminGroupInfo{{ID: "42"}}}, err: errors.New("secret error group-secret")}
	svc := newAdminGroupsService(provider, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API, AccessToken: "jwt", BaseURL: server.URL}}, newFakeRepository())
	svc.modelDiscovery = NewModelDiscoveryRunner()
	_, err := svc.PrepareGroupProbe(context.Background(), "user1", "42")
	if err == nil || err.Error() != groupProbePrefix+"keyUnavailable" {
		t.Fatalf("unsafe key error: %v", err)
	}
	provider.err = nil
	prepared, err := svc.PrepareGroupProbe(context.Background(), "user1", "42")
	if err != nil || !prepared.ModelListUnavailable || len(prepared.Models) != 0 {
		t.Fatal("must allow manual model entry after discovery fails")
	}
}
