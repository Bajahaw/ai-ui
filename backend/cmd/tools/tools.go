package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

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

type ToolCall struct {
	ID          string `json:"id"`
	ReferenceID string `json:"ref_id"`
	ConvID      string `json:"conv_id,omitempty"`
	MessageID   int    `json:"message_id"`
	Name        string `json:"name"`
	Args        string `json:"args,omitempty"`
	Output      string `json:"tool_output,omitempty"`
}

type PendingToolCall struct {
	User     string
	ToolCall ToolCall
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

func ExecuteMCPTool(toolCall ToolCall, user string) string {
	tool, err := tools.GetByName(toolCall.Name)
	if err != nil {
		log.Error("Error retrieving tool", "err", err)
		return "Error occurred while retrieving tool."
	}

	server, err := mcps.GetByID(tool.MCPServerID, user)
	if err != nil {
		log.Error("Error retrieving MCP server", "err", err)
		return "Error occurred while retrieving MCP server."
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
			return "Tool call approval timed out."
		case approved := <-responseChan:
			if !approved {
				return "Tool call was not approved."
			}
		}
	}

	if server.ID == "default" {
		switch tool.Name {
		case "search_ddgs":
			return ddgsTool(toolCall.Args)
		case "get_weather":
			return weatherTool()
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

		session, err = client.Connect(ctx, &mcp.StreamableClientTransport{
			Endpoint:   server.Endpoint,
			HTTPClient: httpClientWithCustomHeaders(headers),
		}, nil)

		if err != nil {
			log.Error("Error connecting to MCP server", "err", err)
			return "Error connecting to MCP server"
		}

		mcpSessionManager.add(server.ID, session)
	}

	// CallToolParams.Arguments field expects any type
	// that will be marshaled to JSON by the SDK itself,
	// not a pre-stringified JSON.
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Args), &args); err != nil {
		log.Error("Error unmarshaling tool arguments", "err", err)
		return "Error parsing tool arguments."
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

		return "Tool execution failed!"
	}

	output := result.Content
	// output is an array of mcp.Content objects
	log.Debug(len(output))
	log.Debug(output)

	rawJSON, _ := json.Marshal(output)
	return string(rawJSON)
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
	}
}

func ddgsTool(q string) string {
	var m map[string]any
	err := json.Unmarshal([]byte(q), &m)
	if err != nil {
		return "Error parsing tool arguments."
	}

	queryVal, ok := m["query"]
	if !ok || queryVal == nil {
		return "Error: 'query' parameter is required."
	}

	query, ok := queryVal.(string)
	if !ok {
		return "Error: 'query' parameter must be a string."
	}

	result, err := ddgo.Query(query, 5)
	if err != nil {
		return "Error occurred while searching DuckDuckGo."
	}

	// combine results into a single string
	output := "Search Success!\n Search Results:\n"
	for i, res := range result {
		output += fmt.Sprintf("%d. %s\n%s\n%s\n\n", i+1, res.Title, res.Info, res.URL)
	}

	if len(result) == 0 {
		output = "Search failed! Probably bot detection triggered."
	}

	return output
}

func weatherTool() string {
	// simulate delay
	time.Sleep(2 * time.Second)
	return "Temperature: 22°C, Condition: Sunny"
}
