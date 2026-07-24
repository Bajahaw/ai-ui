package chatgptoauth

import (
	"fmt"
	"html"
	"net/http"
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

// LoginManager tracks pending OAuth logins. The OAuth redirect is handled by
// the main app HTTP server (see CallbackPath), not a localhost side-listener.
type LoginManager struct {
	mu      sync.Mutex
	pending map[string]*PendingLogin
}

// DefaultLoginManager is the process-wide manager.
var DefaultLoginManager = NewLoginManager()

func NewLoginManager() *LoginManager {
	return &LoginManager{pending: make(map[string]*PendingLogin)}
}

// Start begins a new OAuth login. redirectURI must match the public callback
// URL registered with OpenAI (typically {PUBLIC_BASE_URL}/api/auth/chatgpt/callback).
func (m *LoginManager) Start(username, redirectURI string) (authURL, state string, err error) {
	req, err := CreateAuthRequest(redirectURI, DefaultClientID)
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
	}
	return &out
}

// HandleCallback processes the OAuth redirect on the main app server.
// It exchanges the authorization code and marks the pending login complete.
func (m *LoginManager) HandleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	state := q.Get("state")
	code := q.Get("code")
	errParam := q.Get("error")

	m.mu.Lock()
	p, ok := m.pending[state]
	var codeVerifier, redirectURI string
	if ok {
		// Copy PKCE material under lock; exchange is slow and must not race on map entry.
		codeVerifier = p.CodeVerifier
		redirectURI = p.RedirectURI
	}
	m.mu.Unlock()

	if !ok {
		writeHTML(w, http.StatusBadRequest, "Login failed", "Unknown or expired login session. Return to the app and try again.")
		return
	}

	if errParam != "" {
		m.fail(state, "OpenAI OAuth error: "+errParam)
		writeHTML(w, http.StatusBadRequest, "Login failed", "You can close this window and return to the app.")
		return
	}
	if code == "" {
		m.fail(state, "missing authorization code")
		writeHTML(w, http.StatusBadRequest, "Login failed", "You can close this window and return to the app.")
		return
	}

	tokens, err := ExchangeCode(code, codeVerifier, redirectURI, DefaultClientID)
	if err != nil {
		m.fail(state, err.Error())
		writeHTML(w, http.StatusBadRequest, "Login failed", "Token exchange failed. You can close this window.")
		return
	}

	m.mu.Lock()
	if cur, ok := m.pending[state]; ok && cur.Status == StatusPending {
		cur.Status = StatusSuccess
		cur.Tokens = tokens
	}
	m.mu.Unlock()

	writeHTML(w, http.StatusOK, "Sign-in complete", "Your ChatGPT account is connected. You can close this window and return to the app.")
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
      // Opener is the same app origin; payload is non-secret (status only).
      window.opener.postMessage({ type: "chatgpt-oauth", ok: ok }, window.location.origin);
    }
  } catch (e) {}
  // Browsers only allow close() for script-opened windows; try a few times.
  setTimeout(function () { window.close(); }, 400);
  setTimeout(function () { window.close(); }, 1200);
})();
</script>
</body></html>`, safeTitle, safeTitle, safeMessage, ok)
}
