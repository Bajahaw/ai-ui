package chatgptoauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultClientID  = "app_EMoamEEZ73f0CkXaXp7hrann"
	DefaultIssuer    = "https://auth.openai.com"
	DefaultTokenURL  = "https://auth.openai.com/oauth/token"
	DefaultScope     = "openid profile email offline_access"
	DefaultCodexURL  = "https://chatgpt.com/backend-api/codex"
	DefaultRedirect  = "http://localhost:1455/auth/callback"
	DefaultLoginPort = 1455
	ProviderType     = "chatgpt-oauth"
	ProviderBaseURL  = "chatgpt://oauth"
	// Stable provider id prefix; full id is chatgpt-<account_suffix>
	ProviderIDPrefix = "chatgpt"
)

// Tokens holds a ChatGPT/Codex OAuth session.
type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	AccountID    string `json:"account_id"`
	LastRefresh  string `json:"last_refresh,omitempty"`
	IsFedRamp    bool   `json:"is_fedramp,omitempty"`
}

// AuthRequest is a PKCE authorization request.
type AuthRequest struct {
	AuthorizationURL string
	State            string
	CodeVerifier     string
	RedirectURI      string
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func randomURLSafe(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func codeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// CreateAuthRequest builds a ChatGPT OAuth authorize URL (PKCE).
func CreateAuthRequest(redirectURI, clientID string) (*AuthRequest, error) {
	if redirectURI == "" {
		redirectURI = DefaultRedirect
	}
	if clientID == "" {
		clientID = DefaultClientID
	}
	state, err := randomURLSafe(24)
	if err != nil {
		return nil, err
	}
	verifier, err := randomURLSafe(48)
	if err != nil {
		return nil, err
	}
	challenge := codeChallengeS256(verifier)

	u, err := url.Parse(DefaultIssuer + "/oauth/authorize")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", DefaultScope)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	u.RawQuery = q.Encode()

	return &AuthRequest{
		AuthorizationURL: u.String(),
		State:            state,
		CodeVerifier:     verifier,
		RedirectURI:      redirectURI,
	}, nil
}

// ExchangeCode exchanges an authorization code for tokens.
func ExchangeCode(code, codeVerifier, redirectURI, clientID string) (*Tokens, error) {
	if clientID == "" {
		clientID = DefaultClientID
	}
	if redirectURI == "" {
		redirectURI = DefaultRedirect
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequest(http.MethodPost, DefaultTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("invalid token response: %w", err)
	}
	if resp.StatusCode >= 300 || tr.AccessToken == "" {
		msg := tr.ErrorDesc
		if msg == "" {
			msg = tr.Error
		}
		if msg == "" {
			msg = string(body)
		}
		return nil, fmt.Errorf("token exchange failed (HTTP %d): %s", resp.StatusCode, msg)
	}

	return tokensFromResponse(&tr)
}

// RefreshTokens refreshes an access token using a refresh token.
func RefreshTokens(refreshToken, clientID string) (*Tokens, error) {
	if clientID == "" {
		clientID = DefaultClientID
	}
	payload := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientID,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, DefaultTokenURL, strings.NewReader(string(raw)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("invalid refresh response: %w", err)
	}
	if resp.StatusCode >= 300 || tr.AccessToken == "" {
		msg := tr.ErrorDesc
		if msg == "" {
			msg = tr.Error
		}
		if msg == "" {
			msg = string(body)
		}
		return nil, fmt.Errorf("token refresh failed (HTTP %d): %s", resp.StatusCode, msg)
	}
	if tr.RefreshToken == "" {
		tr.RefreshToken = refreshToken
	}
	return tokensFromResponse(&tr)
}

func tokensFromResponse(tr *tokenResponse) (*Tokens, error) {
	accountID := DeriveAccountID(tr.IDToken)
	if accountID == "" {
		accountID = DeriveAccountID(tr.AccessToken)
	}
	if accountID == "" {
		return nil, fmt.Errorf("ChatGPT account id not found in token response")
	}
	return &Tokens{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		IDToken:      tr.IDToken,
		AccountID:    accountID,
		LastRefresh:  time.Now().UTC().Format(time.RFC3339),
		IsFedRamp:    DeriveIsFedRamp(tr.IDToken) || DeriveIsFedRamp(tr.AccessToken),
	}, nil
}

// ParseJWTClaims decodes the payload of a JWT without verification.
func ParseJWTClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// try padded
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims
}

// DeriveAccountID extracts chatgpt_account_id from a JWT.
func DeriveAccountID(token string) string {
	claims := ParseJWTClaims(token)
	if claims == nil {
		return ""
	}
	if auth, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
		if id, ok := auth["chatgpt_account_id"].(string); ok && id != "" {
			return id
		}
	}
	if id, ok := claims["chatgpt_account_id"].(string); ok && id != "" {
		return id
	}
	if orgs, ok := claims["organizations"].([]any); ok && len(orgs) > 0 {
		if org, ok := orgs[0].(map[string]any); ok {
			if id, ok := org["id"].(string); ok && id != "" {
				return id
			}
		}
	}
	return ""
}

// DeriveIsFedRamp checks FedRAMP claim on a token.
func DeriveIsFedRamp(token string) bool {
	claims := ParseJWTClaims(token)
	if claims == nil {
		return false
	}
	if auth, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
		if v, ok := auth["chatgpt_account_is_fedramp"].(bool); ok {
			return v
		}
	}
	return false
}

