package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	fs "github.com/Bajahaw/ai-ui/cmd/files"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/skills"
	"github.com/google/uuid"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool struct {
	ID              string `json:"id"`
	MCPServerID     string `json:"mcp_server_id,omitempty"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	InputSchema     string `json:"input_schema,omitempty"`
	RequireApproval bool   `json:"require_approval"`
	IsEnabled       bool   `json:"is_enabled"`
}

type PendingToolCall struct {
	User     string
	ToolCall providers.ToolCall
	Channel  chan bool
}

type ToolCallManager struct {
	pending map[string]PendingToolCall
	mu      sync.Mutex
}

var toolCallManager = ToolCallManager{
	pending: make(map[string]PendingToolCall),
	mu:      sync.Mutex{},
}

// // ExecuteListOfToolCalls executes a list of tool calls parallelly and returns them with outputs.
// func ExecuteListOfToolCalls(toolCalls []ToolCall, user string) []ToolCall {
// 	results := make([]ToolCall, len(toolCalls))
// 	ch := make(chan ToolCall)

// 	for _, tc := range toolCalls {
// 		go func(tc ToolCall) {
// 			// output := ExecuteToolCall(tc, user)
// 			// tc.Output = output
// 			ch <- tc
// 		}(tc)
// 	}

// 	for i := range toolCalls {
// 		result := <-ch
// 		results[i] = result
// 	}

// 	return results
// }

func ExecuteMCPTool(ctx context.Context, toolCall providers.ToolCall, user, convID string) providers.ToolOutput {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return providers.ToolOutput{Content: "Tool call was cancelled."}
	}

	tool, err := tools.GetByName(toolCall.Name, user)
	if err != nil {
		log.Error("Error retrieving tool", "err", err)
		return providers.ToolOutput{Content: "Error occurred while retrieving tool."}
	}

	server, err := mcps.GetByID(tool.MCPServerID, user)
	if err != nil {
		log.Error("Error retrieving MCP server", "err", err)
		return providers.ToolOutput{Content: "Error occurred while retrieving MCP server."}
	}

	// Inherit generation cancel; still bound individual tool runtime.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if tool.RequireApproval {
		// wait for approval
		responseChan := make(chan bool, 1)

		toolCallManager.mu.Lock()
		toolCallManager.pending[toolCall.ID] = PendingToolCall{
			User:     user,
			ToolCall: toolCall,
			Channel:  responseChan,
		}
		toolCallManager.mu.Unlock()

		defer func() {
			toolCallManager.mu.Lock()
			delete(toolCallManager.pending, toolCall.ID)
			toolCallManager.mu.Unlock()
		}()

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return providers.ToolOutput{Content: "Tool call was cancelled."}
			}
			return providers.ToolOutput{Content: "Tool call approval timed out."}
		case approved := <-responseChan:
			if !approved {
				return providers.ToolOutput{Content: "Tool call was not approved."}
			}
		}
	}

	if strings.HasPrefix(server.ID, "default") {
		switch tool.Name {
		case "read_skill":
			return readSkillTool(toolCall.Args, user)
		case "search_document":
			return searchDocumentTool(toolCall.Args)
		case "read_document_page":
			return readDocumentPageTool(toolCall.Args)
		case "view_document_page":
			return viewDocumentPageTool(toolCall.Args, user, convID)
		case "generate_image":
			return generateImageTool(toolCall.Args, user, convID)
		case "http_request":
			return httpRequestTool(toolCall.Args, user)
		}
	}

	log.Debug("Executing MCP tool", "tool", tool.Name, "server", server.Name, "args", toolCall.Args)
	log.Debug("MCP tool input schema", "schema", tool.InputSchema, "args", toolCall.Args)

	var session *mcp.ClientSession
	session, ok := mcpSessionManager.get(server.ID)
	if !ok {
		client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
		headers := map[string]string{
			"Authorization": "Bearer " + server.APIKey,
		}
		for k, v := range server.Headers {
			headers[k] = v
		}

		session, err = client.Connect(ctx, &mcp.StreamableClientTransport{
			Endpoint:   server.Endpoint,
			HTTPClient: httpClientWithCustomHeaders(headers),
		}, nil)

		if err != nil {
			log.Error("Error connecting to MCP server", "err", err)
			return providers.ToolOutput{Content: "Error connecting to MCP server"}
		}

		mcpSessionManager.add(server.ID, session)
	}

	// CallToolParams.Arguments field expects any type
	// that will be marshaled to JSON by the SDK itself,
	// not a pre-stringified JSON.
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Args), &args); err != nil {
		log.Error("Error unmarshaling tool arguments", "err", err)
		return providers.ToolOutput{Content: "Error parsing tool arguments."}
	}

	params := &mcp.CallToolParams{
		Name:      toolCall.Name,
		Arguments: args,
	}

	result, err := session.CallTool(ctx, params)
	if err != nil {
		log.Error("Error calling tool on MCP server", "err", err)

		// Remove failed session from cache to force reconnection on next call
		mcpSessionManager.sessions.Delete(server.ID)

		// session.Close() // this might throw the same error if connection is broken

		return providers.ToolOutput{Content: "Tool execution failed!"}
	}

	output := result.Content
	// output is an array of mcp.Content objects
	log.Debug(len(output))
	log.Debug(output)

	rawJSON, _ := json.Marshal(output)
	return providers.ToolOutput{Content: string(rawJSON)}
}

func GetAvailableTools(user string) []*Tool {
	// builtInTools := GetBuiltInTools()
	// mcpTools := toolRepo.GetAllTools()

	allTools := tools.GetAll(user)
	var enabledTools []*Tool
	for _, t := range allTools {
		if t.IsEnabled {
			enabledTools = append(enabledTools, t)
		}
	}
	return enabledTools
}

// GetBuiltInTools returns the platform built-in tools.
// MCPServerID is left empty; callers must set it to the owning server ID
// (e.g. "default-{user}") before persisting, so the Tools FK is satisfied.
func GetBuiltInTools() []*Tool {
	return []*Tool{
		{
			ID:          uuid.New().String(),
			Name:        "search_document",
			Description: "Search a specific attached document for a keyword or phrase constraint. Returns best matching pages.",
			InputSchema: `{"type":"object","properties":{"file_id":{"type":"string","description":"The id of the attached file"},"query":{"type":"string","description":"The keyword or phrase to search for"}},"required":["file_id","query"]}`,
			IsEnabled:   true,
		},
		{
			ID:          uuid.New().String(),
			Name:        "read_document_page",
			Description: "Read the extracted text of specific pages from a retreivable attached document.",
			InputSchema: `{"type":"object","properties":{"file_id":{"type":"string","description":"The id of the attached file"},"start_page":{"type":"integer","description":"The 1-based page number to start reading from"},"end_page":{"type":"integer","description":"The 1-based page number to end reading at (inclusive)"}},"required":["file_id","start_page","end_page"]}`,
			IsEnabled:   true,
		},
		{
			ID:          uuid.New().String(),
			Name:        "view_document_page",
			Description: "Get a screenshot of a specific PDF page (works only with pdf!). Use this when the user specifically mentions looking at an image, chart, format, or layout in a PDF. Pass array of files_ids via file_id property if needed.",
			InputSchema: `{"type":"object","properties":{"file_id":{"type":"string","description":"The id of the attached file"},"page_number":{"type":"integer","description":"The 1-based page number to view"}},"required":["file_id","page_number"]}`,
			IsEnabled:   true,
		},
		{
			ID:          uuid.New().String(),
			Name:        "generate_image",
			Description: "Generate an image via AI model currently selected by user. Pass the user prompt exactly as is, unless user requested you to enhance it. Embed the resulting image file in the chat. ONlY call when user asks for `AI generated image`!, and Never call more than once",
			InputSchema: `{"type":"object","properties":{"prompt":{"type":"string","description":"A detailed prompt for the image generation model"}},"required":["prompt"]}`,
			IsEnabled:   true,
		},
		{
			ID:          uuid.New().String(),
			Name:        "read_skill",
			Description: "Read the full content of a specific skill by its name. Choose the skill that best matches the user's task from the <available_skills> section in the system prompt, then read its full instructions using this tool.",
			InputSchema: `{"type":"object","properties":{"name":{"type":"string","description":"The exact name of the skill to read"}},"required":["name"]}`,
			IsEnabled:   true,
		},
		{
			ID:              uuid.New().String(),
			Name:            "http_request",
			Description:     "HTTPS request to a public host (DNS names only; no IPs; private/loopback/metadata blocked). Request body may be a JSON object/array (preferred for JSON APIs) or a string. Response bodies are reduced by default (HTML scrub, base64/data-URI omit, 32KiB text cap). Binary bodies omitted (metadata+SHA256). verbose skips reduction and raises the text cap to 256KiB. For JSON responses, pass json_pointers (RFC 6901) to return only selected values instead of the full body. For HTML responses, pass css_selectors to return only matching elements as compact JSON (text + href/src/alt/title/aria-label); max 40 matches per selector. json_pointers and css_selectors are mutually exclusive. Secrets: $secrets.NAME$ in headers or URL path/query only — not host or body.",
			InputSchema:     `{"type":"object","properties":{"url":{"type":"string","description":"Full https URL with a DNS hostname (not an IP). $secrets.NAME$ allowed in path and query."},"method":{"type":"string","description":"GET, HEAD, POST, PUT, PATCH, or DELETE","default":"GET"},"headers":{"type":"object","additionalProperties":{"type":"string"},"description":"Optional request headers. $secrets.NAME$ allowed in values. Host/Transfer-Encoding etc. rejected."},"body":{"description":"Optional body for POST/PUT/PATCH/DELETE (max 1MiB). Prefer a JSON object/array for JSON APIs (Content-Type application/json is set if omitted). String is sent as-is for text/form/raw. No secret placeholders."},"verbose":{"type":"boolean","description":"If true, skip body reduction and raise text output cap from 32KiB to 256KiB. Use only when you need the raw/larger payload.","default":false},"json_pointers":{"type":"array","items":{"type":"string"},"description":"Optional RFC 6901 JSON Pointers into a JSON response body (e.g. \"/data/items/0/id\"). Returns a JSON object keyed by each pointer. Empty string is the document root. Requires JSON body; missing paths error. Mutually exclusive with css_selectors."},"css_selectors":{"type":"array","items":{"type":"string"},"description":"Optional CSS selectors into an HTML response (e.g. \"article h1\", \"a.main-link\"). Returns a JSON object keyed by each selector → array of matches with text and useful attrs (href, src, alt, title, aria-label). Empty match lists are allowed. Max 40 matches per selector. Mutually exclusive with json_pointers."}},"required":["url"]}`,
			RequireApproval: true,
			IsEnabled:       true,
		},
	}
}

func searchDocumentTool(args string) providers.ToolOutput {
	var params struct {
		FileID string `json:"file_id"`
		Query  string `json:"query"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	pages, err := files.SearchPages(params.FileID, params.Query, 10)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error searching document: %v", err)}
	}

	var res strings.Builder
	for _, page := range pages {
		res.WriteString(fmt.Sprintf("@Page %d: %s\n\n", page.PageNumber, page.Content))
	}

	if res.Len() == 0 {
		return providers.ToolOutput{Content: "No matching content found in document."}
	}

	return providers.ToolOutput{Content: res.String()}
}

