package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/chatgptoauth"
	"github.com/Bajahaw/ai-ui/cmd/utils"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type SimpleMessage struct {
	Role      string
	Content   string
	Reasoning string
	ToolCall  ToolCall
	Images    []string
	Files     []string
}

type RequestParams struct {
	Messages        []SimpleMessage
	Model           string
	ReasoningEffort openai.ReasoningEffort
	User            string
	MessageID       int
	// Context scopes the full assistant generation (streams + tool calls).
	// Cancelled by CancelStream / EndGeneration.
	Context context.Context
	Tools   []openai.ChatCompletionToolUnionParam
}

type ChatCompletionMessage struct {
	Content   string
	Reasoning string
	ToolCalls []ToolCall
	Stats     utils.StreamStats
	// Cancelled is true when the generation was cancelled mid-stream.
	// Callers must not execute tool calls or continue the agent loop.
	Cancelled bool
}

type ToolCall struct {
	ID          string `json:"id"`
	ReferenceID string `json:"ref_id"`
	ConvID      string `json:"conv_id,omitempty"`
	MessageID   int    `json:"message_id"`
	Name        string `json:"name"`
	Args        string `json:"args,omitempty"`
	Output      string `json:"tool_output,omitempty"`
	File        string `json:"files,omitempty"`
	TokenCount  int    `json:"tokenCount,omitempty"`
	ContextSize int    `json:"contextSize,omitempty"`
}

type ToolOutput struct {
	Content string `json:"content"`
	File    string `json:"file_ids,omitempty"`
}

func (c *ClientImpl) SendChatCompletionRequest(params RequestParams) (*ChatCompletionMessage, error) {
	providerID, model := utils.ExtractProviderID(params.Model)
	provider, err := providers.GetByID(providerID, params.User)
	if err != nil {
		log.Error("Error querying provider", "err", err)
		return nil, errors.New("Model or provider not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if provider.Type == chatgptoauth.ProviderType {
		return sendChatGPTCompletion(ctx, provider, model, params, nil)
	}

	opts := []option.RequestOption{
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	}
	for key, value := range provider.Headers {
		opts = append(opts, option.WithHeader(key, value))
	}
	client := openai.NewClient(opts...)

	openAIparams := openai.ChatCompletionNewParams{
		Model: model,
		Tools: params.Tools,
	}
	OpenAIMessageParams(&openAIparams, params.Messages)

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

	var toolCalls []ToolCall
	for _, tc := range completion.Choices[0].Message.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:          uuid.NewString(),
			ReferenceID: tc.ID,
			Name:        tc.Function.Name,
			Args:        tc.Function.Arguments,
		})
	}

	// Compatibility across providers: some use reasoning_text or reasoning_content.
	reasoning := completion.Choices[0].Message.Reasoning
	if reasoning == "" && completion.Choices[0].Message.ReasoningText != "" {
		reasoning = completion.Choices[0].Message.ReasoningText
	}
	if reasoning == "" && completion.Choices[0].Message.ReasoningContent != "" {
		reasoning = completion.Choices[0].Message.ReasoningContent
	}

	return &ChatCompletionMessage{
		Content:   completion.Choices[0].Message.Content,
		Reasoning: reasoning,
		ToolCalls: toolCalls,
	}, nil
}

