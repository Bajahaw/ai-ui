package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/chatgptoauth"
	logger "github.com/charmbracelet/log"
)

func TestChatGPTLoginPollingRequiresBrowserBinding(t *testing.T) {
	setupTest()
	manager := chatgptoauth.NewLoginManager()
	oldManager := chatgptoauth.DefaultLoginManager
	chatgptoauth.DefaultLoginManager = manager
	var state string
	t.Cleanup(func() {
		chatgptoauth.DefaultLoginManager = oldManager
		_ = manager.CompleteFromCallbackURL("http://localhost:1455/auth/callback?error=cancelled&state=" + state)
		manager.Consume(state)
	})

	start := httptest.NewRecorder()
	startChatGPTLogin(start, httptest.NewRequest(http.MethodPost, "/api/auth/chatgpt/start", nil))
	if start.Code != http.StatusOK {
		t.Fatalf("start: %d %s", start.Code, start.Body.String())
	}
	var started chatgptStartResponse
	if err := json.Unmarshal(start.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	state = started.State
	var binding *http.Cookie
	for _, cookie := range start.Result().Cookies() {
		if cookie.Name == chatGPTStateCookie {
			binding = cookie
		}
	}
	if binding == nil || binding.Value != state || !binding.HttpOnly || !binding.Secure || binding.SameSite != http.SameSiteStrictMode {
		t.Fatalf("invalid browser binding cookie: %+v", binding)
	}

	poll := func(cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/chatgpt/status?state="+state, nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		pollChatGPTLogin(w, req)
		return w
	}
	if w := poll(nil); w.Code != http.StatusForbidden {
		t.Fatalf("missing binding: got %d, want 403", w.Code)
	}
	if w := poll(&http.Cookie{Name: chatGPTStateCookie, Value: "wrong"}); w.Code != http.StatusForbidden {
		t.Fatalf("wrong binding: got %d, want 403", w.Code)
	}
	if w := poll(binding); w.Code != http.StatusOK {
		t.Fatalf("valid binding: got %d %s", w.Code, w.Body.String())
	} else {
		var status chatgptStatusResponse
		_ = json.Unmarshal(w.Body.Bytes(), &status)
		if status.Status != "pending" {
			t.Fatalf("rejected polls consumed pending flow: %+v", status)
		}
	}

	if err := manager.CompleteFromCallbackURL("http://localhost:1455/auth/callback?error=access_denied&state=" + state); err == nil {
		t.Fatal("expected OAuth denial")
	}
	w := poll(binding)
	if w.Code != http.StatusOK {
		t.Fatalf("completed flow: got %d %s", w.Code, w.Body.String())
	}
	cleared := false
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == chatGPTStateCookie && cookie.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("terminal poll did not clear browser binding")
	}
}

func TestRegistrationDefaults(t *testing.T) {
	t.Cleanup(func() { setupTest() })
	t.Setenv("ALLOW_REGISTRATION", "")
	t.Setenv("ENV", "")
	Setup(logger.New(io.Discard), nil)
	if allowRegistration {
		t.Fatal("registration must be disabled by default outside development")
	}

	t.Setenv("ENV", "dev")
	Setup(logger.New(io.Discard), nil)
	if !allowRegistration {
		t.Fatal("development registration default changed")
	}

	t.Setenv("ENV", "")
	t.Setenv("ALLOW_REGISTRATION", "true")
	Setup(logger.New(io.Discard), nil)
	if !allowRegistration {
		t.Fatal("explicit ALLOW_REGISTRATION=true must override the secure default")
	}
}
