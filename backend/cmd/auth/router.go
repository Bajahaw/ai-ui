package auth

import (
	"ai-client/cmd/utils"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	logger "github.com/charmbracelet/log"
)

type AuthStatus struct {
	Authenticated bool `json:"authenticated"`
}

var authCookie = "auth_token"
var log *logger.Logger
var db *sql.DB

func Setup(l *logger.Logger, d *sql.DB) {
	log = l
	db = d

	// _, err := verifyUserCredentials("admin")
	// if err == nil {
	// 	// token = t
	// 	return
	// }

	// // If not found, check environment variable
	// token := os.Getenv("APP_TOKEN")
	// if token != "" {
	// 	err := registerNewUser("admin", token)
	// 	if err != nil {
	// 		log.Error("Failed to register admin", "error", err)
	// 	} else {
	// 		log.Info("Admin user registered")
	// 	}
	// }

	// // If still not found, /api/auth/register can be used to create one
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
		username := r.FormValue("username")
		password := r.FormValue("password")

		err := verifyUserCredentials(username, password)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		cookie := &http.Cookie{
			Name:     authCookie,
			Value:    password,
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
		if err != nil {
			log.Warn("Unauthorized access attempt", "path", r.URL.Path, "ip", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		username, err := extractUsername(cookie.Value)
		if err != nil || username == "" {
			log.Warn("Invalid auth token", "path", r.URL.Path, "ip", r.RemoteAddr)
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

		cookie, err := r.Cookie(authCookie)
		if err != nil {
			status.Authenticated = false
			utils.RespondWithJSON(w, &status, http.StatusOK)
			return
		}

		username, err := extractUsername(cookie.Value)
		if err == nil && username != "" {
			status.Authenticated = true
		}

		utils.RespondWithJSON(w, &status, http.StatusOK)
	})
}

// extractUsername retrieves the username associated with the given token.
// Until a proper jwt implementation is in place, this function just queries the database.
func extractUsername(token string) (string, error) {
	var username string
	err := db.QueryRow(`SELECT username FROM users WHERE token = ?`, token).Scan(&username)
	if err != nil {
		return "", err
	}

	return username, nil
}

func verifyUserCredentials(username, password string) error {
	var storedPass string
	err := db.QueryRow(`SELECT token FROM users WHERE username = ?`, username).Scan(&storedPass)
	if err != nil {
		return err
	}

	if storedPass != password {
		return fmt.Errorf("invalid credentials")
	}

	return nil
}

func GetUsername(r *http.Request) string {
	ctx := r.Context()
	return ctx.Value("user").(string)
}