// SendChatCompletionStreamRequest streams chat completions and returns the full content
func (c *ClientImpl) SendChatCompletionStreamRequest(params RequestParams, sc utils.StreamClient) (*ChatCompletionMessage, error) {
	providerID, model := utils.ExtractProviderID(params.Model)
	provider, err := providers.GetByID(providerID, params.User)
	if err != nil {
		return nil, errors.New("Provider not found")
	}

	parent := params.Context
	if parent == nil {
		parent = context.Background()
	}
	// Bound each provider stream; parent cancellation (user stop) still wins.
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()

	if provider.Type == chatgptoauth.ProviderType {
		sc := sc
		return sendChatGPTCompletion(ctx, provider, model, params, &sc)
	}

	opts := []option.RequestOption{
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
		// option.WithDebugLog(log.StandardLog()),
	}
	for key, value := range provider.Headers {
		opts = append(opts, option.WithHeader(key, value))
	}
	client := openai.NewClient(opts...)

	openAIparams := openai.ChatCompletionNewParams{
		Model:           model,
		ReasoningEffort: params.ReasoningEffort,
		Tools:           params.Tools,
		// Prefer real usage from providers that support the final usage chunk.
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		},
	}
	OpenAIMessageParams(&openAIparams, params.Messages)

	utils.AddStreamHeaders(sc.Writer)

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

			// Compatibility across providers (e.g. GitHub Copilot, Fireworks).
			reasoningTxtDelta := chunk.Choices[0].Delta.ReasoningText
			if reasoningTxtDelta != "" && reasoningDelta == "" {
				reasoningDelta = reasoningTxtDelta
			}

			reasoningContentDelta := chunk.Choices[0].Delta.ReasoningContent
			if reasoningContentDelta != "" && reasoningDelta == "" {
				reasoningDelta = reasoningContentDelta
			}

			if reasoningDelta != "" {
				utils.SendStreamChunk(sc, utils.StreamChunk{
					Payload: reasoningDelta,
					Type:    utils.REASONING,
				})
			}

			if contentDelta != "" {
				utils.SendStreamChunk(sc, utils.StreamChunk{
					Payload: contentDelta,
					Type:    utils.CONTENT,
				})
			}

			if toolCall, ok := acc.JustFinishedToolCall(); ok {

				uniqueToolIDs[toolCall.ID] = uuid.New().String()

				utils.SendStreamChunk(sc, utils.StreamChunk{
					Type: utils.TOOL_CALL,
					Payload: ToolCall{
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
	cancelled := isContextCanceled(ctx.Err()) || isContextCanceled(stream.Err())

	if err := stream.Err(); err != nil {
		log.Info("Stream detailed error", "err", err)
		if isContextCanceled(err) {
			log.Debug("Stream cancelled by user")
			// Ignore context cancelled error and return partial response
		} else {
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
	}

	if !(len(acc.Choices) > 0) {
		log.Debug("Stream completed with no choices")
		// If cancelled by user, return empty content instead of error
		if cancelled {
			return &ChatCompletionMessage{
				Content:   "",
				Reasoning: "",
				ToolCalls: []ToolCall{},
				Stats:     utils.StreamStats{},
				Cancelled: true,
			}, nil
		}
		return nil, fmt.Errorf("no choices in completion")
	}

	log.Debug("Stop reason:", "reason", acc.Choices[0].FinishReason)

	// Compatibility across providers: some use reasoning_text or reasoning_content.
	reasoning := acc.Choices[0].Message.Reasoning
	if reasoning == "" && acc.Choices[0].Message.ReasoningText != "" {
		reasoning = acc.Choices[0].Message.ReasoningText
	}
	if reasoning == "" && acc.Choices[0].Message.ReasoningContent != "" {
		reasoning = acc.Choices[0].Message.ReasoningContent
	}

	content := acc.Choices[0].Message.Content

	// On cancel, never hand partial/incomplete tool calls to the agent loop.
	// JustFinishedToolCall may never have fired for truncated argument streams.
	var toolCalls []ToolCall
	if !cancelled {
		// this mapping is needed because providers are not always
		// guaranteed to generate unique IDs for tool calls,
		// so we generate our own IDs here
		for _, tc := range acc.Choices[0].Message.ToolCalls {
			id, ok := uniqueToolIDs[tc.ID]
			if !ok {
				id = uuid.New().String()
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:          id,
				ReferenceID: tc.ID,
				Name:        tc.Function.Name,
				Args:        tc.Function.Arguments,
			})
		}
	} else {
		log.Debug("Generation cancelled; dropping tool calls from partial stream",
			"count", len(acc.Choices[0].Message.ToolCalls))
	}

	log.Debug("response completed", "content", content, "cancelled", cancelled)
	log.Debug("Usage stats:", "tokens", acc.Usage.TotalTokens, "prompt", acc.Usage.PromptTokens, "completion", acc.Usage.CompletionTokens)

	promptTokens := int(acc.Usage.PromptTokens)
	completionTokens := int(acc.Usage.CompletionTokens)

	// Fallback when the provider omits usage fields (common on some
	// OpenAI-compatible APIs and interrupted streams).
	if completionTokens <= 0 {
		var b strings.Builder
		b.WriteString(content)
		b.WriteString(reasoning)
		for _, tc := range toolCalls {
			b.WriteString(tc.Name)
			b.WriteString(tc.Args)
		}
		completionTokens = utils.EstimateTokens(b.String())
		log.Debug("API usage missing completion tokens; using estimate", "tokens", completionTokens)
	}
	if promptTokens <= 0 {
		promptTokens = utils.EstimateTokens(estimatePromptText(params.Messages))
		log.Debug("API usage missing prompt tokens; using estimate", "tokens", promptTokens)
	}

	seconds := duration.Seconds()
	if seconds == 0 {
		seconds = 1
	}
	speed := float64(completionTokens) / seconds
	log.Debug("Response speed:", "tokens_per_second", speed)

	stats := utils.StreamStats{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		// TotalTokens:      int(acc.Usage.TotalTokens),
		Speed: math.Round(speed*10) / 10,
	}

	if len(toolCalls) > 0 {
		// append tool call stats to the first tool call because
		// we only need stats per completion, not per tool call
		toolCalls[0].TokenCount = stats.CompletionTokens
		toolCalls[0].ContextSize = stats.PromptTokens
	}

	return &ChatCompletionMessage{
		Content:   content,
		Reasoning: reasoning,
		ToolCalls: toolCalls,
		Stats:     stats,
		Cancelled: cancelled,
	}, nil
}

func isContextCanceled(err error) bool {
	return err != nil && errors.Is(err, context.Canceled)
}

// estimatePromptText concatenates request message text used for fallback
// prompt-token estimation when the API omits usage fields.
func estimatePromptText(messages []SimpleMessage) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString(m.Content)
		b.WriteString(m.Reasoning)
		if m.ToolCall.Name != "" || m.ToolCall.Args != "" || m.ToolCall.Output != "" {
			b.WriteString(m.ToolCall.Name)
			b.WriteString(m.ToolCall.Args)
			b.WriteString(m.ToolCall.Output)
		}
	}
	return b.String()
}
