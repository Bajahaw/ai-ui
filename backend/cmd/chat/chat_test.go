package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"ai-client/cmd/data"
	"ai-client/cmd/providers"
	"ai-client/cmd/tools"
	"ai-client/cmd/utils"

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
		ToolCalls: []tools.ToolCall{},
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
	userMsg, err := getMessage(userMsgID)
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

	// 1. Session A starts long polling
	reqSync := httptest.NewRequest(http.MethodGet, "/conversations/sync", nil)
	reqSync.Header.Set("X-Session-ID", "session-a")
	reqSync = reqSync.WithContext(context.WithValue(reqSync.Context(), "user", userID))

	rrSync := httptest.NewRecorder()

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

	// 3. Session A should receive the event
	select {
	case <-syncDone:
		if rrSync.Code != http.StatusOK {
			t.Errorf("expected 200 OK for sync, got %d", rrSync.Code)
		}
		var event ConversationEvent
		if err := json.Unmarshal(rrSync.Body.Bytes(), &event); err != nil {
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

	// 1. Session A starts long polling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reqSync := httptest.NewRequest(http.MethodGet, "/conversations/sync", nil)
	reqSync.Header.Set("X-Session-ID", "session-a")
	reqSync = reqSync.WithContext(context.WithValue(ctx, "user", userID))

	rrSync := httptest.NewRecorder()

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

	// 3. Session A should NOT receive the event immediately
	select {
	case <-syncDone:
		t.Fatal("sync should NOT have finished since sender is excluded")
	case <-time.After(500 * time.Millisecond):
		// Success: it didn't finish prematurely
		cancel() // Now cancel to cleanup
		<-syncDone
	}
}
