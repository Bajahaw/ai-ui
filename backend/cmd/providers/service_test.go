package providers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/utils"
)

func TestConversationIDSentAsSessionIDHeader(t *testing.T) {
	setupProvidersTest(t)

	var mu sync.Mutex
	var requestHeaders []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestHeaders = append(requestHeaders, r.Header.Clone())
		mu.Unlock()

		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(string(body), `"stream":true`) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":null}]}\n\n")
			_, _ = io.WriteString(w, "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		_, _ = io.WriteString(w, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"m1","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer srv.Close()

	if err := providers.Save(&Provider{
		ID:      "session-provider",
		BaseURL: srv.URL,
		APIKey:  "test-key",
		User:    "alice",
	}); err != nil {
		t.Fatal(err)
	}

	params := RequestParams{
		Messages:       []SimpleMessage{{Role: "user", Content: "hello"}},
		Model:          "session-provider/m1",
		User:           "alice",
		ConversationID: "conv-123",
	}
	client := &ClientImpl{}
	if _, err := client.SendChatCompletionRequest(params); err != nil {
		t.Fatalf("non-streaming request: %v", err)
	}
	if _, err := client.SendChatCompletionStreamRequest(params, utils.StreamClient{Writer: httptest.NewRecorder()}); err != nil {
		t.Fatalf("streaming request: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requestHeaders) != 2 {
		t.Fatalf("expected 2 upstream requests, got %d", len(requestHeaders))
	}
	for i, headers := range requestHeaders {
		if got := headers.Get("session-id"); got != "conv-123" {
			t.Errorf("request %d session-id: got %q, want %q", i+1, got, "conv-123")
		}
	}
}
