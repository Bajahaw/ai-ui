package settings

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/utils"
	"net/http"
)

func SettingsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET 	/", getAllSettings)
	mux.HandleFunc("POST 	/update", updateSettings)

	return http.StripPrefix("/api/settings", auth.Authenticated(mux))
}

func getAllSettings(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	settings, err := repo.GetAll(user)
	if err != nil {
		log.Error("Error querying settings", "err", err)
		http.Error(w, "Error querying settings", http.StatusInternalServerError)
		return
	}

	response := Settings{settings}
	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func updateSettings(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	var request Settings
	err := utils.ExtractJSONBody(r, &request)
	if err != nil {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = repo.Save(request.Settings, user)
	if err != nil {
		log.Error("Error updating settings", "err", err)
		http.Error(w, "Error updating settings", http.StatusInternalServerError)
		return
	}

	response := request

	utils.RespondWithJSON(w, &response, http.StatusOK)
}
