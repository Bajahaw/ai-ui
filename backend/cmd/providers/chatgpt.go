package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/chatgptoauth"
	"github.com/Bajahaw/ai-ui/cmd/settings"
	"github.com/Bajahaw/ai-ui/cmd/utils"

	"github.com/openai/openai-go/v3"
)

// findChatGPTProviderByAccount returns an existing ChatGPT provider for this
// user that is already linked to the given ChatGPT account id (reconnect).
func findChatGPTProviderByAccount(username, accountID string) *Provider {
	for _, p := range providers.GetAll(username) {
		if p.Type != chatgptoauth.ProviderType || p.OAuth == nil {
			continue
		}
		if p.OAuth.AccountID == accountID {
			return p
		}
	}
	return nil
}

// SaveChatGPTProvider stores/updates a ChatGPT OAuth provider for the user,
// fetches models, and returns provider id + a preferred default model id.
//
// Multiple ChatGPT accounts per user are supported: each distinct account_id
// becomes its own provider. Reconnecting the same account updates tokens in place.
func SaveChatGPTProvider(username string, tokens *chatgptoauth.Tokens) (providerID string, modelID string, err error) {
	if tokens == nil || tokens.AccessToken == "" || tokens.AccountID == "" {
		return "", "", fmt.Errorf("invalid tokens")
	}

	// Reuse existing row for this account so reconnect keeps short ids / models.
	if existing := findChatGPTProviderByAccount(username, tokens.AccountID); existing != nil {
		providerID = existing.ID
	} else {
		providerID = chatgptoauth.ProviderIDForAccount(tokens.AccountID, username)
	}

	label := chatgptoauth.DeriveEmail(tokens.IDToken)
	if label == "" {
		// Short, non-secret label so multi-account cards are distinguishable.
		aid := strings.ReplaceAll(tokens.AccountID, "-", "")
		if len(aid) > 6 {
			aid = aid[len(aid)-6:]
		}
		label = "••••" + aid
	}

	p := &Provider{
		ID:      providerID,
		Type:    chatgptoauth.ProviderType,
		BaseURL: chatgptoauth.ProviderBaseURL,
		APIKey:  "",
		User:    username,
		Headers: map[string]string{
			"label": label,
		},
		OAuth: tokens,
	}

	if err := providers.Upsert(p); err != nil {
		return "", "", err
	}

	// Fetch models with fresh tokens
	client := chatgptoauth.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	models, fetchErr := client.ListModels(ctx, tokens)
	if fetchErr != nil {
		log.Error("Failed to fetch ChatGPT models", "err", fetchErr)
		// still return provider; models can be refreshed later
		return providerID, "", nil
	}

	var modelList []*Model
	var firstEnabled string
	for _, m := range models {
		id := providerID + "/" + m.Slug
		modelList = append(modelList, &Model{
			ID:         id,
			Name:       m.Slug,
			ProviderID: providerID,
			IsEnabled:  true,
		})
		if firstEnabled == "" {
			firstEnabled = id
		}
	}
	if len(modelList) > 0 {
		if err := providers.SaveModels(modelList, username); err != nil {
			log.Error("Failed to save ChatGPT models", "err", err)
		}
		_ = providers.DeleteModelsNotIn(providerID, func() []string {
			ids := make([]string, len(modelList))
			for i, m := range modelList {
				ids[i] = m.ID
			}
			return ids
		}())
	}

	// Prefer setting default model for new chats when empty/default gpt-4o
	if firstEnabled != "" {
		// best-effort; settings package already initialized in main
		settings.MaybeSetDefaultModel(username, firstEnabled)
	}

	return providerID, firstEnabled, nil
}

// resolveChatGPTTokens loads and refreshes OAuth tokens for a provider.
func resolveChatGPTTokens(p *Provider) (*chatgptoauth.Tokens, error) {
	if p.OAuth == nil {
		return nil, fmt.Errorf("provider has no ChatGPT OAuth session")
	}
	fresh, changed, err := chatgptoauth.EnsureFresh(p.OAuth)
	if err != nil {
		return nil, err
	}
	if changed {
		p.OAuth = fresh
		if err := providers.Upsert(p); err != nil {
			log.Warn("Failed to persist refreshed ChatGPT tokens", "err", err)
		}
	}
	return fresh, nil
}


type chatGPTStreamHandler struct {
	sc utils.StreamClient
}

func (h chatGPTStreamHandler) OnContent(delta string) {
	if h.sc.Writer != nil {
		_ = utils.SendStreamChunk(h.sc, utils.StreamChunk{Type: utils.CONTENT, Payload: delta})
	}
}

func (h chatGPTStreamHandler) OnReasoning(delta string) {
	if h.sc.Writer != nil {
		_ = utils.SendStreamChunk(h.sc, utils.StreamChunk{Type: utils.REASONING, Payload: delta})
	}
}

