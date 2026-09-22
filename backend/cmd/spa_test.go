package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newSPATestHandler(t *testing.T) spaHandler {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>shell</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app-abc123.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sw.js"), []byte("self.skipWaiting()"), 0o644); err != nil {
		t.Fatal(err)
	}
	return spaHandler{staticDir: dir, fs: http.FileServer(http.Dir(dir))}
}

func TestSPAHandlerServesShellWithoutRedirect(t *testing.T) {
	h := newSPATestHandler(t)
	for _, path := range []string{"/", "/index.html", "/c/6756152f-2436-4396-88ea-b444bd19f2be", "/index.html?__WB_REVISION__=abc"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d (Location=%q)", path, rec.Code, rec.Header().Get("Location"))
		}
		if !strings.Contains(rec.Body.String(), "shell") {
			t.Fatalf("%s: expected SPA shell body, got %q", path, rec.Body.String())
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
			t.Fatalf("%s: expected no-cache shell, got Cache-Control=%q", path, cc)
		}
	}
}

func TestSPAHandlerServesRealFilesWithCacheHeaders(t *testing.T) {
	h := newSPATestHandler(t)
	cases := map[string]string{
		"/assets/app-abc123.js": "immutable",
		"/sw.js":                "no-cache",
	}
	for path, want := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, rec.Code)
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, want) {
			t.Fatalf("%s: expected Cache-Control containing %q, got %q", path, want, cc)
		}
	}
}
