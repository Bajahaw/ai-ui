package auth

import (
	"ai-client/cmd/utils"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"time"

	logger "github.com/charmbracelet/log"
)

type AuthStatus struct {
	Registered    bool `json:"registered"`
	Authenticated bool `json:"authenticated"`
}

var token string
var authCookie = "auth_token"
var log *logger.Logger

func Setup(l *logger.Logger) {
	log = l
	token = os.Getenv("APP_TOKEN")
	if token == "" {
		log.Error("APP_TOKEN is not set. Authentication is disabled.")
	}
}

func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /login", Login())
	mux.Handle("POST /logout", Authenticated(Logout()))
	mux.Handle("POST /register", Register())
	mux.Handle("GET /status", GetAuthStatus())

	return http.StripPrefix("/api/auth", mux)
}

func Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inputToken := r.FormValue("token")
		if inputToken != token {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		cookie := &http.Cookie{
			Name:     authCookie,
			Value:    token,
			Path:     "/",
			Expires:  time.Now().Add(30 * 24 * time.Hour),
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
			Name:     authCookie,
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
		cookie, err := r.Cookie(authCookie)
		if token != "" && (err != nil || cookie.Value != token) {
			log.Warn("Unauthorized access attempt", "path", r.URL.Path, "ip", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Register() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			http.Error(w, "Registration is disabled", http.StatusForbidden)
			return
		}

		token = rand.Text()

		utils.RespondWithJSON(w, map[string]string{"token": token}, http.StatusOK)
	})
}

func GetAuthStatus() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var status = AuthStatus{
			Registered:    true,
			Authenticated: false,
		}

		if token == "" {
			status.Registered = false
		}

		cookie, err := r.Cookie(authCookie)
		if err == nil && cookie.Value == token {
			status.Authenticated = true
		}

		var statusCode int

		switch {
		case status.Registered && status.Authenticated:
			statusCode = http.StatusOK
		case status.Registered && !status.Authenticated:
			statusCode = http.StatusUnauthorized
		default:
			statusCode = http.StatusForbidden
		}

		utils.RespondWithJSON(w, &status, statusCode)
	})
}
