package chat

type Settings struct {
	Settings map[string]string `json:"settings"`
}

var settings = func() map[string]string {
	return map[string]string{
		"systemPrompt": "You are a helpful assistant.",
	}
}()
