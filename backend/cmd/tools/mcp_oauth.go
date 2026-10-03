package tools

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/utils"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

const (
	AuthTypeOAuth2        = "oauth2"
	mcpOAuthCallbackPath  = "/api/tools/mcp/oauth/callback"
	mcpOAuthPendingTTL    = 10 * time.Minute
	mcpOAuthClientName    = "AI UI"
	mcpOAuthBroadcastName = "mcp-oauth"
)

var mcpOAuthHTTPClient = &http.Client{Timeout: 30 * time.Second}

// MCPOAuth holds the OAuth2 settings a user enters for an MCP server, plus the
// session negotiated by the last successful authorization.
// ClientID may be blank, in which case Dynamic Client Registration is used.
// AuthURL / TokenURL are optional overrides; blank means auto-discover.
type MCPOAuth struct {
	ClientID     string           `json:"client_id,omitempty"`
	ClientSecret string           `json:"client_secret,omitempty"`
	Scopes       string           `json:"scopes,omitempty"`
	AuthURL      string           `json:"auth_url,omitempty"`
	TokenURL     string           `json:"token_url,omitempty"`
	Session      *MCPOAuthSession `json:"session,omitempty"`
}

// MCPOAuthSession is everything needed to use and refresh the issued token.
type MCPOAuthSession struct {
	ClientID     string           `json:"client_id"`
	ClientSecret string           `json:"client_secret,omitempty"`
	TokenURL     string           `json:"token_url"`
	AuthStyle    oauth2.AuthStyle `json:"auth_style"`
	Scopes       []string         `json:"scopes,omitempty"`
	Token        *oauth2.Token    `json:"token"`
	ConnectedAt  time.Time        `json:"connected_at"`
}

func (o *MCPOAuth) connected() bool {
	return o != nil && o.Session != nil && o.Session.Token != nil
}

func (s *MCPOAuthSession) config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.ClientID,
		ClientSecret: s.ClientSecret,
		Endpoint:     oauth2.Endpoint{TokenURL: s.TokenURL, AuthStyle: s.AuthStyle},
		Scopes:       s.Scopes,
	}
}