func readDocumentPageTool(args string) providers.ToolOutput {
	var params struct {
		FileID    string `json:"file_id"`
		StartPage int    `json:"start_page"`
		EndPage   int    `json:"end_page"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	if params.StartPage > params.EndPage {
		return providers.ToolOutput{Content: "error: start_page must be less than or equal to end_page"}
	}

	pages, err := files.GetPagesRange(params.FileID, params.StartPage, params.EndPage)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error reading document pages: %v", err)}
	}

	if len(pages) == 0 {
		return providers.ToolOutput{Content: "No pages found in the specified range."}
	}

	var contentBuilder strings.Builder
	for _, page := range pages {
		contentBuilder.WriteString(page.Content)
		contentBuilder.WriteString("\n\n")
	}

	return providers.ToolOutput{Content: contentBuilder.String()}
}

func viewDocumentPageTool(args, user, convID string) providers.ToolOutput {
	var params struct {
		FileID     string `json:"file_id"`
		PageNumber int    `json:"page_number"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	docs, err := files.GetByIDs([]string{params.FileID}, user)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error finding document: %v", err)}
	}
	if len(docs) == 0 {
		return providers.ToolOutput{Content: fmt.Sprintf("Unable to find document with id %s", params.FileID)}
	}

	imgData, err := fs.RenderDocPageAsImage(docs[0].Path, params.PageNumber, user)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error rendering document page: %v", err)}
	}

	return providers.ToolOutput{File: imgData.ID, Content: fmt.Sprintf("Rendered page %d of document %s as image. Screenshot ID: %s Path: /%s", params.PageNumber, docs[0].Name, imgData.ID, imgData.Path)}
}

func readSkillTool(args, user string) providers.ToolOutput {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}
	if params.Name == "" {
		return providers.ToolOutput{Content: "Error: 'name' parameter is required."}
	}

	s, err := skills.GetByName(params.Name, user)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("Skill '%s' not found. Select a skill from the <available_skills> section in the system prompt.", params.Name)}
	}

	return providers.ToolOutput{Content: s.Content}
}
