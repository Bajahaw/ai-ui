package tools

import (
	"context"
	"encoding/json"
	"fmt"
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
	ID        string `json:"id"`
	ConvID    string `json:"conv_id,omitempty"`
	MessageID int    `json:"message_id"`
	Name      string `json:"name"`
	Args      string `json:"args,omitempty"`
	Output    string `json:"tool_output,omitempty"`
}

func ExecuteToolCall(toolCall ToolCall) string {

	output := ""

	switch toolCall.Name {
	case "search_ddgs":
		output = ddgsTool(toolCall.Args)
	case "get_weather":
		output = weatherTool()
	default:
		output = executeMCPTool(toolCall)
	}

	toolCallsRepo.SaveToolCall(ToolCall{
		ID:        toolCall.ID,
		ConvID:    toolCall.ConvID,
		MessageID: toolCall.MessageID,
		Name:      toolCall.Name,
		Args:      toolCall.Args,
		Output:    output,
	})

	return output
}

func executeMCPTool(toolCall ToolCall) string {
	tool, err := toolRepo.GetToolByName(toolCall.Name)
	if err != nil {
		return "Error occurred while retrieving tool."
	}

	server, err := mcpRepo.GetMCPServer(tool.MCPServerID)
	if err != nil {
		return "Error occurred while retrieving MCP server."
	}

	log.Debug("Executing MCP tool", "tool", tool.Name, "server", server.Name, "args", toolCall.Args)
	log.Debug("MCP tool input schema", "schema", tool.InputSchema, "args", toolCall.Args)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

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

		mcpSessionManager.add(server.ID, session)

		if err != nil {
			log.Error("Error connecting to MCP server", "err", err)
			return "Error connecting to MCP server."
		}
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
		return "Error calling tool on MCP server."
	}

	output := result.Content
	// output is an array of mcp.Content objects
	log.Debug(len(output))
	log.Debug(output)

	rawJSON, _ := json.Marshal(output)
	return string(rawJSON)
}

func GetAvailableTools() []Tool {
	builtInTools := GetBuiltInTools()
	mcpTools := toolRepo.GetAllTools()

	var enabledMcpTools []Tool
	for _, t := range mcpTools {
		if t.IsEnabled {
			enabledMcpTools = append(enabledMcpTools, t)
		}
	}
	return append(builtInTools, enabledMcpTools...)
}

func GetBuiltInTools() []Tool {
	return []Tool{
		{
			Name:        "search_ddgs",
			Description: "Search the web using DuckDuckGo",
			// input schema should be raw JSON
			InputSchema: `{"type": "object","properties": {"query": {"type": "string","description": "The search query to look up on DuckDuckGo"}},"required": ["query"]}`,
		},
		{
			Name:        "get_weather",
			Description: "Get the current weather",
			InputSchema: `{"type": "object","properties": {"location": {"type": "string","description": "The location to get weather for"}},"required": ["location"]}`,
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
	output := "Search Success! citations should be in [host](full-link) format.\n Search Results:\n"
	for i, res := range result {
		output += fmt.Sprintf("%d. %s\n%s\n%s\n\n", i+1, res.Title, res.Info, res.URL)
	}

	if len(result) == 0 {
		output = "Bot detection triggered. Do not use this tool frequently"
	}

	return output
}

func weatherTool() string {
	// simulate delay
	time.Sleep(2 * time.Second)
	return "Temperature: 22°C, Condition: Sunny"
}
