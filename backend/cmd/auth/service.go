package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// TokenTTL is how long a session remains valid without activity.
// Active use re-issues a fresh token (sliding session).
const TokenTTL = 72 * time.Hour

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

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     AUTH_COOKIE,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(TokenTTL),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     AUTH_COOKIE,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// issueAuthCookie mints a new JWT and sets the auth cookie.
func issueAuthCookie(w http.ResponseWriter, username string) error {
	token, err := generateJWT(username)
	if err != nil {
		return err
	}
	setAuthCookie(w, token)
	return nil
}

// maybeRefreshAuthCookie re-issues the session cookie when the current token
// is past the halfway point of its lifetime (sliding reactivation).
func maybeRefreshAuthCookie(w http.ResponseWriter, username string, claims map[string]any) {
	exp, ok := claims["exp"].(float64)
	if !ok {
		return
	}
	remaining := time.Until(time.Unix(int64(exp), 0))
	if remaining > refreshIfRemainingBelow {
		return
	}
	if err := issueAuthCookie(w, username); err != nil {
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
