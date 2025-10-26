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

func SendChatCompletionRequest(messages []SimpleMessage, model string) (*openai.ChatCompletion, error) {
	providerID, model := utils.ExtractProviderID(model)
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

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: OpenAIMessageParams(messages),

		//Tools: []openai.ChatCompletionToolUnionParam{
		//	{
		//		OfCustom: &openai.ChatCompletionCustomToolParam{
		//			Type: "browser_search",
		//		},
		//	},
		//},
	}

	//
	log.Debug("Sending chat completion request", "params", params)

	completion, err := client.Chat.Completions.New(ctx, params)
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

// SendChatCompletionStreamRequest streams chat completions and returns the full content
func SendChatCompletionStreamRequest(messages []SimpleMessage, model string, w http.ResponseWriter) (*openai.ChatCompletionMessage, error) {
	providerID, model := utils.ExtractProviderID(model)
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

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: OpenAIMessageParams(messages),
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

	stream := client.Chat.Completions.NewStreaming(ctx, params)
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
