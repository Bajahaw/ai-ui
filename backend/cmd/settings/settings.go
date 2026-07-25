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
		// Built-in skills shipped with the app (user can disable for their account)
		"enableBuiltinSkills":        "true",
		"reasoningEffort":            "disabled",
		"attachmentOcrOnly":          "false",
		"agenticDocumentRetrieval":   "false",
		"ocrModel":                   "deepseek-ocr",
		"imageModel":                 "dall-e-3",
		// TTS read-aloud (OpenAI-compatible /audio/speech)
		"ttsModel":                   "",
		"ttsVoice":                   "alloy",
		"ttsSpeed":                   "1",
	}

	if err := repo.SaveDefaults(defaults, user); err != nil {
		log.Error("Error setting default settings", "err", err)
	}
}


// MaybeSetDefaultModel sets the default chat model when unset or still the placeholder.
func MaybeSetDefaultModel(user, modelID string) {
	if modelID == "" || repo == nil {
		return
	}
	v, err := repo.Get("model", user)
	if err != nil || v == "" || v == "gpt-4o" {
		if err := repo.Save(map[string]string{"model": modelID}, user); err != nil {
			log.Error("Error setting default ChatGPT model", "err", err)
		}
	}
}
