package chatgptoauth

import (
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const loginTimeout = 5 * time.Minute

// LoginStatus is the lifecycle of a pending OAuth login.
type LoginStatus string

const (
	StatusPending LoginStatus = "pending"
	StatusSuccess LoginStatus = "success"
	StatusError   LoginStatus = "error"
)

// PendingLogin tracks an in-flight OAuth attempt.
type PendingLogin struct {
	State        string
	CodeVerifier string
	RedirectURI  string
	// Username is set when an already-authenticated user is connecting ChatGPT.
	// Empty means "sign in / register from ChatGPT account".
	Username  string
	Status    LoginStatus
	Error     string
	Tokens    *Tokens
	CreatedAt time.Time
}

// LoginManager runs the localhost:1455 callback server and tracks pending logins.
type LoginManager struct {
	mu       sync.Mutex
	pending  map[string]*PendingLogin
	server   *http.Server
	listener net.Listener
}

// DefaultLoginManager is the process-wide manager (port 1455 is unique).
var DefaultLoginManager = NewLoginManager()

func NewLoginManager() *LoginManager {
	return &LoginManager{pending: make(map[string]*PendingLogin)}
}

// Start begins a new OAuth login. openURL is returned for the browser.
// The localhost:1455 callback server is best-effort: when the process cannot
// bind that port (or is remote and the browser will never hit it), the user can
// paste the redirect URL into the app via CompleteFromCallbackURL.
func (m *LoginManager) Start(username string) (authURL, state string, err error) {
	req, err := CreateAuthRequest(DefaultRedirect, DefaultClientID)
	if err != nil {
		return "", "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Drop expired pending entries.
	now := time.Now()
	for k, p := range m.pending {
		if now.Sub(p.CreatedAt) > loginTimeout {
			delete(m.pending, k)
		}
	}

	m.pending[req.State] = &PendingLogin{
		State:        req.State,
		CodeVerifier: req.CodeVerifier,
		RedirectURI:  req.RedirectURI,
		Username:     username,
		Status:       StatusPending,
		CreatedAt:    now,
	}

	// Local callback server is optional. Remote / container deploys still work
	// with manual paste of the localhost redirect URL.
	_ = m.ensureServerLocked()

	return req.AuthorizationURL, req.State, nil
}

// Poll returns the current status for a pending login state.
// Successful polls consume the result (one-shot).
func (m *LoginManager) Poll(state string) *PendingLogin {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.pending[state]
	if !ok {
		return &PendingLogin{State: state, Status: StatusError, Error: "unknown or expired login state"}
	}
	if time.Since(p.CreatedAt) > loginTimeout && p.Status == StatusPending {
		p.Status = StatusError
		p.Error = "login timed out"
	}
	// Copy for caller
	out := *p
	if p.Tokens != nil {
		tok := *p.Tokens
		out.Tokens = &tok
	}
	if p.Status == StatusSuccess || p.Status == StatusError {
		delete(m.pending, state)
		m.maybeStopServerLocked()
	}
	return &out
}

// Peek returns status without consuming success/error (for intermediate pending checks).
func (m *LoginManager) Peek(state string) *PendingLogin {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pending[state]
	if !ok {
		return &PendingLogin{State: state, Status: StatusError, Error: "unknown or expired login state"}
	}
	if time.Since(p.CreatedAt) > loginTimeout && p.Status == StatusPending {
		p.Status = StatusError
		p.Error = "login timed out"
	}
	out := *p
	if p.Tokens != nil {
		tok := *p.Tokens
		out.Tokens = &tok
	}
	return &out
}

// Consume removes and returns a finished pending login.
func (m *LoginManager) Consume(state string) *PendingLogin {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pending[state]
	if !ok {
		return &PendingLogin{State: state, Status: StatusError, Error: "unknown or expired login state"}
	}
	out := *p
	if p.Tokens != nil {
		tok := *p.Tokens
		out.Tokens = &tok
	}
	if p.Status == StatusSuccess || p.Status == StatusError {
		delete(m.pending, state)
		m.maybeStopServerLocked()
	}
	return &out
}

func (m *LoginManager) ensureServerLocked() error {
	if m.server != nil {
		return nil
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", DefaultLoginPort))
	if err != nil {
		ln, err = net.Listen("tcp", fmt.Sprintf("[::1]:%d", DefaultLoginPort))
		if err != nil {
			return fmt.Errorf("cannot bind OAuth callback port %d (is another login in progress?): %w", DefaultLoginPort, err)
		}
	}
	m.listener = ln
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", m.handleCallback)
	mux.HandleFunc("/cancel", m.handleCancel)
	m.server = &http.Server{Handler: mux}
	go func() {
		_ = m.server.Serve(ln)
	}()
	return nil
}

func (m *LoginManager) maybeStopServerLocked() {
	if len(m.pending) > 0 || m.server == nil {
		return
	}
	srv := m.server
	m.server = nil
	m.listener = nil
	go func() {
		_ = srv.Close()
	}()
}

func (m *LoginManager) handleCancel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Cancelled"))
}

func (m *LoginManager) handleCallback(w http.ResponseWriter, r *http.Request) {
	if err := m.completeFromQuery(r.URL.Query()); err != nil {
		writeHTML(w, http.StatusBadRequest, "Login failed", err.Error()+". You can close this window and return to the app.")
		return
	}
	writeHTML(w, http.StatusOK, "Sign-in complete", "Your ChatGPT account is connected. You can close this window and return to the app.")
}

// CompleteFromCallbackURL finishes a pending login from a browser redirect URL
// (or a bare query string) the user copies when localhost:1455 is unreachable.
// Accepts forms like:
//   - http://localhost:1455/auth/callback?code=...&state=...
//   - /auth/callback?code=...&state=...
//   - code=...&state=...
func (m *LoginManager) CompleteFromCallbackURL(raw string) error {
	q, err := parseCallbackInput(raw)
	if err != nil {
		return err
	}
	return m.completeFromQuery(q)
}

func parseCallbackInput(raw string) (url.Values, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("callback URL is empty")
	}
	// Full URL
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid callback URL: %w", err)
		}
		return u.Query(), nil
	}
	// Path + query (e.g. /auth/callback?code=...&state=...)
	if strings.HasPrefix(raw, "/") {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid callback path: %w", err)
		}
		return u.Query(), nil
	}
	// Bare query string (with or without leading ?)
	q, err := url.ParseQuery(strings.TrimPrefix(raw, "?"))
	if err != nil {
		return nil, fmt.Errorf("invalid callback query: %w", err)
	}
	if q.Get("code") == "" && q.Get("state") == "" && q.Get("error") == "" {
		return nil, fmt.Errorf("callback URL must include code and state (got no OAuth params)")
	}
	return q, nil
}

