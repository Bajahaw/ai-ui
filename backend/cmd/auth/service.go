package auth

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// TokenTTL is how long a session remains valid without activity.
// Active use re-issues a fresh token (sliding session).
const TokenTTL = 72 * time.Hour

// accessTokenTTL is the lifetime of bearer access tokens minted for media
// URLs and profile selection. Long-lived: profiles mode has no passwords, so
// re-auth friction is worse than the token exposure risk on a local device.
const accessTokenTTL = 365 * 24 * time.Hour

// refreshIfRemainingBelow re-issues the auth cookie when less than this much
// lifetime remains, so daily use extends the session without minting a JWT
// on every API request.
const refreshIfRemainingBelow = TokenTTL / 2

func generateJWT(username string) (string, error) {
	return generateJWTWithTTL(username, TokenTTL)
}

func generateJWTWithTTL(username string, ttl time.Duration) (string, error) {
	if JWT_SECRET == "" {
		return "", fmt.Errorf("JWT_SECRET environment variable not set")
	}

	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(ttl).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(JWT_SECRET))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func setAuthCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure, sameSite := cookieAttributes(r)
	http.SetCookie(w, &http.Cookie{
		Name:     AUTH_COOKIE,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(TokenTTL),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	secure, sameSite := cookieAttributes(r)
	http.SetCookie(w, &http.Cookie{
		Name:     AUTH_COOKIE,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

// cookieAttributes adapts the session cookie to the transport so auth keeps
// working outside HTTPS deployments:
//
//   - HTTPS or loopback (127.0.0.1/localhost, incl. the in-app API server):
//     Secure + SameSite=None. None is required because the mobile/desktop
//     WebView (wails.localhost) calls the loopback API cross-site; Secure is
//     still honoured there because loopback is a secure context.
//   - Plain HTTP (LAN IP / hostname): Secure=false + SameSite=Lax, otherwise
//     browsers reject the cookie and sign-in is silently broken.
func cookieAttributes(r *http.Request) (secure bool, sameSite http.SameSite) {
	if requestIsSecure(r) {
		return true, http.SameSiteNoneMode
	}
	return false, http.SameSiteLaxMode
}

func requestIsSecure(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if isLoopbackHost(r.Host) {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto == "https"
	}
	return false
}

func isLoopbackHost(host string) bool {
	h := host
	if i := strings.LastIndex(h, ":"); i >= 0 && !strings.Contains(h[i+1:], "]") {
		// Strip :port (careful with bracketed IPv6 like [::1]:8080).
		if strings.HasPrefix(h, "[") {
			if end := strings.Index(h, "]"); end >= 0 {
				h = h[1:end]
			}
		} else if strings.Count(h, ":") == 1 {
			h = h[:i]
		}
	} else {
		h = strings.Trim(h, "[]")
	}
	return h == "127.0.0.1" || h == "::1" || strings.EqualFold(h, "localhost")
}

// issueAuthCookie mints a new JWT and sets the auth cookie.
func issueAuthCookie(w http.ResponseWriter, r *http.Request, username string) error {
	token, err := generateJWT(username)
	if err != nil {
		return err
	}
	setAuthCookie(w, r, token)
	return nil
}

// generateAccessToken mints a long-lived bearer token for media URLs and
// profile selection responses.
func generateAccessToken(username string) (string, error) {
	return generateJWTWithTTL(username, accessTokenTTL)
}

// maybeRefreshAuthCookie re-issues the session cookie when the current token
// is past the halfway point of its lifetime (sliding reactivation).
func maybeRefreshAuthCookie(w http.ResponseWriter, r *http.Request, username string, claims map[string]any) {
	exp, ok := claims["exp"].(float64)
	if !ok {
		return
	}
	remaining := time.Until(time.Unix(int64(exp), 0))
	if remaining > refreshIfRemainingBelow {
		return
	}
	if err := issueAuthCookie(w, r, username); err != nil {
		log.Warn("Failed to refresh auth cookie", "username", username, "error", err)
	}
}

func claimUsername(claims map[string]any) (string, bool) {
	username, ok := claims["username"].(string)
	return username, ok && username != ""
}

func extractClaims(token string) (map[string]any, error) {
	parsedToken, err := jwt.Parse(token, keyFunc)
	if err != nil {
		return nil, err
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok || !parsedToken.Valid {
		return nil, fmt.Errorf("Invalid token")
	}

	return claims, nil
}

// keyFunc provides the key for verifying the JWT signature
func keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
	}

	if JWT_SECRET == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable not set")
	}
	return []byte(JWT_SECRET), nil
}

func verifyUserCredentials(username, password string) error {
	user, err := users.GetByUsername(username)
	if err != nil {
		log.Debug("User not found", "username", username, "error", err)
		return fmt.Errorf("Invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.passHash), []byte(password))
	if err != nil {
		return fmt.Errorf("Invalid credentials")
	}

	return nil
}

func registerNewUser(username, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}

	user := &User{
		Username: username,
		passHash: string(hash),
	}

	err = users.Save(user)

	return err
}

func hashPassword(password string) ([]byte, error) {
	if len(password) < 8 || len(password) > 64 {
		return nil, fmt.Errorf("Invalid password length")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return hash, nil
}


// IssueSession mints an auth cookie for the given username (used by OAuth sign-in).
func IssueSession(w http.ResponseWriter, r *http.Request, username string) error {
	return issueAuthCookie(w, r, username)
}

// EnsureOAuthUser creates a local user for ChatGPT sign-in if missing.
// Returns (username, created, error).
func EnsureOAuthUser(username string) (string, bool, error) {
	if username == "" {
		return "", false, fmt.Errorf("empty username")
	}
	if _, err := users.GetByUsername(username); err == nil {
		return username, false, nil
	}
	// Random unusable password; account is OAuth-primary.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", false, err
	}
	hash, err := bcrypt.GenerateFromPassword(raw, bcrypt.DefaultCost)
	if err != nil {
		return "", false, err
	}
	err = users.Save(&User{Username: username, passHash: string(hash)})
	if err != nil {
		return "", false, err
	}
	return username, true, nil
}