// DeriveEmail extracts email from an id token when present.
func DeriveEmail(idToken string) string {
	claims := ParseJWTClaims(idToken)
	if claims == nil {
		return ""
	}
	if email, ok := claims["email"].(string); ok {
		return email
	}
	return ""
}

// ShouldRefresh returns true if the access token should be refreshed.
func ShouldRefresh(t *Tokens) bool {
	if t == nil || t.AccessToken == "" {
		return true
	}
	if t.RefreshToken == "" {
		return false
	}
	claims := ParseJWTClaims(t.AccessToken)
	if claims != nil {
		if exp, ok := claims["exp"].(float64); ok {
			// refresh 5 minutes before expiry
			if time.Now().Unix() >= int64(exp)-300 {
				return true
			}
			return false
		}
	}
	if t.LastRefresh != "" {
		if ts, err := time.Parse(time.RFC3339, t.LastRefresh); err == nil {
			return time.Since(ts) > 55*time.Minute
		}
	}
	return true
}

// EnsureFresh refreshes tokens when needed and returns a usable session.
func EnsureFresh(t *Tokens) (*Tokens, bool, error) {
	if t == nil {
		return nil, false, fmt.Errorf("no tokens")
	}
	if !ShouldRefresh(t) {
		return t, false, nil
	}
	if t.RefreshToken == "" {
		return t, false, fmt.Errorf("access token expired and no refresh token")
	}
	refreshed, err := RefreshTokens(t.RefreshToken, DefaultClientID)
	if err != nil {
		return nil, false, err
	}
	// preserve account id if refresh omits it
	if refreshed.AccountID == "" {
		refreshed.AccountID = t.AccountID
	}
	return refreshed, true, nil
}

// UsernameFromAccount builds a stable local username for ChatGPT sign-in.
// Identity is derived only from accountID (hashed). Email must not be used for
// the username: a predictable email-based name can be pre-registered via the
// normal register endpoint and would then receive that ChatGPT user's session.
// The email parameter is accepted for call-site compatibility and ignored.
func UsernameFromAccount(accountID, _ string) string {
	if accountID == "" {
		return "cgpt_unknown"
	}
	sum := sha256.Sum256([]byte("cgpt-user\x00" + accountID))
	return fmt.Sprintf("cgpt_%x", sum[:8])
}

// ProviderIDForAccount returns a short, stable provider id for an account + user.
// Format: "cgpt-" + 6 hex chars (e.g. cgpt-a1b2c3). Username is hashed in so
// multi-user installs don't collide on the global Providers.id primary key,
// without bloating model ids shown in the UI (providerId/modelName).
func ProviderIDForAccount(accountID, username string) string {
	sum := sha256.Sum256([]byte(accountID + "\x00" + username))
	return fmt.Sprintf("cgpt-%x", sum[:3])
}
