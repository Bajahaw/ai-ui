package auth

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

var token = os.Getenv("APP_TOKEN")
var authCookie = "auth_token"

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

func Logout() http.Handler {
	handler := func(w http.ResponseWriter, _ *http.Request) {
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

	return Authenticated(http.HandlerFunc(handler))
}

func Authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(authCookie)
		if err != nil || cookie.Value != token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
