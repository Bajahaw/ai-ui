package providers

import (
	"ai-client/cmd/tools"
	"ai-client/cmd/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type SimpleMessage struct {
	Role     string
	Content  string
	ToolCall tools.ToolCall
	Images   []string
}

type RequestParams struct {
	Messages        []SimpleMessage
	Model           string
	ReasoningEffort openai.ReasoningEffort
	User            string
}

type ChatCompletionMessage struct {
	Content   string
	Reasoning string
	ToolCalls []tools.ToolCall
	Stats     utils.StreamStats
}

func (c *ClientImpl) SendChatCompletionRequest(params RequestParams) (*ChatCompletionMessage, error) {
	providerID, model := utils.ExtractProviderID(params.Model)
	provider, err := providers.GetByID(providerID, params.User)
	if err != nil {
		log.Error("Error querying provider", "err", err)
		return nil, errors.New("Model or provider not found")
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
		Tools:    toOpenAITools(tools.GetAvailableTools(params.User)),
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

	var toolCalls []tools.ToolCall
	for _, tc := range completion.Choices[0].Message.ToolCalls {
		toolCalls = append(toolCalls, tools.ToolCall{
			ID:          uuid.NewString(),
			ReferenceID: tc.ID,
			Name:        tc.Function.Name,
			Args:        tc.Function.Arguments,
		})
	}

	return &ChatCompletionMessage{
		Content:   completion.Choices[0].Message.Content,
		Reasoning: completion.Choices[0].Message.Reasoning,
		ToolCalls: toolCalls,
	}, nil
}

// SendChatCompletionStreamRequest streams chat completions and returns the full content
func (c *ClientImpl) SendChatCompletionStreamRequest(params RequestParams, w http.ResponseWriter) (*ChatCompletionMessage, error) {
	providerID, model := utils.ExtractProviderID(params.Model)
	provider, err := providers.GetByID(providerID, params.User)
	if err != nil {
		return nil, errors.New("Provider not found")
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
		Tools:           toOpenAITools(tools.GetAvailableTools(params.User)),
	}

	utils.AddStreamHeaders(w)

	stream := client.Chat.Completions.NewStreaming(ctx, openAIparams)
	acc := openai.ChatCompletionAccumulator{}
	uniqueToolIDs := make(map[string]string)
	// isDeepseekThinkStyle := -1
	// isDeepseekReasoningFinished := false

	start := time.Now()

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) > 0 {
			// accContent := acc.Choices[0].Message.Content
			contentDelta := chunk.Choices[0].Delta.Content
			reasoningDelta := chunk.Choices[0].Delta.Reasoning

			// for compatibility with specific providers
			reasoningTxtDelta := chunk.Choices[0].Delta.ReasoningText
			if reasoningTxtDelta != "" && reasoningDelta == "" {
				reasoningDelta = reasoningTxtDelta
			}

			if reasoningDelta != "" {
				utils.SendStreamChunk(w, utils.StreamChunk{
					Payload: reasoningDelta,
					Type:    utils.REASONING,
				})
			}

			if contentDelta != "" {
				utils.SendStreamChunk(w, utils.StreamChunk{
					Payload: contentDelta,
					Type:    utils.CONTENT,
				})
			}

			if toolCall, ok := acc.JustFinishedToolCall(); ok {

				uniqueToolIDs[toolCall.ID] = uuid.New().String()

				utils.SendStreamChunk(w, utils.StreamChunk{
					Type: utils.TOOL_CALL,
					Payload: tools.ToolCall{
						ID: uniqueToolIDs[toolCall.ID],
						// ReferenceID: toolCall.ID,
						Name: toolCall.Name,
						Args: toolCall.Arguments,
					},
				})
			}

		}
	}

	duration := time.Since(start)

	if err := stream.Err(); err != nil {
		var apiErr *openai.Error
		if errors.As(err, &apiErr) {
			type Error struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			}
			type ErrorMessage struct {
				Error Error `json:"error"`
			}

			var errMsg ErrorMessage
			err = json.Unmarshal([]byte(apiErr.Message), &errMsg)
			if err != nil {
				errMsg = ErrorMessage{
					Error: Error{Message: apiErr.Message, Code: apiErr.Code},
				}
			}

			if errMsg.Error.Code != "" {
				errMsg.Error.Message = "- " + errMsg.Error.Message
			}

			err = fmt.Errorf("%d %s %s",
				apiErr.StatusCode,
				http.StatusText(apiErr.StatusCode),
				errMsg.Error.Message,
			)
		}

		return nil, err
	}

	if !(len(acc.Choices) > 0) {
		log.Debug("Stream completed with no choices")
		return nil, fmt.Errorf("no choices in completion")
	}

	log.Debug("Stop reason:", "reason", acc.Choices[0].FinishReason)

	// this mapping is needed because providers are not always
	// guaranteed to generate unique IDs for tool calls,
	// so we generate our own IDs here
	var toolCalls []tools.ToolCall
	for _, tc := range acc.Choices[0].Message.ToolCalls {
		id, ok := uniqueToolIDs[tc.ID]
		if !ok {
			id = uuid.New().String()
		}
		toolCalls = append(toolCalls, tools.ToolCall{
			ID:          id,
			ReferenceID: tc.ID,
			Name:        tc.Function.Name,
			Args:        tc.Function.Arguments,
		})
	}

	// for compatibility with specific providers
	reasoning := acc.Choices[0].Message.Reasoning
	if reasoning == "" && acc.Choices[0].Message.ReasoningText != "" {
		reasoning = acc.Choices[0].Message.ReasoningText
	}

	log.Debug("response completed", "content", acc.Choices[0].Message.Content)
	log.Debug("Usage stats:", "tokens", acc.Usage.TotalTokens, "prompt", acc.Usage.PromptTokens, "completion", acc.Usage.CompletionTokens)

	return &ChatCompletionMessage{
		Content:   acc.Choices[0].Message.Content,
		Reasoning: reasoning,
		ToolCalls: toolCalls,
		Stats: utils.StreamStats{
			PromptTokens:     int(acc.Usage.PromptTokens),
			CompletionTokens: int(acc.Usage.CompletionTokens),
			// TotalTokens:      int(acc.Usage.TotalTokens),
			Speed: math.Round(float64(acc.Usage.CompletionTokens)/duration.Seconds()*10) / 10,
		},
	}, nil
}
