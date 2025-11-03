package provider

import (
	"github.com/openai/openai-go/v3"
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
			if msg.Image != "" {
				file := openai.ChatCompletionContentPartUnionParam{
					OfImageURL: &openai.ChatCompletionContentPartImageParam{
						ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
							URL: msg.Image,
						},
					},
				}
				openaiMessages[i].OfUser.Content.OfArrayOfContentParts =
					append(openaiMessages[i].OfUser.Content.OfArrayOfContentParts, file)
			}
		case "assistant":
			openaiMessages[i] = openai.AssistantMessage(msg.Content)
			if msg.ToolCall.ID != "" {
				openaiMessages[i].OfAssistant.ToolCalls = append(openaiMessages[i].OfAssistant.ToolCalls,
					openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      msg.ToolCall.Name,
								Arguments: msg.ToolCall.Args,
							},
						},
					},
				)
			}

		case "tool":
			openaiMessages[i] = openai.ToolMessage(msg.ToolCall.Output, msg.ToolCall.ID)

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
