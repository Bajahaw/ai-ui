package tools

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/evgensoft/ddgo"
)

type Tool struct {
	ID          string         `json:"id"`
	MCPServerID string         `json:"mcp_server_id,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
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
		return "Unknown tool: " + toolCall.Name
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
	return []Tool{
		{
			Name:        "search_ddgs",
			Description: "Search the web using DuckDuckGo",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]string{
						"type": "string",
					},
				},
			},
		},
		{
			Name:        "get_weather",
			Description: "Get the current weather",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]string{
						"type": "string",
					},
				},
				"required": []string{"location"},
			},
		},
	}
}

func ddgsTool(q string) string {
	var m map[string]any
	err := json.Unmarshal([]byte(q), &m)
	if err != nil {
		return "Error parsing tool arguments."
	}
	query := m["query"].(string)

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
