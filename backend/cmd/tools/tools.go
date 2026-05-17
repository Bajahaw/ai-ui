package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	fs "github.com/Bajahaw/ai-ui/cmd/files"

	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/evgensoft/ddgo"
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

func ExecuteMCPTool(toolCall providers.ToolCall, user, convID string) providers.ToolOutput {
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
			return providers.ToolOutput{Content: "Tool call approval timed out."}
		case approved := <-responseChan:
			if !approved {
				return providers.ToolOutput{Content: "Tool call was not approved."}
			}
		}
	}

	if server.ID == "default" {
		switch tool.Name {
		case "search_ddgs":
			return ddgsTool(toolCall.Args)
		case "get_weather":
			return weatherTool()
		case "search_document":
			return searchDocumentTool(toolCall.Args)
		case "read_document_page":
			return readDocumentPageTool(toolCall.Args)
		case "view_document_page":
			return viewDocumentPageTool(toolCall.Args, user, convID)
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

func GetBuiltInTools() []*Tool {
	return []*Tool{
		{
			ID:          "search_ddgs",
			Name:        "search_ddgs",
			MCPServerID: "default",
			Description: "Search the web using DuckDuckGo",
			InputSchema: `{"type": "object","properties": {"query": {"type": "string","description": "The search query to look up on DuckDuckGo"}},"required": ["query"]}`,
			IsEnabled:   true,
		},
		{
			ID:          "get_weather",
			Name:        "get_weather",
			MCPServerID: "default",
			Description: "Get the current weather",
			InputSchema: `{"type": "object","properties": {"location": {"type": "string","description": "The location to get weather for"}},"required": ["location"]}`,
			IsEnabled:   true,
		},
		{
			ID:          "search_document",
			Name:        "search_document",
			MCPServerID: "default",
			Description: "Search a specific attached document for a keyword or phrase constraint. Returns best matching pages.",
			InputSchema: `{"type":"object","properties":{"file_id":{"type":"string","description":"The id of the attached file"},"query":{"type":"string","description":"The keyword or phrase to search for"}},"required":["file_id","query"]}`,
			IsEnabled:   true,
		},
		{
			ID:          "read_document_page",
			Name:        "read_document_page",
			MCPServerID: "default",
			Description: "Read the extracted text of a specific page from an attached document.",
			InputSchema: `{"type":"object","properties":{"file_id":{"type":"string","description":"The id of the attached file"},"page_number":{"type":"integer","description":"The 0-based page number to read"}},"required":["file_id","page_number"]}`,
			IsEnabled:   true,
		},
		{
			ID:          "view_document_page",
			Name:        "view_document_page",
			MCPServerID: "default",
			Description: "Get a screenshot of a specific document page. Use this when the user specifically mentions looking at an image, chart, format, or layout in the document. Pass array of files_ids via file_id property if needed.",
			InputSchema: `{"type":"object","properties":{"file_id":{"type":"string","description":"The id of the attached file"},"page_number":{"type":"integer","description":"The 0-based page number to view"}},"required":["file_id","page_number"]}`,
			IsEnabled:   true,
		},
	}
}

func ddgsTool(q string) providers.ToolOutput {
	var m map[string]any
	err := json.Unmarshal([]byte(q), &m)
	if err != nil {
		return providers.ToolOutput{Content: "Error parsing tool arguments."}
	}

	queryVal, ok := m["query"]
	if !ok || queryVal == nil {
		return providers.ToolOutput{Content: "Error: 'query' parameter is required."}
	}

	query, ok := queryVal.(string)
	if !ok {
		return providers.ToolOutput{Content: "Error: 'query' parameter must be a string."}
	}

	result, err := ddgo.Query(query, 5)
	if err != nil {
		return providers.ToolOutput{Content: "Error occurred while searching DuckDuckGo."}
	}

	// combine results into a single string
	output := "Search Success!\n Search Results:\n"
	for i, res := range result {
		output += fmt.Sprintf("%d. %s\n%s\n%s\n\n", i+1, res.Title, res.Info, res.URL)
	}

	if len(result) == 0 {
		output = "Search failed! Probably bot detection triggered."
	}

	return providers.ToolOutput{Content: output}
}

func weatherTool() providers.ToolOutput {
	// simulate delay
	time.Sleep(2 * time.Second)
	return providers.ToolOutput{Content: "Temperature: 22°C, Condition: Sunny"}
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
		res.WriteString(page.Content)
		res.WriteString("\n\n")
	}

	return providers.ToolOutput{Content: res.String()}
}

func readDocumentPageTool(args string) providers.ToolOutput {
	var params struct {
		FileID     string `json:"file_id"`
		PageNumber int    `json:"page_number"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	page, err := files.GetPage(params.FileID, params.PageNumber)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error reading document page: %v", err)}
	}

	content := page.Content

	return providers.ToolOutput{Content: content}
}

func viewDocumentPageTool(args, user, convID string) providers.ToolOutput {
	var params struct {
		FileID     string `json:"file_id"`
		PageNumber int    `json:"page_number"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	docs := files.GetAllConversationAttachments(convID)
	doc := findAttachment(docs, params.FileID)
	if doc == nil {
		return providers.ToolOutput{Content: fmt.Sprintf("Unable to find document with id %s in this conversation", params.FileID)}
	}

	imgData, err := fs.RenderPDFPageAsBase64(doc.File.Path, params.PageNumber, user)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error rendering document page: %v", err)}
	}

	return providers.ToolOutput{File: imgData.ID}
}

func findAttachment(m map[int][]fs.Attachment, targetID string) *fs.Attachment {
	for _, attachments := range m {
		for _, att := range attachments {
			if att.ID == targetID {
				return &att
			}
		}
	}
	return nil
}
