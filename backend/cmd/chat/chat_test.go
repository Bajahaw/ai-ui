package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/data"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/skills"
	"github.com/Bajahaw/ai-ui/cmd/tools"
	"github.com/Bajahaw/ai-ui/cmd/utils"

	logger "github.com/charmbracelet/log"
)

// flushRecorder wraps httptest.ResponseRecorder and implements http.Flusher
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {}

type mockProviderSuccess struct{}

func (m *mockProviderSuccess) SendChatCompletionRequest(params providers.RequestParams) (*providers.ChatCompletionMessage, error) {
	return nil, nil
}

func (m *mockProviderSuccess) SendChatCompletionStreamRequest(params providers.RequestParams, sc utils.StreamClient) (*providers.ChatCompletionMessage, error) {
	// simulate streaming partial reasoning and content
	_ = utils.SendStreamChunk(sc, utils.StreamChunk{Type: utils.REASONING, Payload: "partial-reasoning"})
	_ = utils.SendStreamChunk(sc, utils.StreamChunk{Type: utils.CONTENT, Payload: "partial-content"})

	// return final completion
	return &providers.ChatCompletionMessage{
		Content:   "final content",
		Reasoning: "final reasoning",
		ToolCalls: []providers.ToolCall{},
		Stats: utils.StreamStats{
			PromptTokens:     1,
			CompletionTokens: 2,
			Speed:            3,
		},
	}, nil
}

type mockProviderError struct{}

func (m *mockProviderError) SendChatCompletionRequest(params providers.RequestParams) (*providers.ChatCompletionMessage, error) {
	return nil, nil
}

func (m *mockProviderError) SendChatCompletionStreamRequest(params providers.RequestParams, sc utils.StreamClient) (*providers.ChatCompletionMessage, error) {
	_ = utils.SendStreamChunk(sc, utils.StreamChunk{Type: utils.CONTENT, Payload: "partial-content"})
	return nil, http.ErrHandlerTimeout
}

// setupTest initializes sqlite DB, logger, utils and chat package with the provided mock provider.
// It returns a teardown function that closes the DB.
func setupTest(t *testing.T, mock providers.Client) func() {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	if err := data.InitDataSource(dbPath); err != nil {
		t.Fatalf("InitDataSource error: %v", err)
	}
	// ensure DB is closed when test finishes
	teardown := func() { _ = data.DB.Close() }

	l := logger.New(os.Stdout)
	utils.Setup(l)

	// insert test user so foreign keys succeed
	_, err := data.DB.Exec("INSERT INTO Users (username, pass_hash) VALUES (?, ?)", "test-user", "x")
	if err != nil {
		// close DB before failing
		_ = data.DB.Close()
		t.Fatalf("failed insert user: %v", err)
	}

	tools.SetUpTools(l, data.DB)
	skills.SetupSkills(l, data.DB)
	SetupChat(l, data.DB, mock)
	return teardown
}

func TestChatStream_Success(t *testing.T) {
	mock := &mockProviderSuccess{}

	teardown := setupTest(t, mock)
	defer teardown()

	// build request
	reqBody := map[string]any{"conversationId": "conv-1", "parentId": 0, "model": "provider-x/model", "content": "hello"}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(b))
	// set user in context
	req = req.WithContext(context.WithValue(req.Context(), "user", "test-user"))

	rr := &flushRecorder{httptest.NewRecorder()}

	chatStream(rr, req)

	body := rr.Body.String()
	if body == "" {
		t.Fatalf("expected stream output, got empty body")
	}
	if !contains(body, "event: metadata") {
		t.Errorf("expected metadata event in body; got: %s", body)
	}
	if !contains(body, "partial-content") && !contains(body, "final content") {
		t.Errorf("expected content chunks in body; got: %s", body)
	}
	if !contains(body, "event: complete") {
		t.Errorf("expected complete event in body; got: %s", body)
	}
}

