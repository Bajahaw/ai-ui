package settings

type Settings struct {
	Settings map[string]string `json:"settings"`
}

func SetDefaults(user string) {
	defaults := map[string]string{
		"model":        "gpt-4o",
		"systemPrompt": "You are a helpful assistant. Provide clear accurate and helpful responses to the user questions.",
		// New toggles to control extra content appended to the system prompt
		"appendDateToSystemPrompt":   "false",
		"appendPlatformInstructions": "true",
		"reasoningEffort":            "disabled",
		"attachmentOcrOnly":          "false",
		"agenticDocumentRetrieval":   "false",
		"ocrModel":                   "deepseek-ocr",
		"imageModel":                 "dall-e-3",
	}

	if err := repo.SaveDefaults(defaults, user); err != nil {
		log.Error("Error setting default settings", "err", err)
	}
}