// MCPOAuthRequest is the OAuth part of MCPServerRequest. A blank ClientSecret
// on update keeps the stored one.
type MCPOAuthRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scopes       string `json:"scopes"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
}

// MCPOAuthResponse never includes the client secret or tokens.
type MCPOAuthResponse struct {
	ClientID        string `json:"client_id"`
	HasClientSecret bool   `json:"has_client_secret"`
	Scopes          string `json:"scopes"`
	AuthURL         string `json:"auth_url"`
	TokenURL        string `json:"token_url"`
	Connected       bool   `json:"connected"`
}

func toMCPOAuthResponse(o *MCPOAuth) *MCPOAuthResponse {
	if o == nil {
		return nil
	}
	return &MCPOAuthResponse{
		ClientID:        o.ClientID,
		HasClientSecret: o.ClientSecret != "",
		Scopes:          o.Scopes,
		AuthURL:         o.AuthURL,
		TokenURL:        o.TokenURL,
		Connected:       o.connected(),
	}
}

// mergeMCPOAuth builds the OAuth settings for a save request. The stored session
// survives only while the settings that shaped it stay the same.
func mergeMCPOAuth(req *MCPOAuthRequest, endpoint string, existing *MCPServer) *MCPOAuth {
	if req == nil {
		req = &MCPOAuthRequest{}
	}
	o := &MCPOAuth{
		ClientID:     strings.TrimSpace(req.ClientID),
		ClientSecret: req.ClientSecret,
		Scopes:       strings.TrimSpace(req.Scopes),
		AuthURL:      strings.TrimSpace(req.AuthURL),
		TokenURL:     strings.TrimSpace(req.TokenURL),
	}
	if existing == nil || existing.OAuth == nil {
		return o
	}
	prev := existing.OAuth
	if o.ClientSecret == "" && o.ClientID == prev.ClientID {
		o.ClientSecret = prev.ClientSecret
	}
	if endpoint == existing.Endpoint && existing.AuthType == AuthTypeOAuth2 &&
		o.ClientID == prev.ClientID && o.ClientSecret == prev.ClientSecret &&
		o.Scopes == prev.Scopes && o.AuthURL == prev.AuthURL && o.TokenURL == prev.TokenURL {
		o.Session = prev.Session
	}
	return o
}

// oauthTokenSource returns a refreshing token source that writes rotated tokens back to the database.
func oauthTokenSource(server MCPServer) (oauth2.TokenSource, error) {
	if !server.OAuth.connected() {
		return nil, errors.New("MCP server is not authorized yet; connect it with OAuth first")
	}
	sess := server.OAuth.Session
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, mcpOAuthHTTPClient)
	return &persistingTokenSource{
		base: sess.config().TokenSource(ctx, sess.Token),
		last: sess.Token.AccessToken,
		save: func(tok *oauth2.Token) {
			if err := mcps.UpdateOAuthToken(server.ID, server.User, sess.ConnectedAt, tok); err != nil {
				log.Warn("Failed to persist refreshed MCP OAuth token", "server", server.ID, "err", err)
			}
		},
	}, nil
}

type persistingTokenSource struct {
	mu   sync.Mutex
	base oauth2.TokenSource
	last string
	save func(*oauth2.Token)
}

func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.base.Token()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	changed := tok.AccessToken != s.last
	s.last = tok.AccessToken
	s.mu.Unlock()
	if changed {
		s.save(tok)
	}
	return tok, nil
}

// --- Authorization flow ---

type pendingMCPOAuth struct {
	state          string
	user           string
	serverID       string
	config         *oauth2.Config
	verifier       string
	resource       string
	issuer         string
	issSupported   bool
	redirectOrigin string
	expires        time.Time
}

type mcpOAuthFlows struct {
	mu      sync.Mutex
	pending map[string]*pendingMCPOAuth
}

var mcpOAuthPending = &mcpOAuthFlows{pending: make(map[string]*pendingMCPOAuth)}

func (f *mcpOAuthFlows) put(state string, p *pendingMCPOAuth) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for k, v := range f.pending {
		if now.After(v.expires) {
			delete(f.pending, k)
		}
	}
	f.pending[state] = p
}

// take removes and returns the flow for state; each state is single-use.
func (f *mcpOAuthFlows) take(state string) *pendingMCPOAuth {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.pending[state]
	if !ok {
		return nil
	}
	delete(f.pending, state)
	if time.Now().After(p.expires) {
		return nil
	}
	return p
}

// mcpOAuthRedirectURL is the callback URL users register with their OAuth app.
// PUBLIC_URL wins; otherwise the browser's origin, so it works behind proxies
// and the Vite dev server.
func mcpOAuthRedirectURL(r *http.Request) string {
	base := strings.TrimSpace(os.Getenv("PUBLIC_URL"))
	if base == "" {
		base = browserOrigin(r)
	}
	if base == "" {
		base = utils.GetServerURL(r)
	}
	return strings.TrimRight(base, "/") + mcpOAuthCallbackPath
}

// browserOrigin returns the page origin from Origin (sent on POST) or Referer (sent on same-origin GET).
func browserOrigin(r *http.Request) string {
	if o := r.Header.Get("Origin"); o != "" && o != "null" {
		return o
	}
	if u, err := url.Parse(r.Header.Get("Referer")); err == nil && u.Scheme != "" && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return ""
}

func getMCPOAuthRedirectURL(w http.ResponseWriter, r *http.Request) {
	utils.RespondWithJSON(w, map[string]string{"redirect_url": mcpOAuthRedirectURL(r)}, http.StatusOK)
}

type mcpOAuthStartResponse struct {
	AuthURL string `json:"auth_url"`
}

func startMCPOAuth(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	server, err := mcps.GetByID(r.PathValue("id"), user)
	if err != nil {
		http.Error(w, "MCP server not found", http.StatusNotFound)
		return
	}
	if server.AuthType != AuthTypeOAuth2 || server.OAuth == nil {
		http.Error(w, "MCP server is not configured for OAuth2", http.StatusBadRequest)
		return
	}

	redirectURL := mcpOAuthRedirectURL(r)
	authURL, pending, err := buildMCPAuthRequest(r.Context(), server, redirectURL)
	if err != nil {
		log.Error("Failed to start MCP OAuth", "server", server.ID, "err", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	pending.user = user
	if u, err := url.Parse(redirectURL); err == nil {
		pending.redirectOrigin = u.Scheme + "://" + u.Host
	}
	mcpOAuthPending.put(pending.state, pending)
	utils.RespondWithJSON(w, mcpOAuthStartResponse{AuthURL: authURL}, http.StatusOK)
}

// buildMCPAuthRequest resolves endpoints and client credentials, then returns
// the authorization URL the user must visit.
func buildMCPAuthRequest(ctx context.Context, server *MCPServer, redirectURL string) (string, *pendingMCPOAuth, error) {
	o := server.OAuth
	disc, err := discoverMCPAuth(ctx, server.Endpoint)
	if err != nil && (o.AuthURL == "" || o.TokenURL == "") {
		return "", nil, err
	}
	if disc == nil {
		disc = &mcpAuthDiscovery{asm: &oauthex.AuthServerMeta{}}
	}
	asm := disc.asm
	if o.AuthURL != "" {
		asm.AuthorizationEndpoint = o.AuthURL
	}
	if o.TokenURL != "" {
		asm.TokenEndpoint = o.TokenURL
	}
	if asm.AuthorizationEndpoint == "" || asm.TokenEndpoint == "" {
		return "", nil, errors.New("could not determine the authorization or token URL; set them manually")
	}

	clientID, clientSecret, authStyle := o.ClientID, o.ClientSecret, selectTokenAuthStyle(o.ClientSecret, asm.TokenEndpointAuthMethodsSupported)
	if clientID == "" {
		if asm.RegistrationEndpoint == "" {
			return "", nil, errors.New("this server does not support dynamic client registration; enter a Client ID and Client Secret")
		}
		reg, err := oauthex.RegisterClient(ctx, asm.RegistrationEndpoint, &oauthex.ClientRegistrationMetadata{
			RedirectURIs:            []string{redirectURL},
			TokenEndpointAuthMethod: "none",
			GrantTypes:              []string{"authorization_code", "refresh_token"},
			ResponseTypes:           []string{"code"},
			ClientName:              mcpOAuthClientName,
		}, mcpOAuthHTTPClient)
		if err != nil {
			return "", nil, fmt.Errorf("dynamic client registration failed: %w", err)
		}
		clientID, clientSecret = reg.ClientID, reg.ClientSecret
		authStyle = authStyleForMethod(reg.TokenEndpointAuthMethod)
	}

	scopes := strings.FieldsFunc(o.Scopes, func(r rune) bool { return r == ' ' || r == ',' })
	if len(scopes) == 0 {
		scopes = disc.scopes
	}
	if slices.Contains(asm.ScopesSupported, "offline_access") && !slices.Contains(scopes, "offline_access") {
		scopes = append(scopes, "offline_access")
	}

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   asm.AuthorizationEndpoint,
			TokenURL:  asm.TokenEndpoint,
			AuthStyle: authStyle,
		},
		RedirectURL: redirectURL,
		Scopes:      scopes,
	}
	state := rand.Text()
	verifier := oauth2.GenerateVerifier()
	opts := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier)}
	if disc.resource != "" {
		opts = append(opts, oauth2.SetAuthURLParam("resource", disc.resource))
	}
	return cfg.AuthCodeURL(state, opts...), &pendingMCPOAuth{
		state:        state,
		serverID:     server.ID,
		config:       cfg,
		verifier:     verifier,
		resource:     disc.resource,
		issuer:       asm.Issuer,
		issSupported: asm.AuthorizationResponseIssParameterSupported,
		expires:      time.Now().Add(mcpOAuthPendingTTL),
	}, nil
}

func selectTokenAuthStyle(secret string, supported []string) oauth2.AuthStyle {
	if secret == "" {
		return oauth2.AuthStyleInParams
	}
	for _, m := range []string{"client_secret_post", "client_secret_basic"} {
		if slices.Contains(supported, m) {
			return authStyleForMethod(m)
		}
	}
	return oauth2.AuthStyleAutoDetect
}

func authStyleForMethod(method string) oauth2.AuthStyle {
	switch method {
	case "client_secret_post", "none":
		return oauth2.AuthStyleInParams
	default:
		return oauth2.AuthStyleInHeader
	}
}

type mcpAuthDiscovery struct {
	asm      *oauthex.AuthServerMeta
	resource string
	scopes   []string
}

// discoverMCPAuth follows the MCP authorization spec: probe the endpoint for a
// WWW-Authenticate challenge, read Protected Resource Metadata, then fetch the
// Authorization Server Metadata, falling back to the 2025-03-26 defaults.
func discoverMCPAuth(ctx context.Context, endpoint string) (*mcpAuthDiscovery, error) {
	var challenges []oauthex.Challenge
	if req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(`{"jsonrpc":"2.0","id":0,"method":"ping"}`)); err == nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if resp, err := mcpOAuthHTTPClient.Do(req); err == nil {
			challenges, _ = oauthex.ParseWWWAuthenticate(resp.Header.Values("WWW-Authenticate"))
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
			_ = resp.Body.Close()
		}
	}

	d := &mcpAuthDiscovery{}
	var metadataURL string
	for _, c := range challenges {
		if metadataURL == "" {
			metadataURL = c.Params["resource_metadata"]
		}
		if c.Scheme == "bearer" && c.Params["scope"] != "" && d.scopes == nil {
			d.scopes = strings.Fields(c.Params["scope"])
		}
	}

	issuer := ""
	for _, u := range protectedResourceMetadataURLs(metadataURL, endpoint) {
		prm, err := oauthex.GetProtectedResourceMetadata(ctx, u.url, u.resource, mcpOAuthHTTPClient)
		if err != nil || prm == nil {
			continue
		}
		if len(prm.AuthorizationServers) == 0 {
			return nil, errors.New("protected resource metadata lists no authorization servers")
		}
		issuer, d.resource = prm.AuthorizationServers[0], prm.Resource
		if d.scopes == nil {
			d.scopes = prm.ScopesSupported
		}
		break
	}
	if issuer == "" {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid endpoint: %w", err)
		}
		issuer = u.Scheme + "://" + u.Host
	}

	asm, err := auth.GetAuthServerMetadata(ctx, issuer, mcpOAuthHTTPClient)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch authorization server metadata: %w", err)
	}
	if asm == nil {
		asm = &oauthex.AuthServerMeta{
			Issuer:                issuer,
			AuthorizationEndpoint: issuer + "/authorize",
			TokenEndpoint:         issuer + "/token",
			RegistrationEndpoint:  issuer + "/register",
		}
	}
	d.asm = asm
	return d, nil
}

type prmCandidate struct{ url, resource string }

func protectedResourceMetadataURLs(metadataURL, endpoint string) []prmCandidate {
	var out []prmCandidate
	if metadataURL != "" {
		out = append(out, prmCandidate{metadataURL, endpoint})
	}
	ru, err := url.Parse(endpoint)
	if err != nil {
		return out
	}
	mu := *ru
	mu.RawQuery = ""
	mu.Path = "/.well-known/oauth-protected-resource/" + strings.TrimLeft(ru.Path, "/")
	out = append(out, prmCandidate{mu.String(), endpoint})
	mu.Path = "/.well-known/oauth-protected-resource"
	root := *ru
	root.Path, root.RawQuery = "", ""
	return append(out, prmCandidate{mu.String(), root.String()})
}

func mcpOAuthCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p := mcpOAuthPending.take(q.Get("state"))
	if p == nil {
		renderMCPOAuthResult(w, "", "", "This authorization link is invalid or has expired. Start the connection again from settings.")
		return
	}
	fail := func(msg string) { renderMCPOAuthResult(w, p.redirectOrigin, p.serverID, msg) }

	if e := q.Get("error"); e != "" {
		if d := q.Get("error_description"); d != "" {
			e += ": " + d
		}
		fail("Authorization was denied: " + e)
		return
	}
	if iss := q.Get("iss"); (p.issSupported && iss == "") || (iss != "" && p.issuer != "" && iss != p.issuer) {
		fail("Authorization response came from an unexpected issuer.")
		return
	}
	code := q.Get("code")
	if code == "" {
		fail("Authorization response did not include a code.")
		return
	}

	opts := []oauth2.AuthCodeOption{oauth2.VerifierOption(p.verifier)}
	if p.resource != "" {
		opts = append(opts, oauth2.SetAuthURLParam("resource", p.resource))
	}
	ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), oauth2.HTTPClient, mcpOAuthHTTPClient), mcpConnectTimeout)
	defer cancel()
	token, err := p.config.Exchange(ctx, code, opts...)
	if err != nil {
		log.Error("MCP OAuth token exchange failed", "server", p.serverID, "err", err)
		fail("Token exchange failed: " + err.Error())
		return
	}

	server, err := mcps.GetByID(p.serverID, p.user)
	if err != nil || server.AuthType != AuthTypeOAuth2 || server.OAuth == nil {
		fail("The MCP server no longer exists or is no longer set up for OAuth2.")
		return
	}
	server.OAuth.Session = &MCPOAuthSession{
		ClientID:     p.config.ClientID,
		ClientSecret: p.config.ClientSecret,
		TokenURL:     p.config.Endpoint.TokenURL,
		AuthStyle:    p.config.Endpoint.AuthStyle,
		Scopes:       p.config.Scopes,
		Token:        token,
		ConnectedAt:  time.Now().UTC(),
	}
	if err := mcps.UpdateOAuth(server); err != nil {
		log.Error("Failed to save MCP OAuth session", "server", server.ID, "err", err)
		fail("Failed to save the token.")
		return
	}
	mcpSessionManager.invalidate(server.ID)

	freshTools, info, err := GetMCPTools(*server)
	if err == nil {
		err = storeCatalog(server.ID, server.User, freshTools, info)
	}
	if err != nil {
		log.Error("MCP OAuth connected but listing tools failed", "server", server.ID, "err", err)
		fail("Authorized, but fetching tools failed: " + err.Error())
		return
	}
	renderMCPOAuthResult(w, p.redirectOrigin, server.ID, "")
}

var mcpOAuthResultPage = template.Must(template.New("result").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>MCP authorization</title>
<style>body{font-family:system-ui,sans-serif;background:#1a1a1a;color:#e5e5e5;display:grid;place-items:center;min-height:100vh;margin:0}main{max-width:28rem;padding:2rem;text-align:center}p{color:#a3a3a3}</style>
</head><body><main>
{{if .Error}}<h1>Connection failed</h1><p>{{.Error}}</p>{{else}}<h1>Connected</h1><p>You can close this window.</p>{{end}}
</main>
<script>
(function () {
  var msg = {type: "mcp-oauth", serverId: {{.ServerID}}, error: {{.Error}}};
  try { new BroadcastChannel({{.Channel}}).postMessage(msg); } catch (e) {}
  try { if (window.opener && {{.Origin}}) window.opener.postMessage(msg, {{.Origin}}); } catch (e) {}
  {{if not .Error}}setTimeout(function () { window.close(); }, 800);{{end}}
})();
</script>
</body></html>`))

func renderMCPOAuthResult(w http.ResponseWriter, origin, serverID, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	status := http.StatusOK
	if errMsg != "" {
		status = http.StatusBadRequest
	}
	w.WriteHeader(status)
	_ = mcpOAuthResultPage.Execute(w, map[string]string{
		"Origin":   origin,
		"ServerID": serverID,
		"Error":    errMsg,
		"Channel":  mcpOAuthBroadcastName,
	})
}
