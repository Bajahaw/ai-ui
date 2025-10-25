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

// StreamChunk represents a chunk of streamed content
type StreamChunk struct {
	Content   string `json:"content,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}

// StreamMetadata represents metadata sent at the beginning of a stream
type StreamMetadata struct {
	ConversationID string `json:"conversationId"`
	UserMessageID  int    `json:"userMessageId"`
}

// StreamComplete represents the final data sent when stream is complete
type StreamComplete struct {
	UserMessageID      int `json:"userMessageId"`
	AssistantMessageID int `json:"assistantMessageId"`
}

// SendChatCompletionStreamRequest handles streaming chat completions and returns the full content
func SendChatCompletionStreamRequest(messages []SimpleMessage, model string, w http.ResponseWriter) (string, error) {
	providerID, model := utils.ExtractProviderID(model)
	provider, err := repo.getProvider(providerID)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client := openai.NewClient(
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
		option.WithDebugLog(log.StandardLog()),
	)

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: OpenAIMessageParams(messages),
	}

	log.Debug("Sending streaming chat completion request", "model", model)

	// Set headers for SSE (Server-Sent Events)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering if behind proxy

	flusher, ok := w.(http.Flusher)
	if !ok {
		return "", fmt.Errorf("streaming not supported")
	}

	stream := client.Chat.Completions.NewStreaming(ctx, params)
	acc := openai.ChatCompletionAccumulator{}

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		// Stream content and reasoning deltas to client
		if len(chunk.Choices) > 0 {
			contentDelta := chunk.Choices[0].Delta.Content
			reasoningDelta := chunk.Choices[0].Delta.Reasoning

			if contentDelta != "" || reasoningDelta != "" {
				// Send as SSE data event
				chunkData := StreamChunk{
					Content:   contentDelta,
					Reasoning: reasoningDelta,
				}
				chunkJSON, _ := json.Marshal(chunkData)
				fmt.Fprintf(w, "data: %s\n\n", chunkJSON)
				flusher.Flush()
			}
		}

		// Handle finished content
		if content, ok := acc.JustFinishedContent(); ok {
			log.Debug("Content stream finished", "length", len(content))
		}

		// Handle tool calls if needed in the future
		if tool, ok := acc.JustFinishedToolCall(); ok {
			log.Debug("Tool call stream finished", "index", tool.Index, "name", tool.Name)
		}

		// Handle refusals
		if refusal, ok := acc.JustFinishedRefusal(); ok {
			log.Debug("Refusal stream finished", "refusal", refusal)
		}
	}

	if err := stream.Err(); err != nil {
		return "", err
	}

	// Get the complete content from accumulator
	fullContent := ""
	if len(acc.Choices) > 0 {
		log.Debug("Stream completed", "message", acc.Choices[0])
		fullContent = acc.Choices[0].Message.Content
	}

	log.Debug("Stream completed", "total_length", len(fullContent))

	return fullContent, nil
}
