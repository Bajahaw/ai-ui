package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func startMutableMCPServer(t *testing.T) (string, *mcp.Server) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "mutable", Version: "v1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echo"}, func(_ context.Context, _ *mcp.CallToolRequest, args echoArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: args.Text}}}, nil, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	t.Cleanup(httpServer.Close)
	return httpServer.URL, server
}

func addPingTool(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "ping", Description: "ping"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{}, nil, nil
	})
}

// makeDue pretends the last check ran long ago.
func makeDue(t *testing.T, id string) {
	t.Helper()
	if err := mcps.UpdateSyncState(id, "testuser", false, 0); err != nil {
		t.Fatal(err)
	}
}

func TestMCPUpdateCheck_AutoUpdateAppliesChanges(t *testing.T) {
	endpoint, srv := startMutableMCPServer(t)
	setupOAuthTest(t)

	saved := saveServerReq(t, map[string]any{
		"name": "auto", "endpoint": endpoint, "api_key": "", "headers": map[string]string{}, "auto_update": true,
	})
	if !saved.AutoUpdate {
		t.Fatal("auto_update not saved")
	}

	addPingTool(srv)
	// Just saved, so not due yet.
	if res := CheckMCPUpdates("testuser"); len(res.Updated)+len(res.Pending) != 0 {
		t.Fatalf("check should be throttled, got %+v", res)
	}

	makeDue(t, saved.ID)
	res := CheckMCPUpdates("testuser")
	if !slices.Equal(res.Updated, []string{saved.ID}) || len(res.Pending) != 0 {
		t.Fatalf("got %+v", res)
	}
	server, _ := mcps.GetByID(saved.ID, "testuser")
	if len(server.Tools) != 2 || server.UpdatePending {
		t.Fatalf("tools=%d pending=%v", len(server.Tools), server.UpdatePending)
	}
	if time.Since(time.Unix(server.LastCheckedAt, 0)) > time.Minute {
		t.Fatal("last_checked_at not recorded")
	}
}

func TestMCPUpdateCheck_FlagsWhenAutoUpdateOff(t *testing.T) {
	endpoint, srv := startMutableMCPServer(t)
	setupOAuthTest(t)

	saved := saveServerReq(t, map[string]any{
		"name": "manual", "endpoint": endpoint, "api_key": "", "headers": map[string]string{},
	})
	if saved.AutoUpdate {
		t.Fatal("auto_update should default to off")
	}

	makeDue(t, saved.ID)
	if res := CheckMCPUpdates("testuser"); len(res.Updated)+len(res.Pending) != 0 {
		t.Fatalf("unchanged server reported %+v", res)
	}

	addPingTool(srv)
	makeDue(t, saved.ID)
	res := CheckMCPUpdates("testuser")
	if !slices.Equal(res.Pending, []string{saved.ID}) || len(res.Updated) != 0 {
		t.Fatalf("got %+v", res)
	}
	server, _ := mcps.GetByID(saved.ID, "testuser")
	if len(server.Tools) != 1 || !server.UpdatePending {
		t.Fatalf("tools must not change: tools=%d pending=%v", len(server.Tools), server.UpdatePending)
	}

	// A manual refresh applies it and clears the flag.
	r := withUser(httptest.NewRequest(http.MethodPost, "/mcp/refresh-tools/"+saved.ID, nil))
	r.SetPathValue("id", saved.ID)
	w := httptest.NewRecorder()
	refreshMCPTools(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("refresh: %d %s", w.Code, w.Body.String())
	}
	server, _ = mcps.GetByID(saved.ID, "testuser")
	if len(server.Tools) != 2 || server.UpdatePending {
		t.Fatalf("after refresh: tools=%d pending=%v", len(server.Tools), server.UpdatePending)
	}
}

func TestMCPUpdateCheck_UnreachableServerIsThrottled(t *testing.T) {
	endpoint, _ := startMutableMCPServer(t)
	setupOAuthTest(t)
	saved := saveServerReq(t, map[string]any{
		"name": "gone", "endpoint": endpoint, "api_key": "", "headers": map[string]string{},
	})

	server, _ := mcps.GetByID(saved.ID, "testuser")
	server.Endpoint = "http://127.0.0.1:1/mcp"
	if err := mcps.Update(server); err != nil {
		t.Fatal(err)
	}
	makeDue(t, saved.ID)

	if res := CheckMCPUpdates("testuser"); len(res.Updated)+len(res.Pending) != 0 {
		t.Fatalf("got %+v", res)
	}
	server, _ = mcps.GetByID(saved.ID, "testuser")
	if len(server.Tools) != 1 {
		t.Fatalf("a failed check must not touch tools, got %d", len(server.Tools))
	}
	if server.LastCheckedAt == 0 {
		t.Fatal("failed check should still be recorded so it isn't retried on every load")
	}
}

func TestWaitForMCPUpdateCheck_WaitsForRun(t *testing.T) {
	WaitForMCPUpdateCheck("nobody") // no run: returns immediately

	run := &mcpUpdateRun{done: make(chan struct{})}
	mcpUpdateRuns.mu.Lock()
	mcpUpdateRuns.runs["waiter"] = run
	mcpUpdateRuns.mu.Unlock()
	t.Cleanup(func() {
		mcpUpdateRuns.mu.Lock()
		delete(mcpUpdateRuns.runs, "waiter")
		mcpUpdateRuns.mu.Unlock()
	})

	returned := make(chan struct{})
	go func() {
		WaitForMCPUpdateCheck("waiter")
		close(returned)
	}()
	select {
	case <-returned:
		t.Fatal("returned before the run finished")
	case <-time.After(50 * time.Millisecond):
	}
	close(run.done)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("did not return after the run finished")
	}
}

func TestMCPCatalogChanged(t *testing.T) {
	server := &MCPServer{
		Info:  MCPServerInfo{Name: "s", Version: "1"},
		Tools: []*Tool{{Name: "a", Description: "A", InputSchema: `{}`}, {Name: "b", Description: "B", InputSchema: `{}`}},
	}
	same := []*Tool{{Name: "b", Description: "B", InputSchema: `{}`}, {Name: "a", Description: "A", InputSchema: `{}`}}
	if mcpCatalogChanged(server, same, server.Info) {
		t.Error("reordered tools should not count as a change")
	}
	if !mcpCatalogChanged(server, same, MCPServerInfo{Name: "s", Version: "2"}) {
		t.Error("version change not detected")
	}
	if !mcpCatalogChanged(server, []*Tool{same[0], {Name: "a", Description: "A", InputSchema: `{"x":1}`}}, server.Info) {
		t.Error("schema change not detected")
	}
	if !mcpCatalogChanged(server, same[:1], server.Info) {
		t.Error("removed tool not detected")
	}
}
