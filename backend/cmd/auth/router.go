package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Bajahaw/ai-ui/cmd/utils"

	logger "github.com/charmbracelet/log"
)

type AuthStatus struct {
	Authenticated       bool   `json:"authenticated"`
	RegistrationEnabled bool   `json:"registration_enabled"`
	AuthMode            string `json:"auth_mode"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// PostRegisterHook defines the signature for actions after registration
type PostRegisterHook func(username string)

// OnRegister is a list of functions to run after successful registration
var OnRegister []PostRegisterHook
var log *logger.Logger
var db *sql.DB
var users UserRepository
var JWT_SECRET string

// allowRegistration gates POST /register and new ChatGPT OAuth user creation.
// Default true when unset so existing deploys keep open signup.
var allowRegistration = true

// Auth modes: "password" (default, multi-user server) or "profiles" (local
// single-device use: switchable passwordless profiles, no sign-in screen).
const (
	AuthModePassword = "password"
	AuthModeProfiles = "profiles"
)

var authMode = AuthModePassword

// machineSecretFile persists the JWT secret for profiles mode so sessions
// survive app restarts without any sign-in.
const machineSecretFile = "./data/.aiui_secret"

const AUTH_COOKIE = "auth_token"

func Setup(l *logger.Logger, d *sql.DB) {
	log = l
	db = d
	users = NewUserRepository(db)
	authMode = parseAuthMode(os.Getenv("AUTH_MODE"))
	JWT_SECRET = os.Getenv("JWT_SECRET")
	if JWT_SECRET == "" {
		if authMode == AuthModeProfiles {
			JWT_SECRET = loadOrCreateMachineSecret()
		} else {
			JWT_SECRET = rand.Text()
			log.Warn("JWT_SECRET not set in environment; using random secret for this session")
		}
	}
	allowRegistration = parseBoolEnv("ALLOW_REGISTRATION", true)
	if authMode == AuthModeProfiles {
		// Password sign-up makes no sense for passwordless profiles.
		allowRegistration = false
		log.Info("Profiles auth mode: passwordless local profiles, no sign-in required")
	} else if !allowRegistration {
		log.Info("Registration is disabled (ALLOW_REGISTRATION=false)")
	}
}

// AuthMode reports the active auth mode ("password" or "profiles").
func AuthMode() string {
	return authMode
}

func parseAuthMode(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), AuthModeProfiles) {
		return AuthModeProfiles
	}
	return AuthModePassword
}

// loadOrCreateMachineSecret returns a stable per-device secret, creating and
// persisting it on first run. File is 0600; failure falls back to random.
func loadOrCreateMachineSecret() string {
	if raw, err := os.ReadFile(machineSecretFile); err == nil {
		if s := strings.TrimSpace(string(raw)); len(s) >= 32 {
			return s
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		log.Warn("Failed to generate machine secret; using random session secret")
		return rand.Text()
	}
	secret := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(machineSecretFile), 0o755); err == nil {
		if err := os.WriteFile(machineSecretFile, []byte(secret+"\n"), 0o600); err != nil {
			log.Warn("Failed to persist machine secret; sessions end with this run", "err", err)
		}
	}
	return secret
}

// RegistrationAllowed reports whether new account creation is permitted.
func RegistrationAllowed() bool {
	return allowRegistration
}

func parseBoolEnv(key string, defaultVal bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultVal
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultVal
	}
}

func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /login", Login())
	mux.Handle("POST /logout", Authenticated(Logout()))
	mux.Handle("POST /register", Register())
	mux.Handle("GET /status", GetAuthStatus())
	mux.Handle("POST /change-pass", Authenticated(http.HandlerFunc(UpdateUser)))
	mux.HandleFunc("POST /chatgpt/start", startChatGPTLogin)
	mux.HandleFunc("POST /chatgpt/callback", submitChatGPTCallback)
	mux.HandleFunc("GET /chatgpt/status", pollChatGPTLogin)
	mux.Handle("GET /profiles", Profiles())
	mux.Handle("POST /profiles/select", SelectProfile())
	mux.Handle("POST /profiles/create", CreateProfile())
	mux.Handle("DELETE /profiles/{username}", DeleteProfile())

	return http.StripPrefix("/api/auth", mux)
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {
	if authMode == AuthModeProfiles {
		http.Error(w, "Password changes are disabled in profiles mode", http.StatusForbidden)
		return
	}
	username := utils.ExtractContextUser(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		Password        string `json:"password"`
	}
	if err := utils.ExtractJSONBody(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" {
		http.Error(w, "Current password is required", http.StatusBadRequest)
		return
	}

	if err := verifyUserCredentials(username, req.CurrentPassword); err != nil {
		http.Error(w, "Invalid current password", http.StatusUnauthorized)
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		if err.Error() == "Invalid password length" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	user := &User{
		Username: username,
		passHash: string(hash),
	}

	if err := users.Update(user); err != nil {
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func Register() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authMode == AuthModeProfiles {
			http.Error(w, "Password registration is disabled in profiles mode", http.StatusForbidden)
			return
		}
		if !allowRegistration {
			http.Error(w, "Registration is disabled", http.StatusForbidden)
			return
		}

		var req RegisterRequest
		if err := utils.ExtractJSONBody(r, &req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if len(req.Username) == 0 || len(req.Password) < 8 {
			http.Error(w, "Bad Credentials", http.StatusBadRequest)
			return
		}

		err := registerNewUser(req.Username, req.Password)
		if err != nil {
			log.Error("Failed to register user", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Debug("calling hooks")
		for _, hook := range OnRegister {
			hook(req.Username)
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

func Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authMode == AuthModeProfiles {
			http.Error(w, "Password login is disabled in profiles mode", http.StatusForbidden)
			return
		}
		username := r.FormValue("username")
		password := r.FormValue("password")

		err := verifyUserCredentials(username, password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		if err := issueAuthCookie(w, r, username); err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "Login successful. Cookie set.")
	}
}

func Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clearAuthCookie(w, r)
		fmt.Fprintln(w, "Logged out.")
	}
}

func Authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if username, ok := usernameFromRequest(r); ok {
			// Sliding session: re-issue when the token is in the second half of its life.
			if claims, err := claimsFromRequest(r); err == nil {
				maybeRefreshAuthCookie(w, r, username, claims)
			}
			r = r.WithContext(context.WithValue(r.Context(), "user", username))
			next.ServeHTTP(w, r)
			return
		}
		log.Warn("Unauthorized access attempt", "path", r.URL.Path, "ip", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// usernameFromRequest resolves the session user from the auth cookie, or
// from an ?access_token= bearer JWT (used by <img>/<video> tags and other
// resource loads that cannot send cookies, e.g. inside the mobile WebView).
func usernameFromRequest(r *http.Request) (string, bool) {
	if cookie, err := r.Cookie(AUTH_COOKIE); err == nil {
		if claims, err := extractClaims(cookie.Value); err == nil {
			if username, ok := claimUsername(claims); ok {
				return username, true
			}
		}
	}
	if token := r.URL.Query().Get("access_token"); token != "" {
		if claims, err := extractClaims(token); err == nil {
			if username, ok := claimUsername(claims); ok {
				return username, true
			}
		}
	}
	return "", false
}

func claimsFromRequest(r *http.Request) (map[string]any, error) {
	if cookie, err := r.Cookie(AUTH_COOKIE); err == nil {
		return extractClaims(cookie.Value)
	}
	return nil, fmt.Errorf("no session cookie")
}

func GetAuthStatus() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := AuthStatus{
			Authenticated:       false,
			RegistrationEnabled: allowRegistration,
			AuthMode:            authMode,
		}

		username, ok := usernameFromRequest(r)
		if !ok {
			utils.RespondWithJSON(w, &status, http.StatusOK)
			return
		}

		status.Authenticated = true
		// App open / connection: extend session if the token is aging.
		if claims, err := claimsFromRequest(r); err == nil {
			maybeRefreshAuthCookie(w, r, username, claims)
		}
		utils.RespondWithJSON(w, &status, http.StatusOK)
	})
}