func TestChatStream_ProviderError(t *testing.T) {
	mock := &mockProviderError{}
	teardown := setupTest(t, mock)
	defer teardown()

	reqBody := map[string]any{"conversationId": "conv-err", "parentId": 0, "model": "provider-x/model", "content": "hello"}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(b))
	req = req.WithContext(context.WithValue(req.Context(), "user", "test-user"))

	rr := &flushRecorder{httptest.NewRecorder()}

	chatStream(rr, req)

	body := rr.Body.String()
	if !contains(body, "event: error") {
		t.Errorf("expected error event in body; got: %s", body)
	}
}

func contains(s, sub string) bool { return bytes.Contains([]byte(s), []byte(sub)) }

func firstSSEDataLine(body []byte) ([]byte, bool) {
	lines := bytes.Split(body, []byte("\n"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("data: ")) {
			return bytes.TrimPrefix(line, []byte("data: ")), true
		}
	}
	return nil, false
}

func TestChatStream_DBContentSaved(t *testing.T) {
	mock := &mockProviderSuccess{}
	teardown := setupTest(t, mock)
	defer teardown()

	userContent := "test user message"
	model := "provider-x/model"

	// build request
	reqBody := map[string]any{
		"conversationId": "new-conv",
		"parentId":       0,
		"model":          model,
		"content":        userContent,
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(b))
	req = req.WithContext(context.WithValue(req.Context(), "user", "test-user"))

	rr := &flushRecorder{httptest.NewRecorder()}

	chatStream(rr, req)

	// Extract conversation ID and user message ID from stream metadata
	body := rr.Body.String()
	var convID string
	var userMsgID int

	// Parse SSE stream to extract metadata
	lines := bytes.Split([]byte(body), []byte("\n"))
	for i, line := range lines {
		if bytes.HasPrefix(line, []byte("event: metadata")) {
			// Next line should be data:
			if i+1 < len(lines) && bytes.HasPrefix(lines[i+1], []byte("data: ")) {
				dataLine := bytes.TrimPrefix(lines[i+1], []byte("data: "))
				// The data is wrapped as: { "metadata": {...} }
				var wrapper struct {
					Metadata struct {
						ConversationID string `json:"conversationId"`
						UserMessageID  int    `json:"userMessageId"`
					} `json:"metadata"`
				}
				if err := json.Unmarshal(dataLine, &wrapper); err == nil {
					convID = wrapper.Metadata.ConversationID
					userMsgID = wrapper.Metadata.UserMessageID
					break
				}
			}
		}
	}

	if convID == "" || userMsgID == 0 {
		t.Fatalf("failed to extract metadata from stream response: convID=%s, userMsgID=%d", convID, userMsgID)
	}

	// Verify conversation was created using repository
	conv, err := conversations.GetByID(convID, "test-user")
	if err != nil {
		t.Fatalf("conversation not found: %v", err)
	}
	if conv.ID != convID {
		t.Errorf("expected conversation ID %s, got %s", convID, conv.ID)
	}
	if conv.UserID != "test-user" {
		t.Errorf("expected user ID 'test-user', got %s", conv.UserID)
	}

	// Get all messages for the conversation using repository
	messages := getAllConversationMessages(convID, "test-user")
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages in conversation, got %d", len(messages))
	}

	// Verify user message was saved
	userMsg, err := getMessage(userMsgID, "test-user")
	if err != nil {
		t.Fatalf("user message not found: %v", err)
	}
	if userMsg.ID != userMsgID {
		t.Errorf("expected user message ID %d, got %d", userMsgID, userMsg.ID)
	}
	if userMsg.Content != userContent {
		t.Errorf("expected user message content '%s', got '%s'", userContent, userMsg.Content)
	}
	if userMsg.Role != "user" {
		t.Errorf("expected user message role 'user', got '%s'", userMsg.Role)
	}
	if userMsg.ParentID != 0 {
		t.Errorf("expected user message parent_id 0, got %d", userMsg.ParentID)
	}
	if userMsg.ConvID != convID {
		t.Errorf("expected user message conv_id %s, got %s", convID, userMsg.ConvID)
	}

	// Find and verify assistant message
	var assistantMsg *Message
	for _, msg := range messages {
		if msg.Role == "assistant" {
			assistantMsg = msg
			break
		}
	}
	if assistantMsg == nil {
		t.Fatalf("assistant message not found in conversation")
	}

	if assistantMsg.Content != "final content" {
		t.Errorf("expected assistant message content 'final content', got '%s'", assistantMsg.Content)
	}
	if assistantMsg.Reasoning != "final reasoning" {
		t.Errorf("expected assistant message reasoning 'final reasoning', got '%s'", assistantMsg.Reasoning)
	}
	if assistantMsg.Role != "assistant" {
		t.Errorf("expected assistant message role 'assistant', got '%s'", assistantMsg.Role)
	}
	if assistantMsg.Model != model {
		t.Errorf("expected assistant message model %s, got %s", model, assistantMsg.Model)
	}
	if assistantMsg.ParentID != userMsgID {
		t.Errorf("expected assistant message parent_id %d, got %d", userMsgID, assistantMsg.ParentID)
	}
	if assistantMsg.Status != "completed" {
		t.Errorf("expected assistant message status 'completed', got '%s'", assistantMsg.Status)
	}
	if assistantMsg.ConvID != convID {
		t.Errorf("expected assistant message conv_id %s, got %s", convID, assistantMsg.ConvID)
	}

	// Verify stats were saved correctly
	if assistantMsg.TokenCount != 2 {
		t.Errorf("expected assistant message token_count 2, got %d", assistantMsg.TokenCount)
	}
	if assistantMsg.ContextSize != 1 {
		t.Errorf("expected assistant message context_size 1, got %d", assistantMsg.ContextSize)
	}
	if assistantMsg.Speed != 3 {
		t.Errorf("expected assistant message speed 3, got %f", assistantMsg.Speed)
	}

	// Verify message parent-child relationship
	if len(userMsg.Children) != 1 {
		t.Errorf("expected 1 child message for user message, got %d", len(userMsg.Children))
	} else if userMsg.Children[0] != assistantMsg.ID {
		t.Errorf("expected child message ID %d, got %d", assistantMsg.ID, userMsg.Children[0])
	}
}
func TestSync_Simple(t *testing.T) {
	teardown := setupTest(t, nil)
	defer teardown()

	userID := "test-user"
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 1. Session A starts sync SSE stream
	reqSync := httptest.NewRequest(http.MethodGet, "/conversations/sync?sessionId=session-a", nil)
	reqSync = reqSync.WithContext(context.WithValue(ctx, "user", userID))

	rrSync := &flushRecorder{httptest.NewRecorder()}

	syncDone := make(chan struct{})
	go func() {
		syncHandler(rrSync, reqSync)
		close(syncDone)
	}()

	// Give it a moment to subscribe
	time.Sleep(100 * time.Millisecond)

	// 2. Session B creates a conversation
	convBody := Conversation{Title: "Synced Conv"}
	reqBody := map[string]any{"conversation": convBody}
	b, _ := json.Marshal(reqBody)
	reqAdd := httptest.NewRequest(http.MethodPost, "/conversations/add", bytes.NewReader(b))
	reqAdd.Header.Set("X-Session-ID", "session-b")
	reqAdd = reqAdd.WithContext(context.WithValue(reqAdd.Context(), "user", userID))

	rrAdd := httptest.NewRecorder()
	saveConversation(rrAdd, reqAdd)

	if rrAdd.Code != http.StatusCreated {
		t.Fatalf("failed to create conversation: %v", rrAdd.Body.String())
	}

	// 3. Stream should finish when request context times out
	select {
	case <-syncDone:
		if rrSync.Code != http.StatusOK {
			t.Errorf("expected 200 OK for sync, got %d", rrSync.Code)
		}
		dataLine, ok := firstSSEDataLine(rrSync.Body.Bytes())
		if !ok {
			t.Fatalf("expected SSE data line in sync response, got: %s", rrSync.Body.String())
		}
		var event SyncEvent
		if err := json.Unmarshal(dataLine, &event); err != nil {
			t.Fatalf("failed to unmarshal sync event: %v", err)
		}
		if event.Type != EventConversationCreated {
			t.Errorf("expected event type %s, got %s", EventConversationCreated, event.Type)
		}
		if event.Conversation.Title != "Synced Conv" {
			t.Errorf("expected title 'Synced Conv', got '%s'", event.Conversation.Title)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for sync event")
	}
}

func TestSync_ExcludeSender(t *testing.T) {
	teardown := setupTest(t, nil)
	defer teardown()

	userID := "test-user"

	// 1. Session A starts sync SSE stream
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	reqSync := httptest.NewRequest(http.MethodGet, "/conversations/sync?sessionId=session-a", nil)
	reqSync = reqSync.WithContext(context.WithValue(ctx, "user", userID))

	rrSync := &flushRecorder{httptest.NewRecorder()}

	syncDone := make(chan struct{})
	go func() {
		syncHandler(rrSync, reqSync)
		close(syncDone)
	}()

	// Give it a moment to subscribe
	time.Sleep(100 * time.Millisecond)

	// 2. Session A (SAME SESSION) creates a conversation
	convBody := Conversation{Title: "Same Session Conv"}
	reqBody := map[string]any{"conversation": convBody}
	b, _ := json.Marshal(reqBody)
	reqAdd := httptest.NewRequest(http.MethodPost, "/conversations/add", bytes.NewReader(b))
	reqAdd.Header.Set("X-Session-ID", "session-a") // SAME SESSION ID
	reqAdd = reqAdd.WithContext(context.WithValue(reqAdd.Context(), "user", userID))

	rrAdd := httptest.NewRecorder()
	saveConversation(rrAdd, reqAdd)

	if rrAdd.Code != http.StatusCreated {
		t.Fatalf("failed to create conversation: %v", rrAdd.Body.String())
	}

	// 3. Session A should not receive an event from its own session
	select {
	case <-syncDone:
		if dataLine, ok := firstSSEDataLine(rrSync.Body.Bytes()); ok {
			t.Fatalf("expected no SSE event for same-session updates, got: %s", string(dataLine))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for sync handler to finish")
	}
}

// mockProviderWithToolCalls simulates a model that produces content, then a
// tool call, then more content.  The second streaming call must be preceded by
// a "\n" content chunk so the two pieces of text don't run together.
type mockProviderWithToolCalls struct {
	callCount int
}

func (m *mockProviderWithToolCalls) SendChatCompletionRequest(params providers.RequestParams) (*providers.ChatCompletionMessage, error) {
	return nil, nil
}

func (m *mockProviderWithToolCalls) SendChatCompletionStreamRequest(params providers.RequestParams, sc utils.StreamClient) (*providers.ChatCompletionMessage, error) {
	m.callCount++

	if m.callCount == 1 {
		// First call: stream some content, then return a tool call.
		_ = utils.SendStreamChunk(sc, utils.StreamChunk{Type: utils.CONTENT, Payload: "Before tool"})
		return &providers.ChatCompletionMessage{
			Content: "Before tool",
			ToolCalls: []providers.ToolCall{
				{
					ID:          "tc-1",
					ReferenceID: "ref-1",
					Name:        "fake_tool",
					Args:        `{}`,
				},
			},
		}, nil
	}

	// Second call (after tool execution): stream post-tool content.
	_ = utils.SendStreamChunk(sc, utils.StreamChunk{Type: utils.CONTENT, Payload: "After tool"})
	return &providers.ChatCompletionMessage{
		Content: "After tool",
		Stats:   utils.StreamStats{PromptTokens: 1, CompletionTokens: 2, Speed: 1},
	}, nil
}

// TestChatStream_NewlineSeparatorStreamedBetweenToolCalls verifies that when
// the model produces content, uses a tool, then produces more content, a "\n"
// content chunk is streamed to the client between the two content segments.
// This is a regression test for the bug where the newline was only added to the
// saved DB message but never sent over the SSE stream.
func TestChatStream_NewlineSeparatorStreamedBetweenToolCalls(t *testing.T) {
	mock := &mockProviderWithToolCalls{}
	teardown := setupTest(t, mock)
	defer teardown()

	reqBody := map[string]any{
		"conversationId": "conv-tool-nl",
		"parentId":       0,
		"model":          "provider-x/model",
		"content":        "hello",
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(b))
	req = req.WithContext(context.WithValue(req.Context(), "user", "test-user"))

	rr := &flushRecorder{httptest.NewRecorder()}
	chatStream(rr, req)

	body := rr.Body.String()

	// Collect all SSE data lines that carry content chunks, in order.
	var contentChunks []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			continue
		}
		raw, ok := m["content"]
		if !ok {
			continue
		}
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			continue
		}
		contentChunks = append(contentChunks, text)
	}

	// We expect at least: "Before tool", "\n", "After tool"
	if len(contentChunks) < 3 {
		t.Fatalf("expected at least 3 content chunks, got %d: %v", len(contentChunks), contentChunks)
	}

	// Find the index of the newline separator.
	nlIdx := -1
	for i, c := range contentChunks {
		if c == "\n" {
			nlIdx = i
			break
		}
	}

	if nlIdx == -1 {
		t.Fatalf("expected a '\\n' content chunk between tool calls in stream, got chunks: %v", contentChunks)
	}

	// The "\n" must appear after "Before tool" and before "After tool".
	beforeFound := false
	for i := 0; i < nlIdx; i++ {
		if contentChunks[i] == "Before tool" {
			beforeFound = true
		}
	}
	afterFound := false
	for i := nlIdx + 1; i < len(contentChunks); i++ {
		if contentChunks[i] == "After tool" {
			afterFound = true
		}
	}

	if !beforeFound {
		t.Errorf("expected 'Before tool' content chunk before the '\\n' separator, got chunks: %v", contentChunks)
	}
	if !afterFound {
		t.Errorf("expected 'After tool' content chunk after the '\\n' separator, got chunks: %v", contentChunks)
	}
}

