package tools

import (
	"database/sql"
	"os"
	"path"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
)

// setupTestDB creates a temp SQLite DB with full schema and returns the db + repo.
func setupTestDB(t *testing.T) (*sql.DB, ToolRepository) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := path.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(tmpDir)
	})

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	if err := data.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Insert a test user and MCP server (required by foreign keys)
	if _, err := db.Exec("INSERT INTO Users (username, pass_hash) VALUES ('testuser', 'hash')"); err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}
	if _, err := db.Exec("INSERT INTO MCPServers (id, name, endpoint, api_key, user) VALUES ('server1', 'Test Server', 'http://localhost', 'key', 'testuser')"); err != nil {
		t.Fatalf("Failed to insert test MCP server: %v", err)
	}

	repo := NewToolRepository(db)
	return db, repo
}

// --------------------------------------------------------------------------
// SaveAll tests — used by enable/disable workflow
// --------------------------------------------------------------------------

func TestSaveAll_InsertsNewTools(t *testing.T) {
	_, repo := setupTestDB(t)

	tools := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "tool_a", Description: "desc A", InputSchema: `{"type":"object"}`, IsEnabled: true},
		{ID: "t2", MCPServerID: "server1", Name: "tool_b", Description: "desc B", InputSchema: `{"type":"object"}`, IsEnabled: false},
	}

	if err := repo.SaveAll(tools); err != nil {
		t.Fatalf("SaveAll insert failed: %v", err)
	}

	got := repo.GetAllByMCPServerID("server1")
	if len(got) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(got))
	}
}

func TestSaveAll_OnlyUpdatesFlags(t *testing.T) {
	_, repo := setupTestDB(t)

	// Insert initial tools
	initial := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "tool_a", Description: "original desc", InputSchema: `{"v":1}`, IsEnabled: true, RequireApproval: false},
	}
	if err := repo.SaveAll(initial); err != nil {
		t.Fatalf("SaveAll insert failed: %v", err)
	}

	// SaveAll with same ID but changed description, schema, AND flags
	updated := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "tool_a", Description: "CHANGED desc", InputSchema: `{"v":2}`, IsEnabled: false, RequireApproval: true},
	}
	if err := repo.SaveAll(updated); err != nil {
		t.Fatalf("SaveAll update failed: %v", err)
	}

	got, err := repo.GetByID("t1")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Flags should be updated
	if got.IsEnabled != false {
		t.Errorf("Expected IsEnabled=false, got %v", got.IsEnabled)
	}
	if got.RequireApproval != true {
		t.Errorf("Expected RequireApproval=true, got %v", got.RequireApproval)
	}

	// Description and schema should NOT be updated by SaveAll
	if got.Description != "original desc" {
		t.Errorf("SaveAll should not update description; expected 'original desc', got %q", got.Description)
	}
	if got.InputSchema != `{"v":1}` {
		t.Errorf("SaveAll should not update input_schema; expected '{\"v\":1}', got %q", got.InputSchema)
	}
}

// --------------------------------------------------------------------------
// UpsertAll tests — used by MCP refresh workflow
// --------------------------------------------------------------------------

func TestUpsertAll_InsertsNewTools(t *testing.T) {
	_, repo := setupTestDB(t)

	tools := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "tool_a", Description: "desc A", InputSchema: `{"type":"object"}`, IsEnabled: true},
	}

	if err := repo.UpsertAll(tools); err != nil {
		t.Fatalf("UpsertAll insert failed: %v", err)
	}

	got := repo.GetAllByMCPServerID("server1")
	if len(got) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(got))
	}
	if got[0].Name != "tool_a" {
		t.Errorf("Expected name 'tool_a', got %q", got[0].Name)
	}
}

