package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
