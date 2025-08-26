package chat

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/utils"
	"net/http"
)

type Settings struct {
	Settings map[string]string `json:"settings"`
}

var settings = func() map[string]string {
	return map[string]string{
		"systemPrompt": "You are a helpful assistant.",
	}
}()

func SettingsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", getAllSettings)
	mux.HandleFunc("POST /update", updateSettings)

	return http.StripPrefix("/api/settings", auth.Authenticated(mux))
}

func getAllSettings(w http.ResponseWriter, _ *http.Request) {
	response := Settings{settings}
	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func updateSettings(w http.ResponseWriter, r *http.Request) {
	var request Settings
	err := utils.ExtractJSONBody(r, &request)
	if err != nil {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	for key, value := range request.Settings {

		if key == "" {
			log.Error("Empty setting key", "key", key, "value", value)
			http.Error(w, "Invalid setting key", http.StatusBadRequest)
			return
		}

		settings[key] = value
	}

	response := Settings{settings}

	utils.RespondWithJSON(w, &response, http.StatusOK)
}