func TestUpsertAll_UpdatesSchemaAndDescription(t *testing.T) {
	_, repo := setupTestDB(t)

	// Insert initial tool
	initial := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "tool_a", Description: "old desc", InputSchema: `{"v":1}`, IsEnabled: true, RequireApproval: false},
	}
	if err := repo.UpsertAll(initial); err != nil {
		t.Fatalf("UpsertAll insert failed: %v", err)
	}

	// UpsertAll with same ID but updated schema and description
	refreshed := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "tool_a_renamed", Description: "NEW desc", InputSchema: `{"v":2,"new_field":"yes"}`, IsEnabled: true, RequireApproval: false},
	}
	if err := repo.UpsertAll(refreshed); err != nil {
		t.Fatalf("UpsertAll update failed: %v", err)
	}

	got, err := repo.GetByID("t1")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.Description != "NEW desc" {
		t.Errorf("UpsertAll should update description; expected 'NEW desc', got %q", got.Description)
	}
	if got.InputSchema != `{"v":2,"new_field":"yes"}` {
		t.Errorf("UpsertAll should update input_schema; expected '{\"v\":2,\"new_field\":\"yes\"}', got %q", got.InputSchema)
	}
	if got.Name != "tool_a_renamed" {
		t.Errorf("UpsertAll should update name; expected 'tool_a_renamed', got %q", got.Name)
	}
}

func TestUpsertAll_PreservesUserFlags(t *testing.T) {
	_, repo := setupTestDB(t)

	// Insert tool with user-set flags
	initial := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "tool_a", Description: "desc", InputSchema: `{"v":1}`, IsEnabled: false, RequireApproval: true},
	}
	if err := repo.UpsertAll(initial); err != nil {
		t.Fatalf("UpsertAll insert failed: %v", err)
	}

	// Simulate refresh: same ID, updated schema, but flags passed through from existing state
	refreshed := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "tool_a", Description: "updated desc", InputSchema: `{"v":2}`, IsEnabled: false, RequireApproval: true},
	}
	if err := repo.UpsertAll(refreshed); err != nil {
		t.Fatalf("UpsertAll refresh failed: %v", err)
	}

	got, err := repo.GetByID("t1")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Flags should be preserved (passed through by caller)
	if got.IsEnabled != false {
		t.Errorf("Expected IsEnabled=false (preserved), got %v", got.IsEnabled)
	}
	if got.RequireApproval != true {
		t.Errorf("Expected RequireApproval=true (preserved), got %v", got.RequireApproval)
	}
	// Schema should be updated
	if got.Description != "updated desc" {
		t.Errorf("Expected description='updated desc', got %q", got.Description)
	}
}

// --------------------------------------------------------------------------
// DeleteNotIn tests — used by refresh to remove stale tools
// --------------------------------------------------------------------------

func TestDeleteNotIn_RemovesStaleTool(t *testing.T) {
	_, repo := setupTestDB(t)

	initial := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "keep", Description: "d", InputSchema: "{}"},
		{ID: "t2", MCPServerID: "server1", Name: "remove", Description: "d", InputSchema: "{}"},
	}
	if err := repo.SaveAll(initial); err != nil {
		t.Fatalf("SaveAll failed: %v", err)
	}

	// Keep only t1
	if err := repo.DeleteNotIn("server1", []string{"t1"}); err != nil {
		t.Fatalf("DeleteNotIn failed: %v", err)
	}

	got := repo.GetAllByMCPServerID("server1")
	if len(got) != 1 {
		t.Fatalf("Expected 1 tool after DeleteNotIn, got %d", len(got))
	}
	if got[0].ID != "t1" {
		t.Errorf("Expected tool 't1' to remain, got %q", got[0].ID)
	}
}

func TestDeleteNotIn_EmptyList_DeletesAll(t *testing.T) {
	_, repo := setupTestDB(t)

	initial := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "a", Description: "d", InputSchema: "{}"},
		{ID: "t2", MCPServerID: "server1", Name: "b", Description: "d", InputSchema: "{}"},
	}
	if err := repo.SaveAll(initial); err != nil {
		t.Fatalf("SaveAll failed: %v", err)
	}

	if err := repo.DeleteNotIn("server1", []string{}); err != nil {
		t.Fatalf("DeleteNotIn with empty list failed: %v", err)
	}

	got := repo.GetAllByMCPServerID("server1")
	if len(got) != 0 {
		t.Fatalf("Expected 0 tools after DeleteNotIn with empty list, got %d", len(got))
	}
}

// --------------------------------------------------------------------------
// Integration test: simulates full refresh flow
// --------------------------------------------------------------------------

