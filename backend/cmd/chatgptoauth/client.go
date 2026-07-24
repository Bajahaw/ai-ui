package chatgptoauth

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const defaultCodexVersion = "0.144.1"

// ModelInfo is a Codex model entry.
type ModelInfo struct {
	Slug           string `json:"slug"`
	Visibility     string `json:"visibility,omitempty"`
	SupportedInAPI *bool  `json:"supported_in_api,omitempty"`
}

// ChatMessage is a simplified chat message for the Codex bridge.
type ChatMessage struct {
	Role       string
	Content    string
	Reasoning  string
	ToolCallID string
	ToolName   string
	ToolArgs   string
	ToolOutput string
	Images     []string
}

// ToolDef is an OpenAI-style function tool.
type ToolDef struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// ChatResult is the completed assistant response.
type ChatResult struct {
	Content          string
	Reasoning        string
	ToolCalls        []ResultToolCall
	PromptTokens     int
	CompletionTokens int
	Cancelled        bool
}

// ResultToolCall is a tool invocation from the model.
type ResultToolCall struct {
	ID          string
	ReferenceID string
	Name        string
	Args        string
}

// StreamHandler receives incremental stream events.
type StreamHandler interface {
	OnContent(delta string)
	OnReasoning(delta string)
	OnToolCall(tc ResultToolCall)
}

// Client talks to the ChatGPT Codex backend with OAuth tokens.
type Client struct {
	HTTP     *http.Client
	BaseURL  string
	CodexVer string
}

func NewClient() *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: 30 * time.Minute},
		BaseURL:  DefaultCodexURL,
		CodexVer: defaultCodexVersion,
	}
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return DefaultCodexURL
}

func (c *Client) authHeaders(t *Tokens) http.Header {
	h := make(http.Header)
	h.Set("Authorization", "Bearer "+t.AccessToken)
	h.Set("chatgpt-account-id", t.AccountID)
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "text/event-stream, application/json")
	if t.IsFedRamp {
		h.Set("X-OpenAI-Fedramp", "true")
	}
	return h
}

