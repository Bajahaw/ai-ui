package chatgptoauth

import (
	"encoding/json"
	"testing"
)

func TestBuildResponsesBody_PreToolTextPrecedesFunctionCall(t *testing.T) {
	raw, err := buildResponsesBody("m", []ChatMessage{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "let me check", ToolCallID: "c1", ToolName: "search", ToolArgs: `{}`},
		{Role: "tool", ToolCallID: "c1", ToolName: "search", ToolOutput: "res"},
		{Role: "assistant", ToolCallID: "c2", ToolName: "search", ToolArgs: `{}`},
		{Role: "tool", ToolCallID: "c2", ToolName: "search", ToolOutput: "res2"},
	}, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Input []map[string]any `json:"input"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}

	var kinds []string
	for _, item := range body.Input {
		if typ, ok := item["type"].(string); ok {
			kinds = append(kinds, typ)
		} else {
			kinds = append(kinds, item["role"].(string))
		}
	}
	want := []string{"user", "assistant", "function_call", "function_call_output", "function_call", "function_call_output"}
	if len(kinds) != len(want) {
		t.Fatalf("got items %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("got items %v, want %v", kinds, want)
		}
	}
	text := body.Input[1]["content"].([]any)[0].(map[string]any)["text"]
	if text != "let me check" {
		t.Fatalf("pre-tool text = %v", text)
	}
}
