package providers

import (
	"ai-client/cmd/tools"
	"encoding/json"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
)

func OpenAIMessageParams(messages []SimpleMessage) []openai.ChatCompletionMessageParamUnion {
	openaiMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	for i, msg := range messages {
		switch msg.Role {
		case "system":
			openaiMessages[i] = openai.SystemMessage(msg.Content)
		case "user":
			openaiMessages[i] = openai.ChatCompletionMessageParamUnion{
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
			if len(msg.Images) > 0 {
				for _, imageURL := range msg.Images {
					file := openai.ChatCompletionContentPartUnionParam{
						OfImageURL: &openai.ChatCompletionContentPartImageParam{
							ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
								URL: imageURL,
							},
						},
					}
					openaiMessages[i].OfUser.Content.OfArrayOfContentParts =
						append(openaiMessages[i].OfUser.Content.OfArrayOfContentParts, file)
				}
			}
		case "assistant":
			openaiMessages[i] = openai.AssistantMessage(msg.Content)
			if msg.ToolCall.ID != "" {
				openaiMessages[i].OfAssistant.ToolCalls = append(openaiMessages[i].OfAssistant.ToolCalls,
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

		case "tool":
			openaiMessages[i] = openai.ChatCompletionMessageParamUnion{
				OfTool: &openai.ChatCompletionToolMessageParam{
					// ToolCallID: msg.ToolCall.ID, // changed to ReferenceID (the one from provider)
					ToolCallID: msg.ToolCall.ReferenceID,
					Content: openai.ChatCompletionToolMessageParamContentUnion{
						OfString: param.Opt[string]{Value: msg.ToolCall.Output},
					},
				},
			}
			// for compatibility with gemini tool messages
			openaiMessages[i].OfTool.SetExtraFields(
				map[string]any{"name": msg.ToolCall.Name},
			)

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