// completeFromQuery exchanges the authorization code for tokens and updates pending state.
func (m *LoginManager) completeFromQuery(q url.Values) error {
	state := q.Get("state")
	code := q.Get("code")
	errParam := q.Get("error")

	if state == "" {
		return fmt.Errorf("missing state in callback")
	}

	m.mu.Lock()
	p, ok := m.pending[state]
	var codeVerifier, redirectURI string
	if ok {
		// Copy PKCE material under lock; exchange is slow and must not race on map entry.
		codeVerifier = p.CodeVerifier
		redirectURI = p.RedirectURI
		if p.Status != StatusPending {
			m.mu.Unlock()
			if p.Status == StatusSuccess {
				return nil // already completed (e.g. double-submit)
			}
			return fmt.Errorf("login already failed: %s", p.Error)
		}
	}
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("unknown or expired login session; start ChatGPT sign-in again")
	}

	if errParam != "" {
		m.fail(state, "OpenAI OAuth error: "+errParam)
		return fmt.Errorf("OpenAI OAuth error: %s", errParam)
	}
	if code == "" {
		m.fail(state, "missing authorization code")
		return fmt.Errorf("missing authorization code in callback")
	}

	tokens, err := ExchangeCode(code, codeVerifier, redirectURI, DefaultClientID)
	if err != nil {
		m.fail(state, err.Error())
		return fmt.Errorf("token exchange failed: %w", err)
	}

	m.mu.Lock()
	if cur, ok := m.pending[state]; ok && cur.Status == StatusPending {
		cur.Status = StatusSuccess
		cur.Tokens = tokens
	}
	m.mu.Unlock()
	return nil
}

func (m *LoginManager) fail(state, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.pending[state]; ok {
		p.Status = StatusError
		p.Error = msg
	}
}

func writeHTML(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Connection", "close")
	w.WriteHeader(status)
	// Auto-close when opened as a popup; also notify the opener so the app can close it.
	ok := "false"
	if status >= 200 && status < 300 && title == "Sign-in complete" {
		ok = "true"
	}
	safeTitle := html.EscapeString(title)
	safeMessage := html.EscapeString(message)
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>%s</title>
<style>
body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#0b0b0f;color:#f5f5f5}
.card{max-width:28rem;padding:2rem;border-radius:1rem;background:#16161d;border:1px solid #2a2a35;text-align:center}
h1{font-size:1.25rem;margin:0 0 .75rem}
p{color:#a1a1aa;margin:0;line-height:1.5}
</style></head>
<body><div class="card"><h1>%s</h1><p>%s</p></div>
<script>
(function () {
  var ok = %s;
  try {
    if (window.opener && !window.opener.closed) {
      // Opener may be any app origin; payload is non-secret (status only).
      window.opener.postMessage({ type: "chatgpt-oauth", ok: ok }, "*");
    }
  } catch (e) {}
  // Browsers only allow close() for script-opened windows; try a few times.
  setTimeout(function () { window.close(); }, 400);
  setTimeout(function () { window.close(); }, 1200);
})();
</script>
</body></html>`, safeTitle, safeTitle, safeMessage, ok)
}
