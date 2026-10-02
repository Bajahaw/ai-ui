package version

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHandleGetVersion_UsesBuildIDNotLegacyBuild(t *testing.T) {
	prev := BuildID
	BuildID = "260918.120000"
	t.Cleanup(func() { BuildID = prev })

	req := httptest.NewRequest("GET", "/api/version", nil)
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
