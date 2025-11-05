package tools

import (
	"time"
)

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	ArgsSchema  map[string]any `json:"args_schema,omitempty"`
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
			ArgsSchema: map[string]any{
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
			ArgsSchema: map[string]any{
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

func ddgsTool(_ string) string {
	time.Sleep(2 * time.Second)
	return "DuckDuckGo search is not yet implemented."
}

func weatherTool() string {
	// simulate delay
	time.Sleep(2 * time.Second)
	return "Temperature: 22°C, Condition: Sunny"
}
