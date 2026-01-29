package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
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

// Stream cache stores temporary stream data for users to be able to resume streams
// when user hit api/chat/resume the stream will be replayed from the cache,
// if the cache is not complete the stream should wait until the original stream is complete
var StreamCache = NewStreamCache()

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

	// Store in cache
	StreamCache.AppendChunk(client.User, client.MessageID, chunk)

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

// ReplayChunk writes a chunk to the provided ResponseWriter without
// modifying the cache. Use this when replaying cached streams.
func ReplayChunk(sc StreamClient, chunk StreamChunk) error {
	return streamChunk(sc.Writer, chunk)
}

type StreamEntry struct {
	mu        sync.RWMutex
	Chunks    []StreamChunk
	IsDone    bool
	Listeners []chan StreamChunk
	Done      chan struct{} // Closed when stream is finished
}

type StreamStore struct {
	mu    sync.RWMutex
	Users map[string]map[int]*StreamEntry
}

func NewStreamCache() *StreamStore {
	return &StreamStore{
		Users: make(map[string]map[int]*StreamEntry),
	}
}

func (e *StreamEntry) appendChunk(chunk StreamChunk) {
	e.mu.Lock()
	// append chunk
	e.Chunks = append(e.Chunks, chunk)

	// notify listeners (non-blocking). For EVENT_COMPLETE we avoid
	// enqueueing the complete event into listener channels to prevent
	// buffered delivery that would require an extra read before the
	// channel close is observed by clients.
	listeners := append([]chan StreamChunk(nil), e.Listeners...)
	if chunk.Type != EVENT_COMPLETE {
		for _, l := range listeners {
			select {
			case l <- chunk:
			default:
			}
		}
	}

	// if complete, mark done and close channels
	if chunk.Type == EVENT_COMPLETE {
		if !e.IsDone {
			e.IsDone = true
			close(e.Done)
			// complete: close listener channels (don't enqueue the event)
			for _, l := range listeners {
				// safe to close; listeners should handle closed channel
				close(l)
			}
			e.Listeners = nil
		}
	}
	e.mu.Unlock()
}

// addListener registers a listener and returns the channel and a cancel func
func (e *StreamEntry) addListener() (<-chan StreamChunk, func()) {
	ch := make(chan StreamChunk, 8)

	e.mu.Lock()
	// copy existing chunks to replay
	existing := append([]StreamChunk(nil), e.Chunks...)
	if e.IsDone {
		// already finished: replay existing and close
		e.mu.Unlock()
		go func() {
			for _, c := range existing {
				ch <- c
			}
			close(ch)
		}()
		return ch, func() {}
	}

	e.Listeners = append(e.Listeners, ch)
	e.mu.Unlock()

	// replay existing chunks asynchronously. Use a separate goroutine per
	// chunk and recover from panics to avoid races where the channel may
	// be closed concurrently (avoid test flakes / double-close panics).
	go func() {
		for _, c := range existing {
			cc := c
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// swallow panic caused by send on closed channel
					}
				}()
				ch <- cc
			}()
		}
	}()

	cancel := func() {
		e.mu.Lock()
		removed := false
		for i, l := range e.Listeners {
			if l == ch {
				e.Listeners = append(e.Listeners[:i], e.Listeners[i+1:]...)
				removed = true
				break
			}
		}
		// only close if we actually removed the listener; if the stream
		// completed it may have already closed the channel.
		if removed {
			close(ch)
		}
		e.mu.Unlock()
	}

	return ch, cancel
}

func (c *StreamStore) GetOrCreate(userID string, streamID int) *StreamEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.Users[userID]; !ok {
		c.Users[userID] = make(map[int]*StreamEntry)
	}
	e, ok := c.Users[userID][streamID]
	if !ok {
		e = &StreamEntry{Chunks: make([]StreamChunk, 0), Done: make(chan struct{})}
		c.Users[userID][streamID] = e
	}
	return e
}

func (c *StreamStore) AppendChunk(userID string, streamID int, chunk StreamChunk) {
	e := c.GetOrCreate(userID, streamID)
	e.appendChunk(chunk)
}

func (c *StreamStore) Subscribe(userID string, streamID int) (<-chan StreamChunk, func()) {
	e := c.GetOrCreate(userID, streamID)
	return e.addListener()
}

func (c *StreamStore) GetChunks(userID string, streamID int) ([]StreamChunk, bool) {
	c.mu.RLock()
	userMap, ok := c.Users[userID]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	e, ok := userMap[streamID]
	if !ok {
		return nil, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]StreamChunk(nil), e.Chunks...), true
}

// Delete removes a stream entry from the cache
func (c *StreamStore) Delete(userID string, streamID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if m, ok := c.Users[userID]; ok {
		delete(m, streamID)
		if len(m) == 0 {
			delete(c.Users, userID)
		}
	}
}
