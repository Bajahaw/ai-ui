package settings

type Settings struct {
	Settings map[string]string `json:"settings"`
}

func SetDefaults(user string) {
	defaults := map[string]string{
		"model": "gpt-4o",
		// "temperature":       "0.7",
		// "max_tokens":        "2048",
		// "top_p":             "1",
		// "frequency_penalty": "0",
		// "presence_penalty":  "0",
		"systemPrompt": "You are a helpful assistant. Provide clear accurate and helpful responses to the user questions.",
		// "responseType":      "stream",
		"reasoningEffort":   "disabled",
		"attachmentOcrOnly": "false",
		"ocrModel":          "deepseek-ocr",
	}

	if err := repo.SaveDefaults(defaults, user); err != nil {
		log.Error("Error setting default settings", "err", err)
	}
}
