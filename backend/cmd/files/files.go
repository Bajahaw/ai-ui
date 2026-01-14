package files

import (
	"ai-client/cmd/providers"
	"os"
	"strings"
)

type File struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Size      int64  `json:"size"`
	Path      string `json:"path"`
	URL       string `json:"url"`
	Content   string `json:"content"`
	User      string `json:"user,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// extractFileContent extracts text content from the file at the given URL.
// It sends a request to the OCR service and returns the extracted text.
// currently supports images only. if file content is text, then it is not sent to OCR.
func extractFileContent(file File, model string) (string, error) {
	log.Debug("Extracting content from file", "path", file.Path, "type", file.Type)
	if strings.HasPrefix(file.Type, "text/") {
		fileContent, err := os.ReadFile(file.Path)
		if err != nil {
			log.Error("Error reading text file", "err", err)
			return "", err
		}
		return string(fileContent), nil
	}

	params := providers.RequestParams{
		Messages: []providers.SimpleMessage{
			{
				Role:    "system",
				Content: "You are an Image recognition and OCR assistant.",
			},
			{
				Role: "user",
				Content: "Extract text content from the given file. " +
					"preserve formatting of code, latex, tables etc. " +
					"as much as possible. If main content is not text, " +
					"provide a detailed description of the image instead.",
				Images: []string{
					file.URL,
				},
			},
		},
		Model: model,
		User:  file.User,
	}

	response, err := provider.SendChatCompletionRequest(params)
	if err != nil || len(response.Content) == 0 {
		return "", err
	}

	return response.Content, nil
}
