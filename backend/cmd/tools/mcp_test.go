package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func startDescribedMCPServer(t *testing.T) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{
		Name: "github-mcp-server", Title: "GitHub MCP Server", Version: "v0.9.0", Description: "Issues, PRs and repos",
	}, &mcp.ServerOptions{Instructions: "Use list_prs before reviewing."})
	mcp.AddTool(server, &mcp.Tool{Name: "list_prs", Description: "List PRs"}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{}, nil, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(httpServer.Close)
	return httpServer.URL
}

func TestSaveMCPServer_StoresServerInfo(t *testing.T) {
	endpoint := startDescribedMCPServer(t)
	setupOAuthTest(t)

	saved := saveServerReq(t, map[string]any{
		"name": "work-gh", "endpoint": endpoint, "api_key": "", "headers": map[string]string{},
	})
	want := MCPServerInfo{
		Name: "github-mcp-server", Title: "GitHub MCP Server", Version: "v0.9.0",
		Description: "Issues, PRs and repos", Instructions: "Use list_prs before reviewing.",
	}
	if saved.ServerInfo == nil || *saved.ServerInfo != want {
		t.Fatalf("save response server_info = %+v, want %+v", saved.ServerInfo, want)
	}

	server, err := mcps.GetByID(saved.ID, "testuser")
	if err != nil {
		t.Fatal(err)
	}
	if server.Info != want {
		t.Fatalf("stored info = %+v, want %+v", server.Info, want)
	}

	// Updating clears nothing: the update path re-captures the info.
	saveServerReq(t, map[string]any{
		"id": saved.ID, "name": "renamed", "endpoint": endpoint, "api_key": "", "headers": map[string]string{},
	})
	server, _ = mcps.GetByID(saved.ID, "testuser")
	if server.Name != "renamed" || server.Info != want {
		t.Fatalf("after update: name=%q info=%+v", server.Name, server.Info)
	}
}

func TestServerInfoFromInit_CapsLength(t *testing.T) {
	info := serverInfoFromInit(&mcp.InitializeResult{
		Instructions: strings.Repeat("é", mcpMaxInstructionsRunes+10),
		ServerInfo:   &mcp.Implementation{Name: "  srv  ", Description: strings.Repeat("d", mcpMaxInfoFieldRunes+10)},
	})
	if info.Name != "srv" {
		t.Errorf("name not trimmed: %q", info.Name)
	}
	if n := utf8.RuneCountInString(info.Instructions); n != mcpMaxInstructionsRunes {
		t.Errorf("instructions runes = %d, want %d", n, mcpMaxInstructionsRunes)
	}
	if n := utf8.RuneCountInString(info.Description); n != mcpMaxInfoFieldRunes {
		t.Errorf("description runes = %d, want %d", n, mcpMaxInfoFieldRunes)
	}
	if serverInfoFromInit(nil) != (MCPServerInfo{}) {
		t.Error("nil result should give empty info")
	}
}

func TestExecuteMCPTool_RejectsDisabledTool(t *testing.T) {
	endpoint := startTestMCPServer(t)
	setupOAuthTest(t)

	saved := saveServerReq(t, map[string]any{
		"name": "Plain MCP", "endpoint": endpoint, "api_key": "", "headers": map[string]string{},
	})
	server, _ := mcps.GetByID(saved.ID, "testuser")
	echo := *server.Tools[0]

	call := providers.ToolCall{ID: "c1", Name: "echo", Args: `{"text":"hi"}`}
	if out := ExecuteMCPTool(context.Background(), call, "testuser", "conv"); out.Content != "hi" {
		t.Fatalf("enabled tool: got %q", out.Content)
	}

	echo.IsEnabled = false
	if err := tools.SaveAll([]*Tool{&echo}); err != nil {
		t.Fatal(err)
	}
	if out := ExecuteMCPTool(context.Background(), call, "testuser", "conv"); out.Content != "Tool 'echo' is disabled." {
		t.Fatalf("disabled tool: got %q", out.Content)
	}
}

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
