package chat

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/tools"
	"github.com/Bajahaw/ai-ui/cmd/utils"
)

const platformInstructions = `
<platform_instructions>

- To utilize the platform features, make sure to format and beautify your responses. make them clear to read and understand.
- If the response is long, use paragraphs, or seperators --- to make it easier on the eyes.
- Tool calls must be one at a time! parellal calling is not supported yet!

- When search is used, site all your used sources inline and at the end of the response. An inline citation badge is interactive reference for the source and written in the format of ([Source name][number]). example:
>This is a paragraph with some facts from the internet which should include references for the sources using inline citation badges, one at a time after the end of this paragraph. ([Source name][number])([Another source][number++]) 
>rest of the response till the end ... 
>
>
>
>
>[number]: https://source.link/article "Discription or snippet"
>[number++]: https://source.link/another-article "Discription or snippet"

- In case the user is asking math question, for latex to render correctly, use $inline syntax$ or $$$ for blocks of latex. e.g. as the following:
>The equation $e=mc...$
>the explanation:
>$$$
>latex
>$$$

- Because the symbol ` + "`$`" + ` is reserved for latex, when it is used in chat, it must be escaped or wrapped. e.g.: 
>I bought this for only 20\$ or ` + "`20$`" + `, not 30\$!

- To render Mermaid charts and diagrams, just wrap using a code block with "mermaid" as the language.
- To render other complex diagrams or visuals, use the svg code block with "svg" tag (not xml or html).

</platform_instructions>
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
	finalSystemPrompt := "<user_instructions>\n\n" + systemPrompt + "\n\n</user_instructions>"
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

func enterAgentLoop(
	calls []tools.ToolCall,
	providerParams providers.RequestParams,
	responseMessage *Message,
	convID, user string,
	sc utils.StreamClient,
) (*providers.ChatCompletionMessage, error) {
	for _, toolCall := range calls {

		providerParams.Messages = append(providerParams.Messages, providers.SimpleMessage{
			Role:     "assistant",
			ToolCall: toolCall,
		})

		toolCall.MessageID = responseMessage.ID
		toolCall.ConvID = convID

		output := tools.ExecuteMCPTool(toolCall, user)
		toolCall.Output = output

		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.TOOL_CALL,
			Payload: toolCall,
		})

		err := toolCalls.Save(&toolCall)
		if err != nil {
			log.Error("Error saving tool call output", "err", err)
		}

		// Append tool result message to context for continued completion
		providerParams.Messages = append(providerParams.Messages, providers.SimpleMessage{
			Role: "tool",
			ToolCall: tools.ToolCall{
				ID:          toolCall.ID,
				ReferenceID: toolCall.ReferenceID,
				Name:        toolCall.Name,
				Output:      output,
				TokenCount:  toolCall.TokenCount,
				ContextSize: toolCall.ContextSize,
			},
		})

	}

	completion, err := provider.SendChatCompletionStreamRequest(providerParams, sc)
	if err != nil {
		log.Error("Error streaming chat completion after tool call", "err", err)
		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.EVENT_ERROR,
			Payload: err.Error(),
		})
		return completion, err
	}

	// Accumulate reasoning for all tool calls
	if responseMessage.Reasoning != "" || completion.Reasoning != "" {
		for _, toolCall := range calls {
			responseMessage.Reasoning += "  \n`using tool:" + toolCall.Name + "`  \n"
		}
		responseMessage.Reasoning += completion.Reasoning
	}

	calls = completion.ToolCalls
	if len(calls) > 0 {
		return enterAgentLoop(calls, providerParams, responseMessage, convID, user, sc)
	}

	return completion, err
}

func toBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
