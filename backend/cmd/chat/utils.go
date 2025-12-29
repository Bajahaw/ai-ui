package chat

import (
	"ai-client/cmd/provider"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// Helper
func buildContext(convID string, start int) []provider.SimpleMessage {
	var convMessages = getAllConversationMessages(convID) // todo: cache or something
	var path []int
	var current = start
	log.Debug("Current message ID", "id", current)
	for {
		leaf, ok := convMessages[current]
		if !ok {
			break
		}
		path = append(path, current)
		current = leaf.ParentID
	}

	systemPrompt, _ := getSetting("systemPrompt")
	attachmentOcrOnly, _ := getSetting("attachmentOcrOnly")
	ocrOnly := attachmentOcrOnly == "true"

	var messages = []provider.SimpleMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	for i := len(path) - 1; i >= 0; i-- {
		msg, ok := convMessages[path[i]]
		if !ok {
			break
		}

		// If the assistant message has tool call, include it before the assistant content
		if msg.Role == "assistant" && len(msg.Tools) > 0 {
			for _, tool := range msg.Tools {

				messages = append(messages, provider.SimpleMessage{
					Role:     "assistant",
					ToolCall: tool,
				})

				messages = append(messages, provider.SimpleMessage{
					Role:     "tool",
					ToolCall: tool,
				})
			}
		}

		var imageURLs []string
		if ocrOnly {
			// For each attachment, append attachments content to message content
			for i, att := range msg.Attachments {
				if att.File.Content != "" {
					msg.Content += "\n\n" +
						"[user attachment " + fmt.Sprintf("%d", i+1) + ": \n" +
						"type: " + att.File.Type + "\n" +
						"content: " + att.File.Content + "\n\n]"
				}
			}

		} else {
			// append only image URLs
			for _, att := range msg.Attachments {
				if strings.HasPrefix(att.File.Type, "image/") && att.File.URL != "" {
					image, err := os.ReadFile(att.File.Path)
					if err != nil {
						log.Error("Error reading attachment file", "err", err)
						continue
					}
					b64url := "data:" + att.File.Type + ";base64," + toBase64(image)
					imageURLs = append(imageURLs, b64url)
				}
			}
		}

		messages = append(messages, provider.SimpleMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Images:  imageURLs,
		})
	}
	return messages
}

func toBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
