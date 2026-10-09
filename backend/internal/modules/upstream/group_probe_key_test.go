package upstream

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGroupProbeMalformedKeyResponseDoesNotLogCredentials(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"key":"never-log-this-key"`))
	}))
	defer server.Close()
	svc := NewPlatformService(NewHTTPClient(server.Client()))
	_, err := svc.EnsureSub2APIGroupProbeKey(Session{Platform: PlatformSub2API, BaseURL: server.URL, AccessToken: "jwt"}, "42", "dedicated")
	if err == nil || strings.Contains(output.String(), "never-log-this-key") {
		t.Fatal("malformed key response must fail without logging its body")
	}
}

func TestGroupProbeKeyCreatesBoundKeyThenReusesIt(t *testing.T) {
	var keys []any = []any{}
	creates := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/keys" || r.Header.Get("Authorization") != "Bearer user-token" {
			t.Errorf("incorrect key endpoint/auth")
		}
		if r.Method == http.MethodGet {
			writeJSON(w, map[string]any{"data": map[string]any{"items": keys, "total": len(keys)}})
			return
		}
		creates++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["name"] != "dedicated" || body["group_id"] != float64(42) {
			t.Errorf("incorrect key binding: %v", body)
		}
		key := map[string]any{"id": 1, "name": "dedicated", "group_id": 42, "status": "active", "key": "test-group-key"}
		keys = append(keys, key)
		writeJSON(w, map[string]any{"data": key})
	}))
	defer server.Close()
	svc := NewPlatformService(NewHTTPClient(server.Client()))
	session := Session{Platform: PlatformSub2API, BaseURL: server.URL, AccessToken: "user-token", TokenType: "Bearer"}
	for i := 0; i < 2; i++ {
		cred, err := svc.EnsureSub2APIGroupProbeKey(session, "42", "dedicated")
		if err != nil || cred.BaseURL != server.URL || cred.Key != "test-group-key" {
			t.Fatalf("key unavailable: %v", err)
		}
	}
	if creates != 1 {
		t.Fatalf("created %d keys, want one", creates)
	}
}

func TestGroupProbeKeyPaginationAndRefusal(t *testing.T) {
	for _, scenario := range []string{"later-page", "reassigned", "inactive", "masked", "list-error", "invalid-list"} {
		t.Run(scenario, func(t *testing.T) {
			gets, creates := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					creates++
					w.WriteHeader(500)
					return
				}
				gets++
				if scenario == "list-error" {
					w.WriteHeader(403)
					return
				}
				if scenario == "invalid-list" {
					writeJSON(w, map[string]any{"data": map[string]any{}})
					return
				}
				key := map[string]any{"name": "dedicated", "group_id": 42, "status": "active", "key": "test-group-key"}
				switch scenario {
				case "reassigned":
					key["group_id"] = 43
				case "inactive":
					key["status"] = "inactive"
				case "masked":
					key["key"] = "sk-***"
				}
				items := []any{key}
				if scenario == "later-page" && r.URL.Query().Get("page") == "1" {
					items = make([]any, 100)
					for i := range items {
						items[i] = map[string]any{"name": "unrelated", "group_id": 42, "status": "active", "key": "not-for-probing"}
					}
				}
				writeJSON(w, map[string]any{"data": map[string]any{"items": items, "total": 101}})
			}))
			defer server.Close()
			svc := NewPlatformService(NewHTTPClient(server.Client()))
			cred, err := svc.EnsureSub2APIGroupProbeKey(Session{Platform: PlatformSub2API, BaseURL: server.URL, AccessToken: "jwt"}, "42", "dedicated")
			if scenario == "later-page" {
				if err != nil || cred.Key != "test-group-key" || gets != 2 {
					t.Fatalf("later page not reused: gets=%d err=%v", gets, err)
				}
			} else if err == nil {
				t.Fatal("unsafe or unreadable key should be refused")
			}
			if creates != 0 {
				t.Fatal("must not create a replacement after refusal or reuse")
			}
		})
	}
}
