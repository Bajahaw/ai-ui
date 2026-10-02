package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/encryption"
	logger "github.com/charmbracelet/log"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

type fakeOAuthEnv struct {
	mcpURL        string
	authURL       string
	registrations atomic.Int32
	refreshes     atomic.Int32

	mu         sync.Mutex
	challenges map[string]string // code -> code_challenge
	validToken map[string]bool
	lastSecret string
}

// startFakeOAuthEnv runs an authorization server and an MCP server that only
// accepts tokens it issued, wired together via protected resource metadata.
func startFakeOAuthEnv(t *testing.T) *fakeOAuthEnv {
	t.Helper()
	env := &fakeOAuthEnv{challenges: map[string]string{}, validToken: map[string]bool{}}

	authMux := http.NewServeMux()
	authSrv := httptest.NewServer(authMux)
	t.Cleanup(authSrv.Close)
	env.authURL = authSrv.URL

	authMux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, http.StatusOK, map[string]any{
			"issuer":                           env.authURL,
			"authorization_endpoint":           env.authURL + "/authorize",
			"token_endpoint":                   env.authURL + "/token",
			"registration_endpoint":            env.authURL + "/register",
			"code_challenge_methods_supported": []string{"S256"},
			"scopes_supported":                 []string{"mcp", "offline_access"},
		})
	})
	authMux.HandleFunc("POST /register", func(w http.ResponseWriter, r *http.Request) {
		env.registrations.Add(1)
		var meta map[string]any
		_ = json.NewDecoder(r.Body).Decode(&meta)
		writeTestJSON(w, http.StatusCreated, map[string]any{
			"client_id":                  "dyn-client",
			"redirect_uris":              meta["redirect_uris"],
			"token_endpoint_auth_method": "none",
		})
	})
	authMux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		env.mu.Lock()
		defer env.mu.Unlock()
		if _, secret, ok := r.BasicAuth(); ok {
			env.lastSecret = secret
		} else {
			env.lastSecret = r.Form.Get("client_secret")
		}
		var access string
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			challenge, ok := env.challenges[r.Form.Get("code")]
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			delete(env.challenges, r.Form.Get("code"))
			access = "at-1"
		case "refresh_token":
			if r.Form.Get("refresh_token") != "rt-1" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			env.refreshes.Add(1)
			access = "at-2"
		default:
			http.Error(w, `{"error":"unsupported_grant_type"}`, http.StatusBadRequest)
			return
		}
		env.validToken[access] = true
		writeTestJSON(w, http.StatusOK, map[string]any{
			"access_token": access, "token_type": "Bearer", "refresh_token": "rt-1", "expires_in": 3600,
		})
	})

	server := mcp.NewServer(&mcp.Implementation{Name: "oauth-test", Version: "v1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echo"}, func(_ context.Context, _ *mcp.CallToolRequest, args echoArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: args.Text}}}, nil, nil
	})
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)

	mcpMux := http.NewServeMux()
	mcpSrv := httptest.NewServer(mcpMux)
	t.Cleanup(mcpSrv.Close)
	env.mcpURL = mcpSrv.URL + "/mcp"

	mcpMux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, http.StatusOK, map[string]any{
			"resource":              env.mcpURL,
			"authorization_servers": []string{env.authURL},
			"scopes_supported":      []string{"mcp"},
		})
	})
	mcpMux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		env.mu.Lock()
		ok := env.validToken[tok]
		env.mu.Unlock()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+mcpSrv.URL+`/.well-known/oauth-protected-resource/mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})
	return env
}

func writeTestJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// approve plays the authorization server's login page: it records the PKCE
// challenge and returns the callback query the browser would be sent to.
func (env *fakeOAuthEnv) approve(t *testing.T, authURL string) url.Values {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	q := u.Query()
	env.mu.Lock()
	env.challenges["the-code"] = q.Get("code_challenge")
	env.mu.Unlock()
	return url.Values{"code": {"the-code"}, "state": {q.Get("state")}}
}

// setupOAuthTest must run after startFakeOAuthEnv so cached sessions close
// before the test servers (httptest.Server.Close waits on open SSE streams).
func setupOAuthTest(t *testing.T) {
	t.Helper()
	db, toolRepo := setupTestDB(t)
	log = logger.New(os.Stderr)
	tools = toolRepo
	mcps = NewMCPRepository(db, toolRepo)
	mcpSessionManager = newMCPSessionManager()
	t.Cleanup(mcpSessionManager.closeAll)
}

func withUser(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), "user", "testuser"))
}

func saveServerReq(t *testing.T, body map[string]any) MCPServerResponse {
	t.Helper()
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	saveMCPServer(w, withUser(httptest.NewRequest(http.MethodPost, "/mcp/save", bytes.NewReader(raw))))
	if w.Code != http.StatusOK {
		t.Fatalf("save: status %d: %s", w.Code, w.Body.String())
	}
	var resp MCPServerResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	return resp
}

func startOAuthReq(t *testing.T, id string) string {
	t.Helper()
	r := withUser(httptest.NewRequest(http.MethodPost, "/mcp/oauth/start/"+id, nil))
	r.SetPathValue("id", id)
	r.Header.Set("Origin", "http://app.local")
	w := httptest.NewRecorder()
	startMCPOAuth(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("start: status %d: %s", w.Code, w.Body.String())
	}
	var resp mcpOAuthStartResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.AuthURL
}

func callbackReq(q url.Values) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	mcpOAuthCallback(w, httptest.NewRequest(http.MethodGet, "/mcp/oauth/callback?"+q.Encode(), nil))
	return w
}

func TestMCPOAuth_DynamicRegistrationFlow(t *testing.T) {
	env := startFakeOAuthEnv(t)
	setupOAuthTest(t)

	saved := saveServerReq(t, map[string]any{
		"name": "OAuth MCP", "endpoint": env.mcpURL, "api_key": "", "headers": map[string]string{},
		"auth_type": "oauth2", "oauth": map[string]string{},
	})
	if saved.OAuth == nil || saved.OAuth.Connected {
		t.Fatalf("expected unconnected oauth server, got %+v", saved.OAuth)
	}

	authURL := startOAuthReq(t, saved.ID)
	q, _ := url.Parse(authURL)
	params := q.Query()
	if got := params.Get("client_id"); got != "dyn-client" {
		t.Errorf("client_id = %q, want dyn-client", got)
	}
	if got := params.Get("redirect_uri"); got != "http://app.local"+mcpOAuthCallbackPath {
		t.Errorf("redirect_uri = %q", got)
	}
	if got := params.Get("resource"); got != env.mcpURL {
		t.Errorf("resource = %q, want %q", got, env.mcpURL)
	}
	if got := params.Get("scope"); got != "mcp offline_access" {
		t.Errorf("scope = %q", got)
	}
	if env.registrations.Load() != 1 {
		t.Errorf("expected 1 dynamic registration, got %d", env.registrations.Load())
	}

	cb := env.approve(t, authURL)
	w := callbackReq(cb)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Connected") {
		t.Fatalf("callback: status %d: %s", w.Code, w.Body.String())
	}

	server, err := mcps.GetByID(saved.ID, "testuser")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !server.OAuth.connected() || server.OAuth.Session.Token.AccessToken != "at-1" {
		t.Fatalf("expected stored token at-1, got %+v", server.OAuth.Session)
	}
	if len(server.Tools) != 1 || server.Tools[0].Name != "echo" {
		t.Fatalf("expected echo tool after connect, got %+v", server.Tools)
	}

	var rawOAuth string
	if err := mcps.(*MCPRepositoryImpl).db.QueryRow(`SELECT oauth_json FROM MCPServers WHERE id = ?`, saved.ID).Scan(&rawOAuth); err != nil {
		t.Fatalf("read oauth_json: %v", err)
	}
	if !encryption.IsEncrypted(rawOAuth) || strings.Contains(rawOAuth, "at-1") {
		t.Fatal("oauth_json must be encrypted at rest")
	}

	if w := callbackReq(cb); w.Code != http.StatusBadRequest {
		t.Fatalf("replayed state should be rejected, got %d", w.Code)
	}

	// The list response must never leak tokens.
	lw := httptest.NewRecorder()
	listMCPServers(lw, withUser(httptest.NewRequest(http.MethodGet, "/mcp/all", nil)))
	if strings.Contains(lw.Body.String(), "at-1") || strings.Contains(lw.Body.String(), "rt-1") {
		t.Fatal("list response leaked OAuth tokens")
	}
}

func TestMCPOAuth_RefreshesAndPersistsToken(t *testing.T) {
	env := startFakeOAuthEnv(t)
	setupOAuthTest(t)

	saved := saveServerReq(t, map[string]any{
		"name": "OAuth MCP", "endpoint": env.mcpURL, "api_key": "", "headers": map[string]string{},
		"auth_type": "oauth2", "oauth": map[string]string{},
	})
	if w := callbackReq(env.approve(t, startOAuthReq(t, saved.ID))); w.Code != http.StatusOK {
		t.Fatalf("callback: %d %s", w.Code, w.Body.String())
	}

	// Expire the stored token and drop the cached session.
	server, _ := mcps.GetByID(saved.ID, "testuser")
	connectedAt := server.OAuth.Session.ConnectedAt
	expired := *server.OAuth.Session.Token
	expired.Expiry = time.Now().Add(-time.Hour)
	if err := mcps.UpdateOAuthToken(server.ID, server.User, connectedAt, &expired); err != nil {
		t.Fatalf("expire token: %v", err)
	}
	mcpSessionManager.invalidate(server.ID)

	server, _ = mcps.GetByID(saved.ID, "testuser")
	res, err := mcpSessionManager.callTool(context.Background(), *server, &mcp.CallToolParams{
		Name: "echo", Arguments: map[string]any{"text": "hi"},
	})
	if err != nil {
		t.Fatalf("callTool after expiry: %v", err)
	}
	if txt, _ := res.Content[0].(*mcp.TextContent); txt == nil || txt.Text != "hi" {
		t.Fatalf("unexpected result %#v", res.Content)
	}
	if env.refreshes.Load() == 0 {
		t.Fatal("expected a refresh_token grant")
	}
	server, _ = mcps.GetByID(saved.ID, "testuser")
	if got := server.OAuth.Session.Token.AccessToken; got != "at-2" {
		t.Fatalf("refreshed token not persisted, got %q", got)
	}
}

func TestMCPOAuth_PreregisteredClientSkipsRegistration(t *testing.T) {
	env := startFakeOAuthEnv(t)
	setupOAuthTest(t)

	saved := saveServerReq(t, map[string]any{
		"name": "OAuth MCP", "endpoint": env.mcpURL, "api_key": "", "headers": map[string]string{},
		"auth_type": "oauth2", "oauth": map[string]string{"client_id": "my-app", "client_secret": "s3cret"},
	})
	if !saved.OAuth.HasClientSecret || saved.OAuth.ClientID != "my-app" {
		t.Fatalf("unexpected oauth response %+v", saved.OAuth)
	}
	authURL := startOAuthReq(t, saved.ID)
	if got, _ := url.Parse(authURL); got.Query().Get("client_id") != "my-app" {
		t.Fatalf("expected configured client id in %s", authURL)
	}
	if env.registrations.Load() != 0 {
		t.Fatal("dynamic registration must not run when a Client ID is set")
	}
	if w := callbackReq(env.approve(t, authURL)); w.Code != http.StatusOK {
		t.Fatalf("callback: %d %s", w.Code, w.Body.String())
	}
	env.mu.Lock()
	defer env.mu.Unlock()
	if env.lastSecret != "s3cret" {
		t.Fatalf("token request did not authenticate with the client secret, got %q", env.lastSecret)
	}
}

func TestMergeMCPOAuth(t *testing.T) {
	sess := &MCPOAuthSession{ClientID: "a", Token: &oauth2.Token{AccessToken: "x"}}
	existing := &MCPServer{
		Endpoint: "https://mcp.example.com",
		AuthType: AuthTypeOAuth2,
		OAuth:    &MCPOAuth{ClientID: "a", ClientSecret: "secret", Session: sess},
	}

	got := mergeMCPOAuth(&MCPOAuthRequest{ClientID: "a"}, existing.Endpoint, existing)
	if got.ClientSecret != "secret" || got.Session != sess {
		t.Fatalf("unchanged settings should keep secret and session, got %+v", got)
	}

	got = mergeMCPOAuth(&MCPOAuthRequest{ClientID: "a", Scopes: "read"}, existing.Endpoint, existing)
	if got.Session != nil {
		t.Fatal("changing scopes should drop the session")
	}

	got = mergeMCPOAuth(&MCPOAuthRequest{ClientID: "b"}, existing.Endpoint, existing)
	if got.ClientSecret != "" || got.Session != nil {
		t.Fatalf("a new client id must not inherit the old secret or session, got %+v", got)
	}

	got = mergeMCPOAuth(&MCPOAuthRequest{ClientID: "a"}, "https://other.example.com", existing)
	if got.Session != nil {
		t.Fatal("changing the endpoint should drop the session")
	}
}
