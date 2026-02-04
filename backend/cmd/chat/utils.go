package chat

import (
	"ai-client/cmd/providers"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

const platformInstructions = `
<platform>

- to utilize the platform features, make sure to format and beautify your responses. make them clear to read and understand.
- tool calls must be one at a time! parellal calling is not supported yet!
- when search is used, site all your used sources inline and at the end of the response. IMPORTANT to render inline citation correctly, they should be as the following format:
>some facts from the enternet. ([Source name][number])([Another source][number]) // single source at a time
>rest of the response till the end ... 
>
>
>[number]: https://source.link/article "Discription or snippet"

- for latex to render correctly, use $inline syntax$ or $$$ for blocks of latex. e.g. as the following:
>The equation $e=mc...$
>the explanation:
>$$$
>latex
>$$$

- because ` + "`$`" + ` is reserved for latex, when the symbol is used as the currency it must be escaped! 
>I bought this for only 20\$ 

<platform>
`

// Helper
func buildContext(convID string, start int, user string) []providers.SimpleMessage {
	var convMessages = getAllConversationMessages(convID, user) // todo: cache or something
	var path []int
	var current = start
	// log.Debug("Current message ID", "id", current)
	for {
		leaf, ok := convMessages[current]
		if !ok {
			break
		}
		path = append(path, current)
		current = leaf.ParentID
	}

	systemPrompt, _ := settings.Get("systemPrompt", user)
	appendDateFlag, _ := settings.Get("appendDateToSystemPrompt", user)
	appendPlatformFlag, _ := settings.Get("appendPlatformInstructions", user)

	// Append date and/or platform instructions based on user settings
	finalSystemPrompt := systemPrompt
	if appendDateFlag == "true" {
		finalSystemPrompt = "Current date: " + time.Now().Format("2006-01-02") + "\n\n" + finalSystemPrompt
	}
	if appendPlatformFlag == "true" {
		finalSystemPrompt += "\n\n" + platformInstructions
	}
	attachmentOcrOnly, _ := settings.Get("attachmentOcrOnly", user)
	ocrOnly := attachmentOcrOnly == "true"

	var messages = []providers.SimpleMessage{
		{
			Role:    "system",
			Content: finalSystemPrompt,
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

				messages = append(messages, providers.SimpleMessage{
					Role:     "assistant",
					ToolCall: *tool,
				})

				messages = append(messages, providers.SimpleMessage{
					Role:     "tool",
					ToolCall: *tool,
				})
			}
		}

		var imageURLs []string
		var fileURLs []string
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
				file, err := os.ReadFile(att.File.Path)
				if err != nil {
					log.Error("Error reading attachment file", "err", err)
					continue
				}
				b64url := "data:" + att.File.Type + ";base64," + toBase64(file)

				if strings.HasPrefix(att.File.Type, "image/") {
					imageURLs = append(imageURLs, b64url)
				} else {
					fileURLs = append(fileURLs, b64url)
				}
			}
		}

		messages = append(messages, providers.SimpleMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Images:  imageURLs,
			Files:   fileURLs,
		})
	}
	return messages
}

func toBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
