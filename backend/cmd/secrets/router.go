package secrets

import (
	"errors"
	"net/http"

	"github.com/Bajahaw/ai-ui/cmd/auth"
	"github.com/Bajahaw/ai-ui/cmd/utils"
)

func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /all", listSecrets)
	mux.HandleFunc("POST /save", saveSecret)
	mux.HandleFunc("DELETE /{id}", deleteSecret)

	return http.StripPrefix("/api/secrets", auth.Authenticated(mux))
}

func listSecrets(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	utils.RespondWithJSON(w, SecretListResponse{Secrets: ListMeta(user)}, http.StatusOK)
}

func saveSecret(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	var req SecretRequest
	if err := utils.ExtractJSONBody(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ID != "" {
		resp, err := Update(req.ID, user, req.Name, req.Value)
		if err != nil {
			writeSecretError(w, err)
			return
		}
		utils.RespondWithJSON(w, resp, http.StatusOK)
		return
	}

	resp, err := Create(user, req.Name, req.Value)
	if err != nil {
		writeSecretError(w, err)
		return
	}
	utils.RespondWithJSON(w, resp, http.StatusCreated)
}

func deleteSecret(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")
	if err := Delete(id, user); err != nil {
		writeSecretError(w, err)
		return
	}
	utils.RespondWithJSON(w, map[string]string{"status": "success"}, http.StatusOK)
}

func writeSecretError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidName):
		http.Error(w, "Invalid secret name: A-Z, 0-9, underscore only; start with a letter; no spaces", http.StatusBadRequest)
	case errors.Is(err, ErrInvalidValue):
		http.Error(w, "Invalid secret value: non-empty UTF-8, max 8KiB", http.StatusBadRequest)
	case errors.Is(err, ErrNotFound):
		http.Error(w, "Secret not found", http.StatusNotFound)
	case errors.Is(err, ErrLimit):
		http.Error(w, "Secret limit reached (100)", http.StatusBadRequest)
	default:
		if log != nil {
			log.Error("Secret operation failed", "err", err)
		}
		// Surface unique-name errors to the client
		msg := err.Error()
		if msg == "a secret with this name already exists" {
			http.Error(w, msg, http.StatusConflict)
			return
		}
		http.Error(w, "Error saving secret", http.StatusInternalServerError)
	}
}
