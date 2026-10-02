package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPTools_EnabledByDefaultAndRefreshKeepsDisabled(t *testing.T) {
	endpoint := startTestMCPServer(t)
	setupOAuthTest(t) // after the server so the session cache closes first

	saved := saveServerReq(t, map[string]any{
		"name": "Plain MCP", "endpoint": endpoint, "api_key": "", "headers": map[string]string{},
	})
	server, _ := mcps.GetByID(saved.ID, "testuser")
	if len(server.Tools) != 1 || !server.Tools[0].IsEnabled {
		t.Fatalf("new MCP tools should be enabled, got %+v", server.Tools)
	}

	echo := *server.Tools[0]
	echo.IsEnabled = false
	if err := tools.SaveAll([]*Tool{&echo}); err != nil {
		t.Fatalf("disable tool: %v", err)
	}

	r := withUser(httptest.NewRequest(http.MethodPost, "/mcp/refresh-tools/"+saved.ID, nil))
	r.SetPathValue("id", saved.ID)
	w := httptest.NewRecorder()
	refreshMCPTools(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("refresh: status %d: %s", w.Code, w.Body.String())
	}

	server, _ = mcps.GetByID(saved.ID, "testuser")
	if len(server.Tools) != 1 || server.Tools[0].IsEnabled || server.Tools[0].ID != echo.ID {
		t.Fatalf("refresh must keep the disabled tool disabled, got %+v", server.Tools[0])
	}
}
