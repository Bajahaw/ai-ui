package tools

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/singleflight"
)

const mcpConnectTimeout = 60 * time.Second

var mcpClientInfo = mcp.Implementation{Name: "ai-ui", Version: "v1.0.0"}

type sessionEntry struct {
	session     *mcp.ClientSession
	fingerprint string
}

type MCPSessionManager struct {
	client       *mcp.Client
	mu           sync.Mutex
	entries      map[string]*sessionEntry
	connectGroup singleflight.Group
}

func newMCPSessionManager() MCPSessionManager {
	return MCPSessionManager{
		client: mcp.NewClient(&mcpClientInfo, &mcp.ClientOptions{
			Capabilities: &mcp.ClientCapabilities{},
		}),
		entries: make(map[string]*sessionEntry),
	}
}

func (m *MCPSessionManager) session(ctx context.Context, server MCPServer) (*mcp.ClientSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	fp := sessionFingerprint(server)

	if sess := m.cached(server.ID, fp); sess != nil {
		return sess, nil
	}

	v, err, _ := m.connectGroup.Do(server.ID+"\x00"+fp, func() (any, error) {
		if sess := m.cached(server.ID, fp); sess != nil {
			return sess, nil
		}
		m.drop(server.ID)

		sess, err := m.connect(ctx, server)
		if err != nil {
			return nil, err
		}

		m.mu.Lock()
		m.entries[server.ID] = &sessionEntry{session: sess, fingerprint: fp}
		m.mu.Unlock()

		go m.watch(server.ID, sess)
		return sess, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*mcp.ClientSession), nil
}

func (m *MCPSessionManager) callTool(ctx context.Context, server MCPServer, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	sess, err := m.session(ctx, server)
	if err != nil {
		return nil, err
	}
	result, err := sess.CallTool(ctx, params)
	if err == nil || ctx.Err() != nil || !sessionDead(err) {
		return result, err
	}

	m.invalidate(server.ID)
	sess, err = m.session(ctx, server)
	if err != nil {
		return nil, err
	}
	return sess.CallTool(ctx, params)
}

func (m *MCPSessionManager) invalidate(serverID string) {
	m.drop(serverID)
}

func (m *MCPSessionManager) closeAll() {
	m.mu.Lock()
	entries := m.entries
	m.entries = make(map[string]*sessionEntry)
	m.mu.Unlock()
	for _, e := range entries {
		_ = e.session.Close()
	}
}

func (m *MCPSessionManager) cached(serverID, fingerprint string) *mcp.ClientSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[serverID]
	if !ok || e.fingerprint != fingerprint {
		return nil
	}
	return e.session
}

func (m *MCPSessionManager) drop(serverID string) {
	m.mu.Lock()
	e, ok := m.entries[serverID]
	if ok {
		delete(m.entries, serverID)
	}
	m.mu.Unlock()
	if ok {
		_ = e.session.Close()
	}
}

func (m *MCPSessionManager) watch(serverID string, sess *mcp.ClientSession) {
	_ = sess.Wait()
	m.mu.Lock()
	if e, ok := m.entries[serverID]; ok && e.session == sess {
		delete(m.entries, serverID)
	}
	m.mu.Unlock()
}

func (m *MCPSessionManager) connect(ctx context.Context, server MCPServer) (*mcp.ClientSession, error) {
	connectCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mcpConnectTimeout)
	defer cancel()

	headers := map[string]string{}
	if server.APIKey != "" {
		headers["Authorization"] = "Bearer " + server.APIKey
	}
	for k, v := range server.Headers {
		headers[k] = v
	}

	return m.client.Connect(connectCtx, &mcp.StreamableClientTransport{
		Endpoint:   server.Endpoint,
		HTTPClient: httpClientWithCustomHeaders(headers),
	}, nil)
}

func sessionDead(err error) bool {
	return errors.Is(err, mcp.ErrSessionMissing) || errors.Is(err, mcp.ErrConnectionClosed)
}

func sessionFingerprint(server MCPServer) string {
	var b strings.Builder
	b.WriteString(server.Endpoint)
	b.WriteByte('\n')
	b.WriteString(server.APIKey)
	if len(server.Headers) == 0 {
		return b.String()
	}
	keys := make([]string, 0, len(server.Headers))
	for k := range server.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteByte('\n')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(server.Headers[k])
	}
	return b.String()
}
