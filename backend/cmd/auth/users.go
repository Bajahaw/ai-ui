package auth

import (
	"ai-client/cmd/utils"
	"crypto/rand"
	"net/http"
)

// PostRegisterHook defines the signature for actions after registration
type PostRegisterHook func(username string)

// OnRegister is a list of functions to run after successful registration
var OnRegister []PostRegisterHook

func Register() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, err := getAdminToken()
		if err == nil && t != "" {
			http.Error(w, "Registration is disabled", http.StatusForbidden)
			return
		}

		token = rand.Text()

		err = registerAdminUser(token)
		if err != nil {
			log.Error("Failed to register admin user", "error", err)
			http.Error(w, "Failed to register admin user", http.StatusInternalServerError)
			return
		}

		log.Debug("calling hooks")
		for _, hook := range OnRegister {
			hook("admin")
		}

		utils.RespondWithJSON(w, map[string]string{"token": token}, http.StatusOK)
	})
}

func registerAdminUser(token string) error {
	_, err := db.Exec(
		`INSERT INTO users (username, token) VALUES (?, ?)`,
		"admin", token)
	return err
}
