package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "empty", text: "", want: 0},
		{name: "short", text: "hi", want: 1},
		{name: "exactly four runes", text: "abcd", want: 1},
		{name: "five runes", text: "abcde", want: 2},
		{name: "sixteen runes", text: strings.Repeat("a", 16), want: 4},
		{name: "unicode runes", text: "你好世界", want: 1}, // 4 runes -> 1 token
		{name: "unicode longer", text: "你好世界!!", want: 2}, // 6 runes -> 2 tokens
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got != tt.want {
				t.Fatalf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

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

func TestCacheControlMiddleware_TTSAllowsHandlerCache(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
		w.Header().Set("ETag", `"abc"`)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/tts/messages/1", nil)
	rr := httptest.NewRecorder()

	cacheControlMiddleware(handler).ServeHTTP(rr, req)

	if rr.Header().Get("Cache-Control") != "private, max-age=0, must-revalidate" {
		t.Errorf("unexpected Cache-Control %q", rr.Header().Get("Cache-Control"))
	}
	if rr.Header().Get("Pragma") != "" {
		t.Errorf("expected no Pragma for TTS path, got %q", rr.Header().Get("Pragma"))
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

func TestCacheControlMiddleware_ServiceWorker(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("/* sw */"))
	})

	req := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
	rr := httptest.NewRecorder()

	cacheControlMiddleware(handler).ServeHTTP(rr, req)

	if rr.Header().Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
		t.Errorf("unexpected Cache-Control %q", rr.Header().Get("Cache-Control"))
	}
	if rr.Header().Get("Service-Worker-Allowed") != "/" {
		t.Errorf("expected Service-Worker-Allowed /, got %q", rr.Header().Get("Service-Worker-Allowed"))
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
