package chat

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	fs "github.com/Bajahaw/ai-ui/cmd/files"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/skills"
	"github.com/Bajahaw/ai-ui/cmd/tools"
	"github.com/Bajahaw/ai-ui/cmd/utils"
	"github.com/openai/openai-go/v3"
)

const platformInstructions = `
<platform_instructions>

- To utilize the platform features, make sure to format and beautify your responses. make them clear to read and understand.
- If the response is long, use paragraphs, or seperators --- to make it easier on the eyes.
- Tool calls must be one at a time! parellal calling is not supported yet!

- When search is used, you must site all your used sources inline and at the end of the response. An inline citation badge is interactive reference for the source and written in the format of ([Source name][number]).
- Every response that uses inline citation badges ([Text][N]) MUST end with a matching numbered reference block listing the full URL and a short description. Example:
>This is a paragraph with some facts from the internet which should include references for the sources using inline citation badges, one at a time after the end of this paragraph, exactly after the full stop. ([Source name][number])([Another source][number++]) 
>rest of the response till the end ... 
>
>
>
>
>[number]: https://source.link/article "Discription or snippet"
>[number++]: https://source.link/another-article "Discription or snippet"

- To render latex, wrap math in $$ ... $$. Inside a paragraph it renders inline, on its own lines it renders as a block. e.g. as the following:
>The equation $$e=mc^2$$
>the explanation:
>$$
>latex
>$$

- To render rich widgets using HTML, CSS, and JS, use a code block tag with "widget" like this: (` + "```widget" + `).
- Widgets can be used for visuals, functional utilities, generating files (e.g. docx, and pdfs), and execute scripts including WASM (e.g. Python).

- To render Mermaid diagrams, use a code block with "mermaid" as the language tag.
- To render svg shapes and visuals, use the svg code block with "svg" language tag.

- To send the user a file, use marked down links [file name](file url). Internal files can be referenced like [name](/data/resources/{file_id.ext}). 
- To render images in chat, use the markdown image syntax ![](image url or path). Otherwise, it will be a downloadable link.

</platform_instructions>
`