// ListModels returns public Codex models for the account.
func (c *Client) ListModels(ctx context.Context, t *Tokens) ([]ModelInfo, error) {
	ver := c.CodexVer
	if ver == "" {
		ver = defaultCodexVersion
	}
	url := fmt.Sprintf("%s/models?client_version=%s", c.base(), ver)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header = c.authHeaders(t)
	req.Header.Del("Content-Type")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("models request failed (HTTP %d): %s", resp.StatusCode, truncate(string(body), 300))
	}

	var parsed struct {
		Models []ModelInfo `json:"models"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid models response: %w", err)
	}
	out := make([]ModelInfo, 0, len(parsed.Models))
	for _, m := range parsed.Models {
		if m.Slug == "" {
			continue
		}
		if m.SupportedInAPI != nil && !*m.SupportedInAPI {
			continue
		}
		if m.Visibility != "" && m.Visibility != "list" {
			continue
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		for _, m := range parsed.Models {
			if m.Slug != "" {
				out = append(out, m)
			}
		}
	}
	return out, nil
}

// ChatStream sends a chat request to Codex /responses and streams results.
func (c *Client) ChatStream(ctx context.Context, t *Tokens, model string, messages []ChatMessage, tools []ToolDef, reasoningEffort string, handler StreamHandler) (*ChatResult, error) {
	body, err := buildResponsesBody(model, messages, tools, reasoningEffort, true)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = c.authHeaders(t)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("codex responses failed (HTTP %d): %s", resp.StatusCode, truncate(string(b), 500))
	}

	return c.consumeSSE(ctx, resp.Body, handler)
}

func (c *Client) consumeSSE(ctx context.Context, r io.Reader, handler StreamHandler) (*ChatResult, error) {
	result := &ChatResult{}
	toolBuffers := map[string]*ResultToolCall{}
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var eventName string
	var dataLines []string

	flush := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = nil
		en := eventName
		eventName = ""

		if data == "[DONE]" {
			return io.EOF
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			return nil
		}
		typ, _ := payload["type"].(string)
		if typ == "" {
			typ = en
		}

		switch typ {
		case "response.output_text.delta", "response.output_text.delta.partial":
			if d, ok := payload["delta"].(string); ok && d != "" {
				result.Content += d
				if handler != nil {
					handler.OnContent(d)
				}
			}
		case "response.reasoning_text.delta", "response.reasoning_summary_text.delta":
			if d, ok := payload["delta"].(string); ok && d != "" {
				result.Reasoning += d
				if handler != nil {
					handler.OnReasoning(d)
				}
			}
		case "response.output_item.added":
			item, _ := payload["item"].(map[string]any)
			if item == nil {
				break
			}
			itemType, _ := item["type"].(string)
			if itemType == "function_call" || itemType == "custom_tool_call" {
				callID, _ := item["call_id"].(string)
				if callID == "" {
					callID, _ = item["id"].(string)
				}
				name, _ := item["name"].(string)
				if callID == "" {
					callID = uuid.NewString()
				}
				tc := &ResultToolCall{
					ID:          uuid.NewString(),
					ReferenceID: callID,
					Name:        name,
				}
				toolBuffers[callID] = tc
			}
		case "response.function_call_arguments.delta":
			callID, _ := payload["call_id"].(string)
			if callID == "" {
				callID, _ = payload["item_id"].(string)
			}
			delta, _ := payload["delta"].(string)
			if tc, ok := toolBuffers[callID]; ok {
				tc.Args += delta
			}
		case "response.function_call_arguments.done":
			callID, _ := payload["call_id"].(string)
			if callID == "" {
				callID, _ = payload["item_id"].(string)
			}
			args, _ := payload["arguments"].(string)
			if tc, ok := toolBuffers[callID]; ok {
				if args != "" {
					tc.Args = args
				}
				if handler != nil {
					handler.OnToolCall(*tc)
				}
				result.ToolCalls = append(result.ToolCalls, *tc)
			}
		case "response.output_item.done":
			item, _ := payload["item"].(map[string]any)
			if item == nil {
				break
			}
			itemType, _ := item["type"].(string)
			if itemType == "message" {
				if content, ok := item["content"].([]any); ok {
					for _, part := range content {
						pm, _ := part.(map[string]any)
						if pm == nil {
							continue
						}
						pt, _ := pm["type"].(string)
						text, _ := pm["text"].(string)
						if (pt == "output_text" || pt == "text") && text != "" && result.Content == "" {
							result.Content = text
							if handler != nil {
								handler.OnContent(text)
							}
						}
					}
				}
				break
			}
			if itemType != "function_call" && itemType != "custom_tool_call" {
				break
			}
			callID, _ := item["call_id"].(string)
			if callID == "" {
				callID, _ = item["id"].(string)
			}
			name, _ := item["name"].(string)
			args, _ := item["arguments"].(string)
			tc, ok := toolBuffers[callID]
			if !ok {
				tc = &ResultToolCall{
					ID:          uuid.NewString(),
					ReferenceID: callID,
					Name:        name,
					Args:        args,
				}
				toolBuffers[callID] = tc
			} else {
				if name != "" {
					tc.Name = name
				}
				if args != "" {
					tc.Args = args
				}
			}
			already := false
			for _, existing := range result.ToolCalls {
				if existing.ReferenceID == tc.ReferenceID {
					already = true
					break
				}
			}
			if !already {
				if handler != nil {
					handler.OnToolCall(*tc)
				}
				result.ToolCalls = append(result.ToolCalls, *tc)
			}
		case "response.completed":
			if respObj, ok := payload["response"].(map[string]any); ok {
				if usage, ok := respObj["usage"].(map[string]any); ok {
					result.PromptTokens = asInt(usage["input_tokens"])
					result.CompletionTokens = asInt(usage["output_tokens"])
				}
				if result.Content == "" {
					if output, ok := respObj["output"].([]any); ok {
						result.Content = extractOutputText(output)
					}
				}
			}
			return io.EOF
		case "response.failed", "error":
			msg := "codex response failed"
			if e, ok := payload["error"].(map[string]any); ok {
				if m, ok := e["message"].(string); ok && m != "" {
					msg = m
				}
			}
			if m, ok := payload["message"].(string); ok && m != "" {
				msg = m
			}
			return fmt.Errorf("%s", msg)
		case "response.cancelled", "response.canceled", "response.incomplete":
			result.Cancelled = true
			return io.EOF
		}
		return nil
	}

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			result.Cancelled = true
			return result, nil
		}
		line := scanner.Text()
		if line == "" {
			if err := flush(); err == io.EOF {
				return result, nil
			} else if err != nil {
				return result, err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := flush(); err != nil && err != io.EOF {
		return result, err
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			result.Cancelled = true
			return result, nil
		}
		return result, err
	}
	return result, nil
}

func buildResponsesBody(model string, messages []ChatMessage, tools []ToolDef, reasoningEffort string, stream bool) ([]byte, error) {
	var instructions string
	input := make([]any, 0, len(messages))

	for _, m := range messages {
		switch m.Role {
		case "system":
			if instructions == "" {
				instructions = m.Content
			} else {
				instructions += "\n\n" + m.Content
			}
		case "user":
			content := []any{}
			if m.Content != "" {
				content = append(content, map[string]any{"type": "input_text", "text": m.Content})
			}
			for _, img := range m.Images {
				content = append(content, map[string]any{
					"type":      "input_image",
					"image_url": img,
				})
			}
			if len(content) == 0 {
				content = append(content, map[string]any{"type": "input_text", "text": ""})
			}
			input = append(input, map[string]any{"role": "user", "content": content})
		case "assistant":
			if m.ToolName != "" && m.ToolCallID != "" {
				input = append(input, map[string]any{
					"type":      "function_call",
					"call_id":   m.ToolCallID,
					"name":      m.ToolName,
					"arguments": m.ToolArgs,
				})
			} else {
				content := []any{}
				if m.Content != "" {
					content = append(content, map[string]any{"type": "output_text", "text": m.Content})
				}
				if len(content) == 0 {
					content = append(content, map[string]any{"type": "output_text", "text": ""})
				}
				input = append(input, map[string]any{"role": "assistant", "content": content})
			}
		case "tool":
			callID := m.ToolCallID
			if callID == "" {
				callID = m.ToolName
			}
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  m.ToolOutput,
			})
		}
	}

	body := map[string]any{
		"model":        model,
		"instructions": instructions,
		"input":        input,
		"stream":       stream,
		"store":        false,
		"include":      []string{"reasoning.encrypted_content"},
	}

	if reasoningEffort != "" && reasoningEffort != "disabled" {
		body["reasoning"] = map[string]any{"effort": reasoningEffort}
	}

	if len(tools) > 0 {
		toolList := make([]any, 0, len(tools))
		for _, t := range tools {
			params := any(map[string]any{"type": "object", "properties": map[string]any{}})
			if len(t.Parameters) > 0 {
				var p any
				if err := json.Unmarshal(t.Parameters, &p); err == nil {
					params = p
				}
			}
			toolList = append(toolList, map[string]any{
				"type":        "function",
				"name":        t.Name,
				"description": t.Description,
				"parameters":   params,
			})
		}
		body["tools"] = toolList
	}

	return json.Marshal(body)
}

func extractOutputText(output []any) string {
	var b strings.Builder
	for _, item := range output {
		im, _ := item.(map[string]any)
		if im == nil {
			continue
		}
		if t, _ := im["type"].(string); t != "message" {
			continue
		}
		content, _ := im["content"].([]any)
		for _, part := range content {
			pm, _ := part.(map[string]any)
			if pm == nil {
				continue
			}
			pt, _ := pm["type"].(string)
			if pt == "output_text" || pt == "text" {
				if text, ok := pm["text"].(string); ok {
					b.WriteString(text)
				}
			}
		}
	}
	return b.String()
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
