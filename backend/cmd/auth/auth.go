package auth

import (
	"ai-client/cmd/utils"
	"net/http"
)

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// PostRegisterHook defines the signature for actions after registration
type PostRegisterHook func(username string)

// OnRegister is a list of functions to run after successful registration
var OnRegister []PostRegisterHook

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
			http.Error(w, "Failed to register user", http.StatusInternalServerError)
			return
		}

		log.Debug("calling hooks")
		for _, hook := range OnRegister {
			hook(req.Username)
		}

		// utils.RespondWithJSON(w, map[string]string{"token": token}, http.StatusOK)
		w.WriteHeader(http.StatusNoContent)
	})
}
