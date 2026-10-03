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
	// The model may name any tool, not only the ones it was sent.
	if !tool.IsEnabled {
		return providers.ToolOutput{Content: fmt.Sprintf("Tool '%s' is disabled.", tool.Name)}
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
		case "browser_sandbox":
			return browserSandboxTool(ctx, toolCall.ID, toolCall.Args, user)
		}
	}

	log.Debug("Executing MCP tool", "tool", tool.Name, "server", server.Name, "args", toolCall.Args)
	log.Debug("MCP tool input schema", "schema", tool.InputSchema, "args", toolCall.Args)

	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Args), &args); err != nil {
		log.Error("Error unmarshaling tool arguments", "err", err)
		return providers.ToolOutput{Content: "Error parsing tool arguments."}
	}

	result, err := mcpSessionManager.callTool(ctx, *server, &mcp.CallToolParams{
		Name:      toolCall.Name,
		Arguments: args,
	})
	if err != nil {
		log.Error("Error calling tool on MCP server", "err", err)
		return providers.ToolOutput{Content: "Tool execution failed!"}
	}

	out := parseMCPToolResult(result, user)
	log.Debug("Parsed MCP tool result", "tool", tool.Name, "contentLen", len(out.Content), "fileID", out.FileID, "isError", result.IsError)
	return out
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

	return providers.ToolOutput{FileID: imgData.ID, Content: fmt.Sprintf("Rendered page %d of document %s as image. Screenshot ID: %s Path: /%s", params.PageNumber, docs[0].Name, imgData.ID, imgData.Path)}
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
