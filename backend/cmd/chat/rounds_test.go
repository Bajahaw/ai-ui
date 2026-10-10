package chat

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/utils"
)

// mockProviderRounds replays scripted completions and records every request.
type mockProviderRounds struct {
	mu       sync.Mutex
	replies  []providers.ChatCompletionMessage
	requests [][]providers.SimpleMessage
}

func (m *mockProviderRounds) SendChatCompletionRequest(params providers.RequestParams) (*providers.ChatCompletionMessage, error) {
	return nil, nil
}

func (m *mockProviderRounds) SendChatCompletionStreamRequest(params providers.RequestParams, sc utils.StreamClient) (*providers.ChatCompletionMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, append([]providers.SimpleMessage(nil), params.Messages...))
	reply := m.replies[0]
	m.replies = m.replies[1:]
	return &reply, nil
}

// Round 1: text A + reasoning R1, calls t1,t2. Round 2: B/R2, call t3.
// Round 3: final C/R3. Each round's text must be sent exactly once, on its
// own first call, and the rebuilt history must match what was sent live.
func TestEnterAgentLoop_RoundsKeepOrderAndRebuildMatches(t *testing.T) {
	mock := &mockProviderRounds{replies: []providers.ChatCompletionMessage{
		{Content: "B", Reasoning: "R2", ToolCalls: []providers.ToolCall{{ID: "t3", ReferenceID: "r3", Name: "tool_c", Args: `{"c":1}`}}},
		{Content: "C", Reasoning: "R3"},
	}}
	teardown := setupTest(t, mock)
	defer teardown()

	old := executeMCPTool
	t.Cleanup(func() { executeMCPTool = old })
	executeMCPTool = func(_ context.Context, tc providers.ToolCall, _, _ string) providers.ToolOutput {
		return providers.ToolOutput{Content: "out-" + tc.Name}
	}

	const convID, model = "conv-rounds", "provider-x/model"
	if _, err := data.DB.Exec(`INSERT INTO Conversations (id, user, title) VALUES (?, ?, ?)`, convID, "test-user", "t"); err != nil {
		t.Fatal(err)
	}
	res, err := data.DB.Exec(
		`INSERT INTO Messages (conv_id, role, model, parent_id, content, reasoning, error, status) VALUES (?, 'assistant', ?, 0, '', '', '', 'pending')`,
		convID, model,
	)
	if err != nil {
		t.Fatal(err)
	}
	id64, _ := res.LastInsertId()
	msgID := int(id64)

	genCtx := providers.StartGeneration(msgID, "test-user")
	defer providers.EndGeneration(msgID)

	msg := &Message{ID: msgID, ConvID: convID, Model: model, Content: "A", Reasoning: "R1"}
	sc := utils.StreamClient{User: "test-user", MessageID: msgID, Writer: &flushRecorder{httptest.NewRecorder()}}
	params := providers.RequestParams{Model: model, User: "test-user", MessageID: msgID, Context: genCtx}

	if _, err := enterAgentLoop(
		[]providers.ToolCall{
			{ID: "t1", ReferenceID: "r1", Name: "tool_a", Args: `{"a":1}`},
			{ID: "t2", ReferenceID: "r2", Name: "tool_b", Args: `{"b":1}`},
		},
		params, msg, convID, "test-user", sc,
	); err != nil {
		t.Fatalf("enterAgentLoop: %v", err)
	}

	if msg.Content != "C" || msg.Reasoning != "R3" {
		t.Fatalf("message must hold only the final round, got content=%q reasoning=%q", msg.Content, msg.Reasoning)
	}

	type row struct{ role, content, reasoning, ref, out string }
	want := []row{
		{"assistant", "A", "R1", "r1", ""},
		{"tool", "", "", "r1", "out-tool_a"},
		{"assistant", "", "", "r2", ""},
		{"tool", "", "", "r2", "out-tool_b"},
		{"assistant", "B", "R2", "r3", ""},
		{"tool", "", "", "r3", "out-tool_c"},
	}
	last := mock.requests[len(mock.requests)-1]
	if len(last) != len(want) {
		t.Fatalf("expected %d messages in final request, got %d", len(want), len(last))
	}
	for i, w := range want {
		g := last[i]
		got := row{g.Role, g.Content, g.Reasoning, g.ToolCall.ReferenceID, g.ToolCall.Output}
		if g.Role == "tool" {
			got.content = ""
		}
		if got != w {
			t.Fatalf("live message %d: got %+v want %+v", i, got, w)
		}
	}

	stored := toolCalls.GetAllByMessageID(msgID)
	if len(stored) != 3 ||
		stored[0].Text != "A" || stored[0].Reasoning != "R1" ||
		stored[1].Text != "" || stored[1].Reasoning != "" ||
		stored[2].Text != "B" || stored[2].Reasoning != "R2" {
		t.Fatalf("unexpected stored tool calls: %+v %+v %+v", *stored[0], *stored[1], *stored[2])
	}

	msg.Status = "completed"
	if _, err := updateMessage(msgID, "test-user", *msg); err != nil {
		t.Fatalf("updateMessage: %v", err)
	}

	// Same model: rebuilt history is the live request plus the final answer.
	rebuilt := buildContext(convID, msgID, "test-user", model)[1:] // drop system prompt
	if len(rebuilt) != len(last)+1 {
		t.Fatalf("expected %d rebuilt messages, got %d", len(last)+1, len(rebuilt))
	}
	// Compare the fields providers actually serialize (assistant entries send
	// id/name/args; tool entries send id/output).
	for i := range last {
		g, w := rebuilt[i], last[i]
		if g.Role != w.Role || g.Content != w.Content || g.Reasoning != w.Reasoning ||
			g.ToolCall.ReferenceID != w.ToolCall.ReferenceID || g.ToolCall.Name != w.ToolCall.Name ||
			(g.Role == "assistant" && g.ToolCall.Args != w.ToolCall.Args) ||
			(g.Role == "tool" && g.ToolCall.Output != w.ToolCall.Output) {
			t.Fatalf("rebuilt message %d differs from live request:\n got  %+v\n want %+v", i, g, w)
		}
	}
	if final := rebuilt[len(rebuilt)-1]; final.Role != "assistant" || final.Content != "C" || final.ToolCall.ID != "" {
		t.Fatalf("expected trailing final answer, got %+v", final)
	}

	// Different model: reasoning is not replayed, text still is.
	for _, m := range buildContext(convID, msgID, "test-user", "other/model") {
		if m.Reasoning != "" {
			t.Fatalf("reasoning replayed for a different model: %+v", m)
		}
	}
}
