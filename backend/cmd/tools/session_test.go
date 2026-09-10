package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoArgs struct {
	Text string `json:"text"`
}

func startTestMCPServer(t *testing.T) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echo"}, func(_ context.Context, _ *mcp.CallToolRequest, args echoArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: args.Text}},
		}, nil, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)
	return httpServer.URL
}

func TestMCPSessionManager_ReusesSession(t *testing.T) {
	url := startTestMCPServer(t)
	m := newMCPSessionManager()
	t.Cleanup(m.closeAll)

	server := MCPServer{ID: "s1", Endpoint: url}
	a, err := m.session(context.Background(), server)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	b, err := m.session(context.Background(), server)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if a != b {
		t.Fatal("expected cached ClientSession to be reused")
	}
}

func TestMCPSessionManager_CallTool(t *testing.T) {
	url := startTestMCPServer(t)
	m := newMCPSessionManager()
	t.Cleanup(m.closeAll)

	server := MCPServer{ID: "s1", Endpoint: url}
	result, err := m.callTool(context.Background(), server, &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"text": "hi"},
	})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content len: %d", len(result.Content))
	}
	got, ok := result.Content[0].(*mcp.TextContent)
	if !ok || got.Text != "hi" {
		t.Fatalf("unexpected result: %#v", result.Content[0])
	}
}

func TestMCPSessionManager_CallToolConcurrent(t *testing.T) {
	url := startTestMCPServer(t)
	m := newMCPSessionManager()
	t.Cleanup(m.closeAll)

	server := MCPServer{ID: "s1", Endpoint: url}
	const n = 8
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := m.callTool(context.Background(), server, &mcp.CallToolParams{
				Name:      "echo",
				Arguments: map[string]any{"text": "x"},
			})
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent callTool: %v", err)
		}
	}
	m.mu.Lock()
	count := len(m.entries)
	m.mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 cached session, got %d", count)
	}
}

func TestMCPSessionManager_ReconnectsAfterInvalidate(t *testing.T) {
	url := startTestMCPServer(t)
	m := newMCPSessionManager()
	t.Cleanup(m.closeAll)

	server := MCPServer{ID: "s1", Endpoint: url}
	a, err := m.session(context.Background(), server)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	m.invalidate(server.ID)
	b, err := m.session(context.Background(), server)
	if err != nil {
		t.Fatalf("session after invalidate: %v", err)
	}
	if a == b {
		t.Fatal("expected a new ClientSession after invalidate")
	}
}

func TestMCPSessionManager_FingerprintChange(t *testing.T) {
	urlA := startTestMCPServer(t)
	urlB := startTestMCPServer(t)
	m := newMCPSessionManager()
	t.Cleanup(m.closeAll)

	server := MCPServer{ID: "s1", Endpoint: urlA}
	a, err := m.session(context.Background(), server)
	if err != nil {
		t.Fatalf("session a: %v", err)
	}
	server.Endpoint = urlB
	b, err := m.session(context.Background(), server)
	if err != nil {
		t.Fatalf("session b: %v", err)
	}
	if a == b {
		t.Fatal("expected a new ClientSession after endpoint change")
	}
}

func TestMCPSessionManager_SingleflightConnect(t *testing.T) {
	url := startTestMCPServer(t)
	m := newMCPSessionManager()
	t.Cleanup(m.closeAll)
	cfg := MCPServer{ID: "s1", Endpoint: url}

	const n = 10
	var wg sync.WaitGroup
	sessions := make([]*mcp.ClientSession, n)
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			sessions[i], errs[i] = m.session(context.Background(), cfg)
		}(i)
	}
	wg.Wait()

	var first *mcp.ClientSession
	for i, err := range errs {
		if err != nil {
			t.Fatalf("session %d: %v", i, err)
		}
		if first == nil {
			first = sessions[i]
		} else if sessions[i] != first {
			t.Fatal("singleflight produced distinct sessions")
		}
	}
}
