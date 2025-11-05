package provider

import (
	"ai-client/cmd/tools"
	"ai-client/cmd/utils"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type SimpleMessage struct {
	Role     string
	Content  string
	ToolCall tools.ToolCall `json:"tool_call,omitzero"`
	Image    string
}

type ProviderRequestParams struct {
	Messages        []SimpleMessage
	Model           string
	ReasoningEffort openai.ReasoningEffort
}

type StreamChunk struct {
	Content   string         `json:"content,omitempty"`
	Reasoning string         `json:"reasoning,omitempty"`
	ToolCall  tools.ToolCall `json:"tool_call,omitzero"`
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

func (c *Client) SendChatCompletionRequest(params ProviderRequestParams) (*openai.ChatCompletion, error) {
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
		Model:    model,
		Messages: OpenAIMessageParams(params.Messages),
		Tools:    toOpenAITools(tools.GetAllTools()),
	}

	log.Debug("Params ReasoningEffort:", "value", params.ReasoningEffort)
	if params.ReasoningEffort != "" {
		openAIparams.ReasoningEffort = params.ReasoningEffort
	}

	//
	log.Debug("Sending chat completion request", "params", openAIparams)

	completion, err := client.Chat.Completions.New(ctx, openAIparams)
	if err != nil {
		return nil, err
	}
	return completion, nil
}

// SendChatCompletionStreamRequest streams chat completions and returns the full content
func (c *Client) SendChatCompletionStreamRequest(params ProviderRequestParams, w http.ResponseWriter) (*openai.ChatCompletionMessage, error) {
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
		// option.WithDebugLog(log.StandardLog()),
	)

	openAIparams := openai.ChatCompletionNewParams{
		Model:           model,
		Messages:        OpenAIMessageParams(params.Messages),
		ReasoningEffort: params.ReasoningEffort,
		Tools:           toOpenAITools(tools.GetAllTools()),
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering if behind proxy

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	stream := client.Chat.Completions.NewStreaming(ctx, openAIparams)
	acc := openai.ChatCompletionAccumulator{}
	// isDeepseekThinkStyle := -1
	// isDeepseekReasoningFinished := false

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) > 0 {
			// accContent := acc.Choices[0].Message.Content
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

			if toolCall, ok := acc.JustFinishedToolCall(); ok {
				toolCallData := StreamChunk{
					ToolCall: tools.ToolCall{
						ID:   toolCall.ID,
						Name: toolCall.Name,
						Args: toolCall.Arguments,
					},
				}

				toolCallJSON, _ := json.Marshal(toolCallData)
				fmt.Fprintf(w, "data: %s\n\n", toolCallJSON)
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

	log.Debug("Stop reason:", "reason", acc.Choices[0].FinishReason)

	return &acc.Choices[0].Message, nil
}