func TestRefreshFlow_UpdatesSchemaPreservesFlags(t *testing.T) {
	_, repo := setupTestDB(t)

	// Step 1: Initial tools from MCP server
	initial := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "search", Description: "Search v1", InputSchema: `{"v":1}`, IsEnabled: true},
		{ID: "t2", MCPServerID: "server1", Name: "delete", Description: "Delete v1", InputSchema: `{"v":1}`, IsEnabled: true},
	}
	if err := repo.UpsertAll(initial); err != nil {
		t.Fatalf("Initial UpsertAll failed: %v", err)
	}

	// Step 2: User disables "search" and enables require_approval on "delete"
	flagUpdates := []*Tool{
		{ID: "t1", MCPServerID: "server1", Name: "search", Description: "Search v1", InputSchema: `{"v":1}`, IsEnabled: false},
		{ID: "t2", MCPServerID: "server1", Name: "delete", Description: "Delete v1", InputSchema: `{"v":1}`, RequireApproval: true, IsEnabled: true},
	}
	if err := repo.SaveAll(flagUpdates); err != nil {
		t.Fatalf("SaveAll flag update failed: %v", err)
	}

	// Verify flags were set
	t1, _ := repo.GetByID("t1")
	if t1.IsEnabled != false {
		t.Fatalf("Expected t1.IsEnabled=false after disable")
	}

	// Step 3: MCP server updates schemas (simulating refresh)
	// Caller (refreshMCPTools) reuses existing IDs and preserves flags
	existing := repo.GetAllByMCPServerID("server1")
	existingMap := make(map[string]*Tool)
	for _, e := range existing {
		existingMap[e.Name] = e
	}

	freshTools := []*Tool{
		{ID: "new-uuid-1", MCPServerID: "server1", Name: "search", Description: "Search v2 UPDATED", InputSchema: `{"v":2,"added":"field"}`},
		{ID: "new-uuid-2", MCPServerID: "server1", Name: "delete", Description: "Delete v2 UPDATED", InputSchema: `{"v":2}`},
		{ID: "new-uuid-3", MCPServerID: "server1", Name: "brand_new", Description: "New tool", InputSchema: `{"type":"object"}`, IsEnabled: true},
	}

	newToolIDs := make([]string, 0, len(freshTools))
	for _, ft := range freshTools {
		if ex, ok := existingMap[ft.Name]; ok {
			ft.ID = ex.ID
			ft.IsEnabled = ex.IsEnabled
			ft.RequireApproval = ex.RequireApproval
		}
		newToolIDs = append(newToolIDs, ft.ID)
	}

	if err := repo.UpsertAll(freshTools); err != nil {
		t.Fatalf("Refresh UpsertAll failed: %v", err)
	}
	if err := repo.DeleteNotIn("server1", newToolIDs); err != nil {
		t.Fatalf("DeleteNotIn failed: %v", err)
	}

	// Verify results
	all := repo.GetAllByMCPServerID("server1")
	if len(all) != 3 {
		t.Fatalf("Expected 3 tools after refresh, got %d", len(all))
	}

	toolsByName := make(map[string]*Tool)
	for _, tool := range all {
		toolsByName[tool.Name] = tool
	}

	// "search": schema updated, still disabled
	search := toolsByName["search"]
	if search.Description != "Search v2 UPDATED" {
		t.Errorf("search description not updated: got %q", search.Description)
	}
	if search.InputSchema != `{"v":2,"added":"field"}` {
		t.Errorf("search schema not updated: got %q", search.InputSchema)
	}
	if search.IsEnabled != false {
		t.Errorf("search should still be disabled after refresh")
	}
	if search.ID != "t1" {
		t.Errorf("search should keep original ID 't1', got %q", search.ID)
	}

	// "delete": schema updated, require_approval preserved
	del := toolsByName["delete"]
	if del.Description != "Delete v2 UPDATED" {
		t.Errorf("delete description not updated: got %q", del.Description)
	}
	if del.RequireApproval != true {
		t.Errorf("delete should still have require_approval=true after refresh")
	}
	if del.ID != "t2" {
		t.Errorf("delete should keep original ID 't2', got %q", del.ID)
	}

	// "brand_new": new tool with defaults
	brandNew := toolsByName["brand_new"]
	if brandNew.IsEnabled != true {
		t.Errorf("brand_new should default to enabled")
	}
	if brandNew.Description != "New tool" {
		t.Errorf("brand_new description wrong: got %q", brandNew.Description)
	}
}