func (h chatGPTStreamHandler) OnToolCall(tc chatgptoauth.ResultToolCall) {
	if h.sc.Writer != nil {
		_ = utils.SendStreamChunk(h.sc, utils.StreamChunk{
			Type: utils.TOOL_CALL,
			Payload: ToolCall{
				ID:          tc.ID,
				ReferenceID: tc.ReferenceID,
				Name:        tc.Name,
				Args:        tc.Args,
			},
		})
	}
}

func toChatGPTMessages(messages []SimpleMessage) []chatgptoauth.ChatMessage {
	out := make([]chatgptoauth.ChatMessage, 0, len(messages))
	for _, m := range messages {
		cm := chatgptoauth.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
			Reasoning: m.Reasoning,
			Images:  m.Images,
		}
		if m.Role == "assistant" && m.ToolCall.Name != "" {
			cm.ToolCallID = m.ToolCall.ReferenceID
			if cm.ToolCallID == "" {
				cm.ToolCallID = m.ToolCall.ID
			}
			cm.ToolName = m.ToolCall.Name
			cm.ToolArgs = m.ToolCall.Args
		}
		if m.Role == "tool" {
			cm.ToolCallID = m.ToolCall.ReferenceID
			if cm.ToolCallID == "" {
				cm.ToolCallID = m.ToolCall.ID
			}
			cm.ToolName = m.ToolCall.Name
			cm.ToolOutput = m.ToolCall.Output
			cm.Images = nil // media goes on a follow-up user message, not the tool row
			out = append(out, cm)
			// Resolved tool media lives on SimpleMessage.Images (data URLs).
			if len(m.Images) > 0 {
				out = append(out, chatgptoauth.ChatMessage{
					Role:    "user",
					Content: toolMediaFollowUpText(m),
					Images:  m.Images,
				})
			}
			continue
		}
		out = append(out, cm)
	}
	return out
}

func toChatGPTTools(tools []openai.ChatCompletionToolUnionParam) []chatgptoauth.ToolDef {
	out := make([]chatgptoauth.ToolDef, 0, len(tools))
	for _, t := range tools {
		// Marshal to generic map to extract function fields from union
		raw, err := json.Marshal(t)
		if err != nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		// shapes: {type,function:{name,description,parameters}} or nested OfFunction
		fn, _ := m["function"].(map[string]any)
		if fn == nil {
			// try openai-go union encoding
			if of, ok := m["OfFunction"].(map[string]any); ok {
				fn, _ = of["function"].(map[string]any)
				if fn == nil {
					fn = of
				}
			}
		}
		if fn == nil {
			continue
		}
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		var params json.RawMessage
		if p, ok := fn["parameters"]; ok {
			params, _ = json.Marshal(p)
		}
		if name == "" {
			continue
		}
		out = append(out, chatgptoauth.ToolDef{
			Name:        name,
			Description: desc,
			Parameters:  params,
		})
	}
	return out
}

func sendChatGPTCompletion(ctx context.Context, provider *Provider, model string, params RequestParams, sc *utils.StreamClient) (*ChatCompletionMessage, error) {
	tokens, err := resolveChatGPTTokens(provider)
	if err != nil {
		return nil, err
	}

	client := chatgptoauth.NewClient()
	messages := toChatGPTMessages(params.Messages)
	tools := toChatGPTTools(params.Tools)
	effort := string(params.ReasoningEffort)

	var handler chatgptoauth.StreamHandler
	if sc != nil && sc.Writer != nil {
		utils.AddStreamHeaders(sc.Writer)
		handler = chatGPTStreamHandler{sc: *sc}
	}

	start := time.Now()
	result, err := client.ChatStream(ctx, tokens, model, messages, tools, effort, handler)
	if err != nil {
		if isContextCanceled(err) || isContextCanceled(ctx.Err()) {
			return &ChatCompletionMessage{Cancelled: true}, nil
		}
		return nil, err
	}

	duration := time.Since(start)
	var toolCalls []ToolCall
	if !result.Cancelled {
		for _, tc := range result.ToolCalls {
			toolCalls = append(toolCalls, ToolCall{
				ID:          tc.ID,
				ReferenceID: tc.ReferenceID,
				Name:        tc.Name,
				Args:        tc.Args,
			})
		}
	}

	promptTokens := result.PromptTokens
	completionTokens := result.CompletionTokens
	if completionTokens <= 0 {
		completionTokens = utils.EstimateTokens(result.Content + result.Reasoning)
	}
	if promptTokens <= 0 {
		promptTokens = utils.EstimateTokens(estimatePromptText(params.Messages))
	}
	seconds := duration.Seconds()
	if seconds == 0 {
		seconds = 1
	}
	speed := float64(completionTokens) / seconds

	stats := utils.StreamStats{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Speed:            math.Round(speed*10) / 10,
	}
	if len(toolCalls) > 0 {
		toolCalls[0].TokenCount = stats.CompletionTokens
		toolCalls[0].ContextSize = stats.PromptTokens
	}

	return &ChatCompletionMessage{
		Content:   result.Content,
		Reasoning: result.Reasoning,
		ToolCalls: toolCalls,
		Stats:     stats,
		Cancelled: result.Cancelled || isContextCanceled(ctx.Err()),
	}, nil
}
