package tools

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	logger "github.com/charmbracelet/log"
)

func setupDiscoverTest(t *testing.T) *sql.DB {
	t.Helper()
	db, toolRepo := setupTestDB(t)
	log = logger.New(os.Stderr)
	tools = toolRepo
	mcps = NewMCPRepository(db, toolRepo)
	mcpSessionManager = newMCPSessionManager()
	t.Cleanup(mcpSessionManager.closeAll)
	return db
}

func insertToolsFor(t *testing.T, db *sql.DB, serverID string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := db.Exec(
			`INSERT INTO Tools (id, mcp_server_id, name, description, input_schema, require_approval, is_enabled)
			 VALUES (?, ?, ?, ?, ?, 0, 1)`,
			fmt.Sprintf("%s-t%d", serverID, i),
			serverID,
			fmt.Sprintf("%s_tool_%d", serverID, i),
			fmt.Sprintf("Does thing %d", i),
			`{"type":"object","properties":{"x":{"type":"integer"}}}`,
		); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGetPromptTools_DefersToolHeavyServers(t *testing.T) {
	db := setupDiscoverTest(t)

	// server1 (from setupTestDB) stays small; add a tool-heavy server.
	if _, err := db.Exec(
		`INSERT INTO MCPServers (id, name, endpoint, api_key, user) VALUES ('big', 'Big MCP', 'http://localhost', '', 'testuser')`,
	); err != nil {
		t.Fatal(err)
	}
	insertToolsFor(t, db, "server1", 3)
	insertToolsFor(t, db, "big", promptDeferThreshold+1)

	got := GetPromptTools("testuser")
	for _, tool := range got {
		if strings.HasPrefix(tool.Name, "big_tool_") {
			t.Fatalf("tool-heavy server must be deferred from the prompt: %s", tool.Name)
		}
	}
	hasSmall := false
	for _, tool := range got {
		if strings.HasPrefix(tool.Name, "server1_tool_") {
			hasSmall = true
			break
		}
	}
	if !hasSmall {
		t.Fatal("small server must stay in the prompt")
	}

	// Under the threshold nothing is deferred.
	if deferred := deferredServerIDs("testuser"); len(deferred) != 1 || !deferred["big"] {
		t.Fatalf("deferred = %v", deferred)
	}
}

func TestDeferredServerSummaries_ListsNames(t *testing.T) {
	db := setupDiscoverTest(t)
	if _, err := db.Exec(
		`INSERT INTO MCPServers (id, name, endpoint, api_key, user) VALUES ('big', 'Big MCP', 'http://localhost', '', 'testuser')`,
	); err != nil {
		t.Fatal(err)
	}
	insertToolsFor(t, db, "big", promptDeferThreshold+1)

	summaries := DeferredServerSummaries("testuser")
	if len(summaries) != 1 {
		t.Fatalf("summaries = %+v", summaries)
	}
	s := summaries[0]
	if s.Server != "Big MCP" || s.Total != promptDeferThreshold+1 {
		t.Fatalf("summary = %+v", s)
	}
	if len(s.ToolNames) != promptDeferThreshold+1 {
		t.Fatalf("names = %d", len(s.ToolNames))
	}
	if s.ToolNames[0] != "big_tool_0" {
		t.Fatalf("names not sorted: %v", s.ToolNames[:3])
	}
}

func TestListToolsTool_MarksDeferredAndFilters(t *testing.T) {
	db := setupDiscoverTest(t)
	if _, err := db.Exec(
		`INSERT INTO MCPServers (id, name, endpoint, api_key, user) VALUES ('big', 'Big MCP', 'http://localhost', '', 'testuser')`,
	); err != nil {
		t.Fatal(err)
	}
	insertToolsFor(t, db, "server1", 2)
	insertToolsFor(t, db, "big", promptDeferThreshold+1)

	out := listToolsTool(`{}`, "testuser")
	if !strings.Contains(out.Content, "big_tool_0 [deferred]") {
		t.Fatalf("deferred marker missing: %s", out.Content[:200])
	}
	if !strings.Contains(out.Content, "(server: Big MCP)") {
		t.Fatalf("server name missing: %s", out.Content[:200])
	}
	if !strings.Contains(out.Content, "server1_tool_0") {
		t.Fatalf("small server tools missing: %s", out.Content[:200])
	}
	if !strings.Contains(out.Content, "get_tool_schema") {
		t.Fatalf("next-step hint missing: %s", out.Content[:200])
	}

	// Query filter matches name and description, case-insensitively.
	out = listToolsTool(`{"query":"DOES THING 1"}`, "testuser")
	if strings.Contains(out.Content, "big_tool_0") || !strings.Contains(out.Content, "Does thing 1") {
		t.Fatalf("query filter broken: %s", out.Content[:300])
	}
}

func TestGetToolSchemaTool(t *testing.T) {
	db := setupDiscoverTest(t)
	if _, err := db.Exec(
		`INSERT INTO MCPServers (id, name, endpoint, api_key, user) VALUES ('big', 'Big MCP', 'http://localhost', '', 'testuser')`,
	); err != nil {
		t.Fatal(err)
	}
	insertToolsFor(t, db, "big", promptDeferThreshold+1)
	// A disabled tool the model should hear about rather than call blind.
	if _, err := db.Exec(
		`INSERT INTO Tools (id, mcp_server_id, name, description, input_schema, require_approval, is_enabled)
		 VALUES ('big-off', 'big', 'big_disabled', 'off', '{}', 0, 0)`,
	); err != nil {
		t.Fatal(err)
	}

	out := getToolSchemaTool(`{"names":["big_tool_0","nope","big_disabled"]}`, "testuser")
	for _, want := range []string{
		"- big_tool_0 (server: Big MCP)",
		"description: Does thing 0",
		`schema: {"type":"object","properties":{"x":{"type":"integer"}}}`,
		"- nope: unknown tool (use list_tools)",
		"- big_disabled: disabled",
	} {
		if !strings.Contains(out.Content, want) {
			t.Fatalf("missing %q in:\n%s", want, out.Content)
		}
	}

	if out := getToolSchemaTool(`{}`, "testuser"); !strings.Contains(out.Content, "'names' is required") {
		t.Fatalf("empty names: %s", out.Content)
	}
}
