package chatgptoauth

import (
	"fmt"
	"html"
	"net"
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

	if err := m.ensureServerLocked(); err != nil {
		delete(m.pending, req.State)
		return "", "", err
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
