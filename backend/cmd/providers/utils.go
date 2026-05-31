package providers

import (
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
)

func OpenAIMessageParams(messages []SimpleMessage) []openai.ChatCompletionMessageParamUnion {
	openaiMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			openaiMessages = append(openaiMessages, openai.SystemMessage(msg.Content))
		case "user":
			userMsg := openai.ChatCompletionMessageParamUnion{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfArrayOfContentParts: []openai.ChatCompletionContentPartUnionParam{
							{
								OfText: &openai.ChatCompletionContentPartTextParam{
									Text: msg.Content,
								},
							},
						},
					},
				},
			}
			for _, imageURL := range msg.Images {
				userMsg.OfUser.Content.OfArrayOfContentParts =
					append(userMsg.OfUser.Content.OfArrayOfContentParts,
						openai.ChatCompletionContentPartUnionParam{
							OfImageURL: &openai.ChatCompletionContentPartImageParam{
								ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
									URL: imageURL,
								},
							},
						})
			}
			for _, fileData := range msg.Files {
				userMsg.OfUser.Content.OfArrayOfContentParts =
					append(userMsg.OfUser.Content.OfArrayOfContentParts,
						openai.ChatCompletionContentPartUnionParam{
							OfFile: &openai.ChatCompletionContentPartFileParam{
								File: openai.ChatCompletionContentPartFileFileParam{
									FileData: param.Opt[string]{Value: fileData},
								},
							},
						})
			}
			openaiMessages = append(openaiMessages, userMsg)

		case "assistant":
			assistantMsg := openai.AssistantMessage(msg.Content)
			if msg.ToolCall.ID != "" {
				assistantMsg.OfAssistant.ToolCalls = append(assistantMsg.OfAssistant.ToolCalls,
					openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							// ID: msg.ToolCall.ID, // changed to ReferenceID (the one from provider)
							ID: msg.ToolCall.ReferenceID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      msg.ToolCall.Name,
								Arguments: msg.ToolCall.Args,
							},
						},
					},
				)
			}
			openaiMessages = append(openaiMessages, assistantMsg)

		case "tool":
			// Always emit tool result as role:"tool" — all providers expect this after a tool_call
			toolMsg := openai.ChatCompletionMessageParamUnion{
				OfTool: &openai.ChatCompletionToolMessageParam{
					// ToolCallID: msg.ToolCall.ID, // changed to ReferenceID (the one from provider)
					ToolCallID: msg.ToolCall.ReferenceID,
					Content: openai.ChatCompletionToolMessageParamContentUnion{
						OfString: param.Opt[string]{Value: msg.ToolCall.Output},
					},
				},
			}
			// for compatibility with gemini tool messages
			toolMsg.OfTool.SetExtraFields(
				map[string]any{"name": msg.ToolCall.Name},
			)
			openaiMessages = append(openaiMessages, toolMsg)

			// If the tool produced an attachment, append a follow-up user message
			// with the file/image content. This keeps role:"tool" intact while
			// still letting the model see the attachment.
			if msg.ToolCall.File != "" {
				attachmentMsg := openai.ChatCompletionMessageParamUnion{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfArrayOfContentParts: []openai.ChatCompletionContentPartUnionParam{
								{
									OfText: &openai.ChatCompletionContentPartTextParam{
										Text: "Here is the result from tool '" + msg.ToolCall.Name + "':",
									},
								},
								{
									OfImageURL: &openai.ChatCompletionContentPartImageParam{
										ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
											URL: msg.ToolCall.File,
										},
									},
								},
							},
						},
					},
				}
				openaiMessages = append(openaiMessages, attachmentMsg)
			}

		default:
			log.Warn("Unknown role %s in message, skipping", msg.Role)
			continue
		}
	}
	return openaiMessages
}

func ReasoningEffort(level string) openai.ReasoningEffort {
	switch level {
	case "disabled":
		return ""
	case "minimal":
		return openai.ReasoningEffortMinimal
	case "low":
		return openai.ReasoningEffortLow
	case "medium":
		return openai.ReasoningEffortMedium
	case "high":
		return openai.ReasoningEffortHigh
	default:
		return openai.ReasoningEffortMedium
	}
}
