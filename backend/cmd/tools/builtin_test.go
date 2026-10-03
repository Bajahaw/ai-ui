package tools

import (
	"slices"
	"testing"
)

// Fails whenever a built-in tool's name, description, schema or defaults
// change, so the displayed version is bumped along with it.
func TestBuiltInToolsVersionIsCurrent(t *testing.T) {
	if got := hashBuiltInTools(); got != builtInToolsHash {
		t.Fatalf("built-in tools changed: bump builtInToolsVersion (now %d) and set builtInToolsHash = %q",
			builtInToolsVersion, got)
	}
}

func TestCheckMCPUpdates_SyncsOutdatedBuiltInServer(t *testing.T) {
	setupOAuthTest(t)
	SaveDefaultMCPServer("testuser")
	id := "default-testuser"

	server, err := mcps.GetByID(id, "testuser")
	if err != nil {
		t.Fatal(err)
	}
	if server.Info != builtInServerInfo() {
		t.Fatalf("new default server should carry the current marker, got %+v", server.Info)
	}
	if res := CheckMCPUpdates("testuser"); slices.Contains(res.Updated, id) {
		t.Fatal("up-to-date built-ins must not be rewritten")
	}

	// Simulate a server saved by an older release: no marker, a tool that
	// didn't exist yet, and a tool the user disabled.
	all := len(server.Tools)
	var disabled *Tool
	for _, tool := range server.Tools {
		switch tool.Name {
		case "http_request":
			if err := tools.DeleteByID(tool.ID); err != nil {
				t.Fatal(err)
			}
		case "generate_image":
			copied := *tool
			copied.IsEnabled = false
			disabled = &copied
		}
	}
	if err := tools.SaveAll([]*Tool{disabled}); err != nil {
		t.Fatal(err)
	}
	if err := mcps.UpdateServerInfo(id, "testuser", MCPServerInfo{}); err != nil {
		t.Fatal(err)
	}

	if res := CheckMCPUpdates("testuser"); !slices.Contains(res.Updated, id) {
		t.Fatalf("outdated built-ins should be reported as updated, got %+v", res)
	}
	server, _ = mcps.GetByID(id, "testuser")
	if len(server.Tools) != all {
		t.Fatalf("missing built-in not restored: %d tools, want %d", len(server.Tools), all)
	}
	if server.Info != builtInServerInfo() {
		t.Fatalf("marker not stored: %+v", server.Info)
	}
	for _, tool := range server.Tools {
		if tool.Name == "generate_image" && tool.IsEnabled {
			t.Fatal("sync must keep the user's disabled flag")
		}
		if tool.Name == "http_request" && (!tool.IsEnabled || !tool.RequireApproval) {
			t.Fatal("a restored tool should get its defaults from code")
		}
	}

	if res := CheckMCPUpdates("testuser"); slices.Contains(res.Updated, id) {
		t.Fatal("second check should be a no-op")
	}
}
