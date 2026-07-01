package chat

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"time"

	fs "github.com/Bajahaw/ai-ui/cmd/files"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/tools"
	"github.com/Bajahaw/ai-ui/cmd/utils"
	"github.com/openai/openai-go/v3"
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
>I bought this for only 20\$ or ` + "`20$`" + `!

- To render rich widgets using HTML, CSS, and JS, use a code block tag with "widget".
- Widgets can be used for visuals, functional utilities, generating files (e.g. docx, and pdfs), and execute scripts including WASM (e.g. Python).
- Widgets should be full width, and use the already passed CSS variables to match the chat interface colors. (--background, --foreground, --muted, --muted-foreground --border).
- The previous vars dont work in canvas e.g. Chart.js, instead you should use the __theme JS object (e.g., __theme['foreground'], __theme.isDark).

- To render Mermaid diagrams, use a code block with "mermaid" as the language.
- To render svg shapes and visuals, use the svg code block with "svg" tag.

- To send the user a file, use marked down links [file name](file url). Internal files can be referenced like [name](/data/resources/{file_id.ext}). 
- To render images in chat, use the markdown image syntax ![](image url or path). Otherwise, it will be a downloadable link.

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
	agenticRetrievalStr, _ := settings.Get("agenticDocumentRetrieval", user)
	agenticRetrieval := agenticRetrievalStr == "true"

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

		// If the assistant message has tool calls, include the text content on the
		// first tool-call message so the model sees what it said before using tools.
		// Then append each tool result. Skip the normal content append below.
		if msg.Role == "assistant" && len(msg.Tools) > 0 {
			for j, tool := range msg.Tools {
				assistantMsg := providers.SimpleMessage{
					Role:     "assistant",
					ToolCall: *tool,
				}
				// Attach the assistant's text to the first tool-call message
				if j == 0 {
					assistantMsg.Content = msg.Content
				}
				messages = append(messages, assistantMsg)

				// TODO: remove this temp hack
				// swap to base64 instead of file id
				tool.File = convertToolCallFileIDToBase64(tool.File, user)

				messages = append(messages, providers.SimpleMessage{
					Role:     "tool",
					ToolCall: *tool,
				})
			}
			continue
		}

		var imageURLs []string
		var fileURLs []string
		if ocrOnly {
			// embed all content if ocrOnly (vision assistant) required
			for _, att := range msg.Attachments {
				msg.Content += embeddedAttachment(att)
			}

		} else {

			// embed docs and encode the rest if agentic retrieval is on, otherwise just provide links to files
			for _, att := range msg.Attachments {

				if fs.IsRetrievableDoc(att.File.Type) && agenticRetrieval {
					// content of first page, model will figure to use tools to get rest of content if needed
					msg.Content += embeddedAttachment(att)
					continue
				}

				file, err := os.ReadFile(att.File.Path)
				if err != nil {
					log.Error("Error reading attachment file", "err", err)
					continue
				}

				// Strip any parameters from the mime type (e.g., ;charset=utf-8)
				mimeType := strings.Split(att.File.Type, ";")[0]
				b64url := "data:" + strings.ReplaceAll(mimeType, " ", "") + ";base64," + toBase64(file)
				log.Debug("Converted attachment to base64", "b64url", b64url[:50]+"...")
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

	log.Debug("Built context messages for conversation", "convID", convID, "messages", messages)
	return messages
}

func convertToolCallFileIDToBase64(f, user string) string {
	if f != "" {
		file, err := files.GetByIDs([]string{f}, user)
		if err != nil {
			log.Error("Error fetching tool call file", "err", err)
		}

		if len(file) > 0 {
			data, err := os.ReadFile(file[0].Path)
			if err != nil {
				log.Error("Error reading tool call file", "err", err)
			} else {
				mimeType := strings.Split(file[0].Type, ";")[0]
				f = "data:" + strings.ReplaceAll(mimeType, " ", "") + ";base64," + toBase64(data)
			}
		}
	}

	return f

}

func enterAgentLoop(
	calls []providers.ToolCall,
	providerParams providers.RequestParams,
	responseMessage *Message,
	convID, user string,
	sc utils.StreamClient,
) (*providers.ChatCompletionMessage, error) {
	for i, toolCall := range calls {

		assistantMsg := providers.SimpleMessage{
			Role:     "assistant",
			ToolCall: toolCall,
		}
		// Include the model's accumulated text in the first tool-call message
		// so the model sees what it already said before invoking tools.
		if i == 0 {
			assistantMsg.Content = responseMessage.Content
		}
		providerParams.Messages = append(providerParams.Messages, assistantMsg)

		toolCall.MessageID = responseMessage.ID
		toolCall.ConvID = convID

		result := tools.ExecuteMCPTool(toolCall, user, convID)
		toolCall.Output = result.Content
		toolCall.File = result.File

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
			ToolCall: providers.ToolCall{
				ID:          toolCall.ID,
				ReferenceID: toolCall.ReferenceID,
				Name:        toolCall.Name,
				Output:      toolCall.Output,
				File:        convertToolCallFileIDToBase64(toolCall.File, user),
				TokenCount:  toolCall.TokenCount,
				ContextSize: toolCall.ContextSize,
			},
		})

	}

	// Stream a newline separator before the post-tool completion so
	// sentences from before and after the tool call don't run together.
	// This mirrors the "\n" that is added to responseMessage.Content below.
	if responseMessage.Content != "" {
		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.CONTENT,
			Payload: "\n",
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

	// Accumulate content from the post-tool completion into the response.
	// Add a newline separator to prevent sentences from running together.
	if completion.Content != "" {
		if responseMessage.Content != "" {
			responseMessage.Content += "\n"
		}
		responseMessage.Content += completion.Content
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

func embeddedAttachment(att fs.Attachment) string {
	return "\n\n" +
		"[user attachment: \n" +
		"id: " + att.File.ID + "\n" +
		"name: " + att.File.Name + "\n" +
		"type: " + att.File.Type + "\n" +
		"content: " + att.File.Content + "\n]\n"
}

func toOpenAITools(tool []*tools.Tool) []openai.ChatCompletionToolUnionParam {
	var result []openai.ChatCompletionToolUnionParam
	for _, t := range tool {
		var inputSchema map[string]any
		_ = json.Unmarshal([]byte(t.InputSchema), &inputSchema)
		result = append(result, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
			Parameters:  inputSchema,
		}))
	}

	return result
}
