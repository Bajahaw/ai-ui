package chatgptoauth_test

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/chatgptoauth"
)

func TestResolveRedirectURI_PublicBase(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "https://aichat.radhi.tech")
	t.Setenv("CHATGPT_OAUTH_REDIRECT_URI", "")
	r, _ := http.NewRequest("POST", "http://internal:8080/api/auth/chatgpt/start", nil)
	got := chatgptoauth.ResolveRedirectURI(r, "")
	want := "https://aichat.radhi.tech/api/auth/chatgpt/callback"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	req, err := chatgptoauth.CreateAuthRequest(got, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(req.AuthorizationURL, "1455") {
		t.Fatal("auth URL still contains 1455:", req.AuthorizationURL)
	}
	if !strings.Contains(req.AuthorizationURL, "aichat.radhi.tech") {
		t.Fatal("auth URL missing domain:", req.AuthorizationURL)
	}
}

func TestResolveRedirectURI_BrowserOrigin(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "")
	t.Setenv("CHATGPT_OAUTH_REDIRECT_URI", "")
	r, _ := http.NewRequest("POST", "http://127.0.0.1:8080/api/auth/chatgpt/start", nil)
	got := chatgptoauth.ResolveRedirectURI(r, "https://aichat.radhi.tech")
	if got != "https://aichat.radhi.tech/api/auth/chatgpt/callback" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveRedirectURI_HeaderOrigin(t *testing.T) {
	os.Unsetenv("PUBLIC_BASE_URL")
	os.Unsetenv("CHATGPT_OAUTH_REDIRECT_URI")
	r, _ := http.NewRequest("POST", "http://127.0.0.1:8080/api/auth/chatgpt/start", nil)
	r.Header.Set("Origin", "https://aichat.radhi.tech")
	got := chatgptoauth.ResolveRedirectURI(r, "")
	if got != "https://aichat.radhi.tech/api/auth/chatgpt/callback" {
		t.Fatalf("got %q", got)
	}
}
