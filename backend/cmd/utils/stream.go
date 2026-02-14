package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	EVENT_METADATA = "metadata"
	EVENT_ERROR    = "error"
	EVENT_CHUNK    = "chunk"
	EVENT_COMPLETE = "complete"
	TOOL_CALL      = "tool_call"
	CONTENT        = "content"
	REASONING      = "reasoning"
)

type StreamClient struct {
	User      string
	MessageID int
	Writer    http.ResponseWriter
}

type StreamChunk struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type StreamMetadata struct {
	ConversationID string `json:"conversationId"`
	UserMessageID  int    `json:"userMessageId"`
}

// StreamComplete sent when stream is complete
type StreamComplete struct {
	UserMessageID      int         `json:"userMessageId"`
	AssistantMessageID int         `json:"assistantMessageId"`
	StreamStats        StreamStats `json:"streamStats"`
}

type StreamStats struct {
	// PromptTokens or Context Size
	PromptTokens int
	// CompletionTokens or Response message size
	CompletionTokens int
	// // TotalTokens = context + response
	// TotalTokens int
	// Tokens per second
	Speed float64
}

func AddStreamHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}

func SendStreamChunk(client StreamClient, chunk StreamChunk) error {
	err := streamChunk(client.Writer, chunk)
	// Stream cache removed
	return err
}

func streamChunk(w http.ResponseWriter, chunk StreamChunk) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	payload, err := json.Marshal(chunk.Payload)
	if err != nil {
		return err
	}

	if chunk.Type == EVENT_ERROR || chunk.Type == EVENT_METADATA || chunk.Type == EVENT_COMPLETE {
		fmt.Fprintf(w, "event: %s\ndata: { \"%s\": %s }\n\n", chunk.Type, chunk.Type, payload)
		flusher.Flush()
		return nil
	}

	fmt.Fprintf(w, "data: { \"%s\": %s }\n\n", chunk.Type, payload)
	flusher.Flush()
	return nil
}
