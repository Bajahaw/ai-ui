package chat

import (
	"ai-client/cmd/data"
	"ai-client/cmd/utils"
	"fmt"
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
			return fmt.Errorf("empty key in settings")
		}

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
		"reasoningEffort":   "medium",
	}

	err := saveUpdatedSettings(Settings{defaults})
	if err != nil {
		log.Error("Error setting default settings", "err", err)
	}
}

func getSystemPrompt() string {
	sql := "SELECT value FROM Settings WHERE key = ?"
	row := data.DB.QueryRow(sql, "systemPrompt")

	var systemPrompt string
	err := row.Scan(&systemPrompt)
	if err != nil {
		log.Error("Error retrieving system prompt", "err", err)
		setDefaultSettings()
		return "You are a helpful assistant. Provide clear accurate and helpful responses to the user questions."
	}
	return systemPrompt
}