// mockProviderCancelMidToolcall simulates the user hitting stop while the model
// is mid-toolcall: first stream returns content + (would-be) tool calls after
// cancelling the generation, and must never be followed by a second stream.
type mockProviderCancelMidToolcall struct {
	calls        int
	secondCalled bool
}

func (m *mockProviderCancelMidToolcall) SendChatCompletionRequest(params providers.RequestParams) (*providers.ChatCompletionMessage, error) {
	return nil, nil
}

func (m *mockProviderCancelMidToolcall) SendChatCompletionStreamRequest(params providers.RequestParams, sc utils.StreamClient) (*providers.ChatCompletionMessage, error) {
	m.calls++
	if m.calls == 1 {
		_ = utils.SendStreamChunk(sc, utils.StreamChunk{Type: utils.CONTENT, Payload: "partial before cancel"})
		// Simulate user stop mid-toolcall. Real provider path also drops tool
		// calls on cancel; here we still return them to ensure chat/agent loop
		// refuses to continue after IsGenerationCancelled.
		_ = providers.CancelStream(params.MessageID, params.User)
		return &providers.ChatCompletionMessage{
			Content: "partial before cancel",
			ToolCalls: []providers.ToolCall{
				{
					ID:          "tc-partial",
					ReferenceID: "ref-partial",
					Name:        "should_not_run",
					Args:        `{"q":`, // incomplete JSON as mid-stream would look
				},
			},
			// Cancelled may be set by the real stream path; leave false so the
			// generation-context check is what protects the agent loop.
		}, nil
	}

	m.secondCalled = true
	_ = utils.SendStreamChunk(sc, utils.StreamChunk{Type: utils.CONTENT, Payload: "AFTER CANCEL LEAK"})
	return &providers.ChatCompletionMessage{
		Content: "AFTER CANCEL LEAK",
	}, nil
}

