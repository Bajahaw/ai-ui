package chat

import (
	"ai-client/cmd/data"
	"ai-client/cmd/utils"
	"net/http"
)

type Settings struct {
	Settings map[string]string `json:"settings"`
}

func getAllSettings(w http.ResponseWriter, _ *http.Request) {
	sql := "SELECT key, value FROM Settings"
	rows, err := data.DB.Query(sql)
	if err != nil {
		log.Error("Error querying settings", "err", err)
		http.Error(w, "Error querying settings", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	settings := make(map[string]string)
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

	err = saveUpdatedSettings(request)
	if err != nil {
		log.Error("Error updating settings", "err", err)
		http.Error(w, "Error updating settings", http.StatusInternalServerError)
		return
	}

	response := request

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func saveUpdatedSettings(s Settings) error {
	for key, value := range s.Settings {
		if key == "" {
			log.Error("empty key in settings")
			continue
		}

		// on conflict, update the value
		sql := "INSERT INTO Settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value"
		_, err := data.DB.Exec(sql, key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func setDefaultSettings() {
	defaults := map[string]string{
		"model":             "gpt-4o",
		"temperature":       "0.7",
		"max_tokens":        "2048",
		"top_p":             "1",
		"frequency_penalty": "0",
		"presence_penalty":  "0",
		"systemPrompt":      "You are a helpful assistant. Provide clear accurate and helpful responses to the user questions.",
		"responseType":      "stream",
		"reasoningEffort":   "disabled",
	}

	for key, value := range defaults {
		if key == "" {
			log.Error("empty key in default settings")
			continue
		}

		// on conflict, do not update the value
		sql := "INSERT INTO Settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=Settings.value"
		_, err := data.DB.Exec(sql, key, value)
		if err != nil {
			log.Error("Error setting default setting", "key", key, "err", err)
			continue
		}
	}
}

func getSetting(key string) (string, error) {
	sql := "SELECT value FROM Settings WHERE key = ?"
	row := data.DB.QueryRow(sql, key)

	var value string
	err := row.Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}