func compileSystemPrompt(user string) string {
	systemPrompt, _ := settings.Get("systemPrompt", user)
	appendDateFlag, _ := settings.Get("appendDateToSystemPrompt", user)
	appendPlatformFlag, _ := settings.Get("appendPlatformInstructions", user)

	var sb strings.Builder

	if appendDateFlag == "true" {
		sb.WriteString("Current date: ")
		sb.WriteString(time.Now().Format("2006-01-02"))
		sb.WriteString("\n\n")
	}

	if appendPlatformFlag == "true" {
		sb.WriteString(strings.TrimSpace(platformInstructions))
		sb.WriteString("\n\n")
	}

	// Inject available skills so the model knows about them up front.
	// Built-ins are omitted when the user disabled enableBuiltinSkills;
	// a user skill with the same name always shadows the built-in.
	availableSkills := skills.GetAvailable(user)
	if len(availableSkills) > 0 {
		sb.WriteString("<available_skills>\n")
		sb.WriteString("Prefer user skills over system built-ins when both could apply.\n\n")
		for _, s := range availableSkills {
			desc := s.Description
			if desc == "" {
				desc = "(no description)"
			}
			sb.WriteString("- ")
			sb.WriteString(s.Name)
			sb.WriteString(": ")
			sb.WriteString(desc)
			sb.WriteString("\n")
		}
		sb.WriteString("\n</available_skills>\n\n")
	}

	sb.WriteString("<user_instructions>\n\n")
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n</user_instructions>")

	return sb.String()
}

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

	systemPrompt := compileSystemPrompt(user)

	attachmentOcrOnly, _ := settings.Get("attachmentOcrOnly", user)
	ocrOnly := attachmentOcrOnly == "true"
	agenticRetrievalStr, _ := settings.Get("agenticDocumentRetrieval", user)
	agenticRetrieval := agenticRetrievalStr == "true"

	var messages = []providers.SimpleMessage{
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

				toolMsg := providers.SimpleMessage{
					Role:     "tool",
					ToolCall: *tool, // FileID stays an id
				}
				attachToolFile(&toolMsg, tool.FileID, user)
				messages = append(messages, toolMsg)
			}
			continue
		}

		var imageURLs []string
		var fileURLs []string
		for _, att := range msg.Attachments {
			msg.Content += embeddedMetadata(att)

			if ocrOnly {
				msg.Content += embeddedContent(att.File.Content)
				continue
			}

			if fs.IsRetrievableDoc(att.File.Type) && agenticRetrieval {
				msg.Content += embeddedContent(att.File.Content)
				continue
			}

			msg.Content += "]\n"

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

// attachToolFile resolves a tool-produced file onto the provider message only.
// Metadata is not persisted on the tool output.
func attachToolFile(msg *providers.SimpleMessage, fileID, user string) {
	f, images, fileDataURLs := resolveToolFileMedia(fileID, user)
	msg.Images = images
	msg.Files = fileDataURLs
	if f.ID != "" {
		msg.Content = embeddedFileMetadata("tool", f) + "]\n"
	}
}

// resolveToolFileMedia loads a tool-produced file by id and returns data URLs
// for the provider request. ToolCall.FileID is never mutated.
func resolveToolFileMedia(fileID, user string) (f fs.File, images, fileDataURLs []string) {
	if fileID == "" {
		return fs.File{}, nil, nil
	}
	found, err := files.GetByIDs([]string{fileID}, user)
	if err != nil {
		log.Error("Error fetching tool call file", "err", err)
		return fs.File{}, nil, nil
	}
	if len(found) == 0 {
		return fs.File{}, nil, nil
	}
	data, err := os.ReadFile(found[0].Path)
	if err != nil {
		log.Error("Error reading tool call file", "err", err)
		return fs.File{}, nil, nil
	}
	mimeType := strings.Split(found[0].Type, ";")[0]
	mimeType = strings.ReplaceAll(mimeType, " ", "")
	dataURL := "data:" + mimeType + ";base64," + toBase64(data)
	if strings.HasPrefix(mimeType, "image/") {
		return found[0], []string{dataURL}, nil
	}
	return found[0], nil, []string{dataURL}
}

func enterAgentLoop(
	calls []providers.ToolCall,
	providerParams providers.RequestParams,
	responseMessage *Message,
	convID, user string,
	sc utils.StreamClient,
) (*providers.ChatCompletionMessage, error) {
	if providers.IsGenerationCancelled(responseMessage.ID) {
		return &providers.ChatCompletionMessage{Cancelled: true}, nil
	}

	for i, toolCall := range calls {
		if providers.IsGenerationCancelled(responseMessage.ID) {
			log.Debug("Generation cancelled before tool execution", "tool", toolCall.Name)
			return &providers.ChatCompletionMessage{Cancelled: true}, nil
		}

		assistantMsg := providers.SimpleMessage{
			Role:     "assistant",
			ToolCall: toolCall,
		}
		// Include content + reasoning on the first tool-call message so the
		// model can continue thinking across tool calls via reasoning_content.
		if i == 0 {
			assistantMsg.Content = responseMessage.Content
			assistantMsg.Reasoning = responseMessage.Reasoning
		}
		providerParams.Messages = append(providerParams.Messages, assistantMsg)

		toolCall.MessageID = responseMessage.ID
		toolCall.ConvID = convID

		result := tools.ExecuteMCPTool(providerParams.Context, toolCall, user, convID)
		// If cancel raced with tool start, drop the result and stop.
		if providers.IsGenerationCancelled(responseMessage.ID) {
			log.Debug("Generation cancelled during tool execution", "tool", toolCall.Name)
			return &providers.ChatCompletionMessage{Cancelled: true}, nil
		}

		toolCall.Output = result.Content
		toolCall.FileID = result.FileID

		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.TOOL_CALL,
			Payload: toolCall,
		})

		err := toolCalls.Save(&toolCall)
		if err != nil {
			log.Error("Error saving tool call output", "err", err)
		}

		// Append tool result; resolve file id to media on the message only.
		toolMsg := providers.SimpleMessage{
			Role: "tool",
			ToolCall: providers.ToolCall{
				ID:          toolCall.ID,
				ReferenceID: toolCall.ReferenceID,
				Name:        toolCall.Name,
				Output:      toolCall.Output,
				FileID:      toolCall.FileID,
				TokenCount:  toolCall.TokenCount,
				ContextSize: toolCall.ContextSize,
			},
		}
		attachToolFile(&toolMsg, toolCall.FileID, user)
		providerParams.Messages = append(providerParams.Messages, toolMsg)

	}

	if providers.IsGenerationCancelled(responseMessage.ID) {
		return &providers.ChatCompletionMessage{Cancelled: true}, nil
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
	// Keep partial content even when the follow-up stream was cancelled.
	if completion.Content != "" {
		if responseMessage.Content != "" {
			responseMessage.Content += "\n"
		}
		responseMessage.Content += completion.Content
	}

	// Accumulate reasoning for all tool calls
	if responseMessage.Reasoning != "" || completion.Reasoning != "" {
		for _, toolCall := range calls {
			responseMessage.Reasoning += "  \n`used tool:" + toolCall.Name + "`  \n"
		}
		responseMessage.Reasoning += completion.Reasoning
	}

	if completion.Cancelled || providers.IsGenerationCancelled(responseMessage.ID) {
		return completion, nil
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

func embeddedMetadata(att fs.Attachment) string {
	return "\n\n" + embeddedFileMetadata("user", att.File)
}

func embeddedFileMetadata(kind string, f fs.File) string {
	return "[" + kind + " attachment: \n" +
		"id: " + f.ID + "\n" +
		"name: " + f.Name + "\n" +
		"type: " + f.Type + "\n" +
		"size: " + strconv.FormatInt(f.Size, 10) + "\n" +
		"path: " + fs.ResourcePath(f) + "\n"
}

func embeddedContent(content string) string {
	return "content: " + content + "\n]\n"
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
