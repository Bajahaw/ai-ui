package tools

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/evgensoft/ddgo"
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
		return "MCP Tool execution not implemented yet."
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

func GetAllTools() []Tool {
	builtInTools := GetBuiltInTools()
	mcpTools := toolRepo.GetAllTools()
	return append(builtInTools, mcpTools...)
}

func GetBuiltInTools() []Tool {
	return []Tool{
		{
			Name:        "search_ddgs",
			Description: "Search the web using DuckDuckGo",
			// input schema should be raw JSON
			InputSchema: "{\"type\": \"object\",\"properties\": {\"query\": {\"type\": \"string\",\"description\": \"The search query to look up on DuckDuckGo\"}},\"required\": [\"query\"]}",
		},
		{
			Name:        "get_weather",
			Description: "Get the current weather",
			InputSchema: "{\"type\": \"object\",\"properties\": {\"location\": {\"type\": \"string\",\"description\": \"The location to get weather for\"}},\"required\": [\"location\"]}",
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
	output := "Search Success! Use the relevant information, and cite the source links using MD links DuckDuckGo Search Results:\n"
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
