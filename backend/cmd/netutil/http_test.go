package netutil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOutboundClientBlocksLocalNetworkByDefault(t *testing.T) {
	t.Setenv("ALLOW_LOCAL_NETWORK", "false")
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, "internal")
	}))
	defer srv.Close()

	resp, err := NewOutboundHTTPClient(0).Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected loopback HTTP request to be blocked")
	}
	if hits.Load() != 0 {
		t.Fatal("blocked request reached the local server")
	}
}

func TestOutboundClientAllowsExplicitLocalNetworkOptIn(t *testing.T) {
	t.Setenv("ALLOW_LOCAL_NETWORK", "true")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	resp, err := NewOutboundHTTPClient(0).Get(srv.URL)
	if err != nil {
		t.Fatalf("local opt-in request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestPublicHTTPSClientIgnoresLocalNetworkOptIn(t *testing.T) {
	t.Setenv("ALLOW_LOCAL_NETWORK", "true")
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	resp, err := NewPublicHTTPSClient(0).Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("public HTTPS client accepted loopback HTTP")
	}
	if hits.Load() != 0 {
		t.Fatal("strict request reached the local server")
	}
}

func TestOutboundClientChecksRedirects(t *testing.T) {
	t.Setenv("ALLOW_LOCAL_NETWORK", "false")
	client := NewOutboundHTTPClient(0)
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/internal", nil)
	err := client.CheckRedirect(req, []*http.Request{req})
	if err == nil || !strings.Contains(err.Error(), "redirect blocked") {
		t.Fatalf("expected blocked redirect, got %v", err)
	}
}

func TestValidateOutboundURLLocalStillRejectsOtherSchemes(t *testing.T) {
	t.Setenv("ALLOW_LOCAL_NETWORK", "true")
	if _, err := ValidateOutboundURL("file:///etc/passwd"); err == nil {
		t.Fatal("local opt-in must not permit non-HTTP schemes")
	}
}
