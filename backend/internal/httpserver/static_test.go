package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStaticHandlerReleaseCaching(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"index.html": "<!doctype html><div id=app></div>", "assets/current.js": "export default 1"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	handler := staticHandler(dir)
	for _, path := range []string{"/", "/embed/group-quality?embed_token=test", "/admin/group-quality"} {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != http.StatusOK || r.Header().Get("Cache-Control") != "no-store" || !strings.Contains(r.Body.String(), "id=app") {
			t.Fatalf("entry %s: %d %v", path, r.Code, r.Header())
		}
	}
	for _, method := range []string{"GET", "HEAD"} {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest(method, "/assets/previous-release.js", nil))
		if r.Code != http.StatusNotFound || r.Header().Get("Cache-Control") != "no-store" || strings.Contains(r.Body.String(), "id=app") {
			t.Fatalf("obsolete asset: %d %v", r.Code, r.Header())
		}
	}
	r := httptest.NewRecorder()
	handler.ServeHTTP(r, httptest.NewRequest("GET", "/assets/current.js", nil))
	if r.Code != http.StatusOK || r.Body.String() != "export default 1" {
		t.Fatal("existing assets no longer served")
	}
}
