package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCacheControlMiddleware_API(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/conversations/", nil)
	rr := httptest.NewRecorder()

	cacheControlMiddleware(handler).ServeHTTP(rr, req)

	cc := rr.Header().Get("Cache-Control")
	expected := "private, no-store, no-cache, must-revalidate"
	if cc != expected {
		t.Errorf("expected Cache-Control %q, got %q", expected, cc)
	}

	if rr.Header().Get("Pragma") != "no-cache" {
		t.Errorf("expected Pragma no-cache, got %q", rr.Header().Get("Pragma"))
	}
}

func TestCacheControlMiddleware_Resources(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file contents"))
	})

	req := httptest.NewRequest(http.MethodGet, "/data/resources/file.xlsx", nil)
	rr := httptest.NewRecorder()

	cacheControlMiddleware(handler).ServeHTTP(rr, req)

	cc := rr.Header().Get("Cache-Control")
	expected := "private, no-cache, must-revalidate"
	if cc != expected {
		t.Errorf("expected Cache-Control %q, got %q", expected, cc)
	}
}

func TestCacheControlMiddleware_StaticAssets(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html></html>"))
	})

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rr := httptest.NewRecorder()

	cacheControlMiddleware(handler).ServeHTTP(rr, req)

	if rr.Header().Get("Cache-Control") != "" {
		t.Errorf("expected no Cache-Control header for static assets, got %q", rr.Header().Get("Cache-Control"))
	}
}

func TestCacheControlMiddleware_HandlerCanOverride(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handlers with specific caching needs may override the default.
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/settings/", nil)
	rr := httptest.NewRecorder()

	cacheControlMiddleware(handler).ServeHTTP(rr, req)

	cc := rr.Header().Get("Cache-Control")
	expected := "no-cache"
	if cc != expected {
		t.Errorf("expected Cache-Control %q, got %q", expected, cc)
	}
}
