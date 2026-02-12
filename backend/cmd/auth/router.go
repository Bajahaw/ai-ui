package auth

import (
	"ai-client/cmd/utils"
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	logger "github.com/charmbracelet/log"
)

type AuthStatus struct {
	Authenticated bool `json:"authenticated"`
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

const AUTH_COOKIE = "auth_token"

func Setup(l *logger.Logger, d *sql.DB) {
	log = l
	db = d
	users = NewUserRepository(db)
	JWT_SECRET = os.Getenv("JWT_SECRET")
	if JWT_SECRET == "" {
		JWT_SECRET = rand.Text()
		log.Warn("JWT_SECRET not set in environment; using random secret for this session")
	}
}

func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /login", Login())
	mux.Handle("POST /logout", Authenticated(Logout()))
	mux.Handle("POST /register", Register())
	mux.Handle("GET /status", GetAuthStatus())
	mux.Handle("POST /change-pass", Authenticated(http.HandlerFunc(UpdateUser)))

	return http.StripPrefix("/api/auth", mux)
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {
	username := utils.ExtractContextUser(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := utils.ExtractJSONBody(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	user := &User{
		Username: username,
		passHash: string(hash),
	}

	log.Debug("changed password", "new pass", user.passHash)

	if err := users.Update(user); err != nil {
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func Register() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		username := r.FormValue("username")
		password := r.FormValue("password")

		err := verifyUserCredentials(username, password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		signedToken, err := generateJWT(username)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		cookie := &http.Cookie{
			Name:     AUTH_COOKIE,
			Value:    signedToken,
			Path:     "/",
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		}
		http.SetCookie(w, cookie)
		fmt.Fprintln(w, "Login successful. Cookie set.")
	}
}

func Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		cookie := &http.Cookie{
			Name:     AUTH_COOKIE,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		}
		http.SetCookie(w, cookie)
		fmt.Fprintln(w, "Logged out.")
	}
}

func Authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(AUTH_COOKIE)
		if err != nil {
			log.Warn("Unauthorized access attempt", "path", r.URL.Path, "ip", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := extractClaims(cookie.Value)
		if err != nil {
			log.Warn("Invalid auth token", "path", r.URL.Path, "ip", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		username := claims["username"].(string)
		exp := claims["exp"].(float64)
		if time.Now().After(time.Unix(int64(exp), 0)) {
			log.Warn("Auth token expired", "path", r.URL.Path, "ip", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), "user", username))

		next.ServeHTTP(w, r)
	})
}

func GetAuthStatus() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var status = AuthStatus{
			Authenticated: false,
		}

		cookie, err := r.Cookie(AUTH_COOKIE)
		if err != nil {
			status.Authenticated = false
			utils.RespondWithJSON(w, &status, http.StatusOK)
			return
		}

		claims, err := extractClaims(cookie.Value)
		if err == nil && claims["username"] != "" {
			status.Authenticated = true
		}

		utils.RespondWithJSON(w, &status, http.StatusOK)
	})
}
