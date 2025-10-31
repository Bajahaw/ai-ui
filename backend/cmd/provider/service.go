package provider

import (
	"ai-client/cmd/utils"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

var log = utils.GetLogger()

type SimpleMessage struct {
	Role    string
	Content string
	Image   string
}

type ProviderRequestParams struct {
	Messages        []SimpleMessage
	Model           string
	ReasoningEffort openai.ReasoningEffort
}

type StreamChunk struct {
	Content   string `json:"content,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}

type StreamMetadata struct {
	ConversationID string `json:"conversationId"`
	UserMessageID  int    `json:"userMessageId"`
}

// StreamComplete sent when stream is complete
type StreamComplete struct {
	UserMessageID      int `json:"userMessageId"`
	AssistantMessageID int `json:"assistantMessageId"`
}

func SendChatCompletionRequest(params ProviderRequestParams) (*openai.ChatCompletion, error) {
	providerID, model := utils.ExtractProviderID(params.Model)
	provider, err := repo.getProvider(providerID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client := openai.NewClient(
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	)

	openAIparams := openai.ChatCompletionNewParams{
		Model:           model,
		Messages:        OpenAIMessageParams(params.Messages),
		ReasoningEffort: params.ReasoningEffort,

		//Tools: []openai.ChatCompletionToolUnionParam{
		//	{
		//		OfCustom: &openai.ChatCompletionCustomToolParam{
		//			Type: "browser_search",
		//		},
		//	},
		//},
	}

	//
	log.Debug("Sending chat completion request", "params", openAIparams)

	completion, err := client.Chat.Completions.New(ctx, openAIparams)
	if err != nil {
		return nil, err
	}
	return completion, nil
}

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
		default:
			log.Warn("Unknown role %s in message, skipping", msg.Role)
			continue
		}
	}
	return openaiMessages
}

func ReasoningEffort(level string) openai.ReasoningEffort {
	switch level {
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

// SendChatCompletionStreamRequest streams chat completions and returns the full content
func SendChatCompletionStreamRequest(params ProviderRequestParams, w http.ResponseWriter) (*openai.ChatCompletionMessage, error) {
	providerID, model := utils.ExtractProviderID(params.Model)
	provider, err := repo.getProvider(providerID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client := openai.NewClient(
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	)

	openAIparams := openai.ChatCompletionNewParams{
		Model:           model,
		Messages:        OpenAIMessageParams(params.Messages),
		ReasoningEffort: params.ReasoningEffort,
	}

	// // Set headers for SSE (Server-Sent Events)
	// w.Header().Set("Content-Type", "text/event-stream")
	// w.Header().Set("Cache-Control", "no-cache")
	// w.Header().Set("Connection", "keep-alive")
	// w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering if behind proxy

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	stream := client.Chat.Completions.NewStreaming(ctx, openAIparams)
	acc := openai.ChatCompletionAccumulator{}

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) > 0 {
			contentDelta := chunk.Choices[0].Delta.Content
			reasoningDelta := chunk.Choices[0].Delta.Reasoning

			if contentDelta != "" || reasoningDelta != "" {

				chunkData := StreamChunk{
					Content:   contentDelta,
					Reasoning: reasoningDelta,
				}

				chunkJSON, _ := json.Marshal(chunkData)
				fmt.Fprintf(w, "data: %s\n\n", chunkJSON)
				flusher.Flush()
			}
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	if !(len(acc.Choices) > 0) {
		log.Debug("Stream completed with no choices")
		return nil, fmt.Errorf("no choices in completion")
	}

	return &acc.Choices[0].Message, nil
}
