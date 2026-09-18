package version

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseSandboxOrigin(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"https://sandbox.example.com", "https://sandbox.example.com"},
		{"https://sandbox.example.com/path?x=1", "https://sandbox.example.com"},
		{"http://sandbox.localhost:8080", "http://sandbox.localhost:8080"},
		{"ftp://sandbox.example.com", ""},
		{"https://user:pass@sandbox.example.com", ""},
		{"not a url", ""},
	}
	for _, tc := range tests {
		if got := ParseSandboxOrigin(tc.in); got != tc.want {
			t.Errorf("ParseSandboxOrigin(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHandleGetVersion_IncludesSandboxOrigin(t *testing.T) {
	t.Setenv("SANDBOX_ORIGIN", "https://sandbox.example.com/ignored")
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rr := httptest.NewRecorder()
	HandleGetVersion(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var got VersionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SandboxOrigin != "https://sandbox.example.com" {
		t.Fatalf("sandbox_origin = %q", got.SandboxOrigin)
	}
}

func TestHandleGetVersion_UsesBuildIDNotLegacyBuild(t *testing.T) {
	prev := BuildID
	BuildID = "260918.120000"
	t.Cleanup(func() { BuildID = prev })

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rr := httptest.NewRecorder()
	HandleGetVersion(rr, req)
	var got VersionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Build != "" {
		t.Fatalf("legacy build should be empty, got %q", got.Build)
	}
	if got.BuildID != "260918.120000" {
		t.Fatalf("build_id = %q", got.BuildID)
	}
	var raw map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["build"]; ok {
		t.Fatal("legacy build field must be omitted for old clients")
	}
}

func TestHandleGetVersion_OmitsUnsetOrigin(t *testing.T) {
	t.Setenv("SANDBOX_ORIGIN", "")
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rr := httptest.NewRecorder()
	HandleGetVersion(rr, req)
	var got VersionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SandboxOrigin != "" {
		t.Fatalf("expected empty origin, got %q", got.SandboxOrigin)
	}
}