func TestChatStream_CancelMidToolcallDoesNotContinueAgentLoop(t *testing.T) {
	mock := &mockProviderCancelMidToolcall{}
	teardown := setupTest(t, mock)
	defer teardown()

	reqBody := map[string]any{
		"conversationId": "conv-cancel-mid-tc",
		"parentId":       0,
		"model":          "provider-x/model",
		"content":        "hello",
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(b))
	req = req.WithContext(context.WithValue(req.Context(), "user", "test-user"))

	rr := &flushRecorder{httptest.NewRecorder()}
	chatStream(rr, req)

	body := rr.Body.String()
	if mock.secondCalled || mock.calls > 1 {
		t.Fatalf("expected no follow-up stream after cancel mid-toolcall, calls=%d secondCalled=%v body=%s",
			mock.calls, mock.secondCalled, body)
	}
	if contains(body, "AFTER CANCEL LEAK") {
		t.Fatalf("follow-up content leaked after cancel: %s", body)
	}
	if !contains(body, "partial before cancel") {
		t.Fatalf("expected partial content to be kept after cancel; body=%s", body)
	}
	if !contains(body, "event: complete") {
		t.Fatalf("expected complete event after cancel; body=%s", body)
	}

	// Assistant message should be completed with partial content only.
	rows, err := data.DB.Query(`SELECT content, status FROM Messages WHERE role = 'assistant'`)
	if err != nil {
		t.Fatalf("query messages: %v", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var content, status string
		if err := rows.Scan(&content, &status); err != nil {
			t.Fatalf("scan: %v", err)
		}
		found = true
		if status != "completed" {
			t.Errorf("expected assistant status completed, got %q", status)
		}
		if content != "partial before cancel" {
			t.Errorf("expected partial content only, got %q", content)
		}
		if strings.Contains(content, "AFTER CANCEL LEAK") {
			t.Errorf("cancelled response should not include follow-up content: %q", content)
		}
	}
	if !found {
		t.Fatal("expected an assistant message in DB")
	}

	// Incomplete tool call must not have been persisted.
	var toolCount int
	if err := data.DB.QueryRow(`SELECT COUNT(*) FROM ToolCalls`).Scan(&toolCount); err != nil {
		t.Fatalf("count tool calls: %v", err)
	}
	if toolCount != 0 {
		t.Fatalf("expected 0 tool calls saved after cancel mid-toolcall, got %d", toolCount)
	}
}

// mockProviderCancelFlagged returns Cancelled=true with tool calls (as the
// real stream path does) and records whether a second stream was requested.
type mockProviderCancelFlagged struct {
	calls int
}

func (m *mockProviderCancelFlagged) SendChatCompletionRequest(params providers.RequestParams) (*providers.ChatCompletionMessage, error) {
	return nil, nil
}

func (m *mockProviderCancelFlagged) SendChatCompletionStreamRequest(params providers.RequestParams, sc utils.StreamClient) (*providers.ChatCompletionMessage, error) {
	m.calls++
	if m.calls == 1 {
		_ = utils.SendStreamChunk(sc, utils.StreamChunk{Type: utils.CONTENT, Payload: "hi"})
		return &providers.ChatCompletionMessage{
			Content: "hi",
			ToolCalls: []providers.ToolCall{
				{ID: "tc-1", ReferenceID: "ref-1", Name: "noop", Args: `{}`},
			},
			Cancelled: true,
		}, nil
	}
	return &providers.ChatCompletionMessage{Content: "should-not-run"}, nil
}

func TestChatStream_CancelledFlagSkipsAgentLoop(t *testing.T) {
	mock := &mockProviderCancelFlagged{}
	teardown := setupTest(t, mock)
	defer teardown()

	reqBody := map[string]any{
		"conversationId": "conv-cancel-flag",
		"parentId":       0,
		"model":          "provider-x/model",
		"content":        "hello",
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(b))
	req = req.WithContext(context.WithValue(req.Context(), "user", "test-user"))

	rr := &flushRecorder{httptest.NewRecorder()}
	chatStream(rr, req)

	if mock.calls != 1 {
		t.Fatalf("expected exactly 1 provider stream when Cancelled=true, got %d", mock.calls)
	}
}

// mockProviderCountsStreams is used to verify cancel during a tool-approval wait
// never starts the post-tool follow-up stream.
type mockProviderCountsStreams struct {
	calls int
}

func (m *mockProviderCountsStreams) SendChatCompletionRequest(params providers.RequestParams) (*providers.ChatCompletionMessage, error) {
	return nil, nil
}

func (m *mockProviderCountsStreams) SendChatCompletionStreamRequest(params providers.RequestParams, sc utils.StreamClient) (*providers.ChatCompletionMessage, error) {
	m.calls++
	if m.calls == 1 {
		_ = utils.SendStreamChunk(sc, utils.StreamChunk{Type: utils.CONTENT, Payload: "need tool"})
		return &providers.ChatCompletionMessage{
			Content: "need tool",
			ToolCalls: []providers.ToolCall{
				{
					ID:          "tc-approve",
					ReferenceID: "ref-approve",
					Name:        "needs_approval_tool",
					Args:        `{}`,
				},
			},
		}, nil
	}
	_ = utils.SendStreamChunk(sc, utils.StreamChunk{Type: utils.CONTENT, Payload: "AFTER APPROVAL"})
	return &providers.ChatCompletionMessage{Content: "AFTER APPROVAL"}, nil
}

func TestChatStream_CancelDuringToolApprovalDoesNotFollowUp(t *testing.T) {
	mock := &mockProviderCountsStreams{}
	teardown := setupTest(t, mock)
	defer teardown()

	// Install a tool that blocks on approval so cancel can race the wait.
	_, err := data.DB.Exec(
		`INSERT INTO MCPServers (id, name, endpoint, api_key, user) VALUES (?, ?, ?, ?, ?)`,
		"default-test", "default", "http://localhost", "", "test-user",
	)
	if err != nil {
		t.Fatalf("insert mcp server: %v", err)
	}
	_, err = data.DB.Exec(
		`INSERT INTO Tools (id, mcp_server_id, name, description, input_schema, require_approval, is_enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"tool-approve", "default-test", "needs_approval_tool", "blocks", `{"type":"object"}`, 1, 1,
	)
	if err != nil {
		t.Fatalf("insert tool: %v", err)
	}

	done := make(chan struct{})
	var body string
	go func() {
		defer close(done)
		reqBody := map[string]any{
			"conversationId": "conv-cancel-approval",
			"parentId":       0,
			"model":          "provider-x/model",
			"content":        "hello",
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/chat/stream", bytes.NewReader(b))
		req = req.WithContext(context.WithValue(req.Context(), "user", "test-user"))
		rr := &flushRecorder{httptest.NewRecorder()}
		chatStream(rr, req)
		body = rr.Body.String()
	}()

	// Wait until the assistant message exists and generation is registered, then cancel.
	var messageID int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = data.DB.QueryRow(
			`SELECT id FROM Messages WHERE role = 'assistant' AND status = 'pending' ORDER BY id DESC LIMIT 1`,
		).Scan(&messageID)
		if messageID > 0 && providers.CancelStream(messageID, "test-user") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if messageID <= 0 {
		t.Fatal("timed out waiting for pending assistant message to cancel")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("chatStream did not finish after cancel during tool approval")
	}

	if mock.calls != 1 {
		t.Fatalf("expected no follow-up stream during approval cancel, provider calls=%d body=%s", mock.calls, body)
	}
	if contains(body, "AFTER APPROVAL") {
		t.Fatalf("follow-up content leaked after cancel during approval: %s", body)
	}

	var toolCount int
	if err := data.DB.QueryRow(`SELECT COUNT(*) FROM ToolCalls`).Scan(&toolCount); err != nil {
		t.Fatalf("count tool calls: %v", err)
	}
	if toolCount != 0 {
		t.Fatalf("expected no tool calls saved when cancelled during approval, got %d", toolCount)
	}
}

func TestCancelStream_EndpointForceCompletes(t *testing.T) {
	teardown := setupTest(t, &mockProviderSuccess{})
	defer teardown()

	// Pending assistant message with an active generation.
	_, err := data.DB.Exec(
		`INSERT INTO Conversations (id, user, title) VALUES (?, ?, ?)`,
		"conv-cancel-ep", "test-user", "t",
	)
	if err != nil {
		t.Fatalf("insert conv: %v", err)
	}
	res, err := data.DB.Exec(
		`INSERT INTO Messages (conv_id, role, model, parent_id, content, reasoning, error, status, speed, token_count, context_size, created_at, updated_at)
		 VALUES (?, 'assistant', 'm', 0, '', '', '', 'pending', 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"conv-cancel-ep",
	)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
	id64, _ := res.LastInsertId()
	messageID := int(id64)

	ctx := providers.StartGeneration(messageID, "test-user")
	defer providers.EndGeneration(messageID)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/chat/cancel?messageId=%d", messageID), nil)
	req = req.WithContext(context.WithValue(req.Context(), "user", "test-user"))
	rr := httptest.NewRecorder()
	cancelStream(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if ctx.Err() == nil {
		t.Fatal("expected generation context cancelled by endpoint")
	}

	var status string
	if err := data.DB.QueryRow(`SELECT status FROM Messages WHERE id = ?`, messageID).Scan(&status); err != nil {
		t.Fatalf("status query: %v", err)
	}
	if status != "completed" {
		t.Fatalf("expected force-completed status, got %q", status)
	}
}

func TestEnterAgentLoop_StopsWhenGenerationCancelled(t *testing.T) {
	mock := &mockProviderWithToolCalls{}
	teardown := setupTest(t, mock)
	defer teardown()

	// Seed conversation + assistant message.
	_, err := data.DB.Exec(
		`INSERT INTO Conversations (id, user, title) VALUES (?, ?, ?)`,
		"conv-loop-cancel", "test-user", "t",
	)
	if err != nil {
		t.Fatalf("insert conv: %v", err)
	}
	res, err := data.DB.Exec(
		`INSERT INTO Messages (conv_id, role, model, parent_id, content, reasoning, error, status, speed, token_count, context_size, created_at, updated_at)
		 VALUES (?, 'assistant', 'm', 0, 'before', '', '', 'pending', 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"conv-loop-cancel",
	)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
	id64, _ := res.LastInsertId()
	msgID := int(id64)

	genCtx := providers.StartGeneration(msgID, "test-user")
	providers.CancelStream(msgID, "test-user")
	defer providers.EndGeneration(msgID)

	rr := &flushRecorder{httptest.NewRecorder()}
	sc := utils.StreamClient{User: "test-user", MessageID: msgID, Writer: rr}
	msg := &Message{ID: msgID, Content: "before", ConvID: "conv-loop-cancel"}
	params := providers.RequestParams{
		Messages:  nil,
		Model:     "provider-x/model",
		User:      "test-user",
		MessageID: msgID,
		Context:   genCtx,
	}

	beforeCalls := mock.callCount
	completion, err := enterAgentLoop(
		[]providers.ToolCall{{ID: "tc", ReferenceID: "r", Name: "fake_tool", Args: `{}`}},
		params,
		msg,
		"conv-loop-cancel",
		"test-user",
		sc,
	)
	if err != nil {
		t.Fatalf("enterAgentLoop error: %v", err)
	}
	if completion == nil || !completion.Cancelled {
		t.Fatalf("expected cancelled completion, got %#v", completion)
	}
	if mock.callCount != beforeCalls {
		t.Fatalf("expected no provider calls after cancel, callCount=%d", mock.callCount)
	}
	if msg.Content != "before" {
		t.Fatalf("content should stay unchanged, got %q", msg.Content)
	}
	if genCtx.Err() == nil {
		t.Fatal("expected generation context cancelled")
	}
}
