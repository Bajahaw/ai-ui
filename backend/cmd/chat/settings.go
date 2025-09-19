package chat

import (
	"ai-client/cmd/data"
	"ai-client/cmd/utils"
	"net/http"
)

type Settings struct {
	Settings map[string]string `json:"settings"`
}

var settings = make(map[string]string)

func getAllSettings(w http.ResponseWriter, _ *http.Request) {
	sql := "SELECT key, value FROM Settings"
	rows, err := data.DB.Query(sql)
	if err != nil {
		log.Error("Error querying settings", "err", err)
		http.Error(w, "Error querying settings", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	settings = make(map[string]string)
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			log.Error("Error scanning setting", "err", err)
			http.Error(w, "Error scanning settings", http.StatusInternalServerError)
			return
		}
		settings[key] = value
	}

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

		sql := "INSERT INTO Settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value"
		_, err = data.DB.Exec(sql, key, value)
		if err != nil {
			log.Error("Error updating setting", "key", key, "value", value, "err", err)
			http.Error(w, "Error updating settings", http.StatusInternalServerError)
			return
		}
		settings[key] = value
	}

	response := Settings{settings}

	utils.RespondWithJSON(w, &response, http.StatusOK)
}
