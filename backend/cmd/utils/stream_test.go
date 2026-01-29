package utils

import (
	"testing"
	"time"
)

func TestAppendAndGetChunks(t *testing.T) {
	user := "testuser"
	id := 42
	StreamCache.Delete(user, id)

	StreamCache.AppendChunk(user, id, StreamChunk{Type: EVENT_METADATA, Payload: "meta"})
	StreamCache.AppendChunk(user, id, StreamChunk{Type: CONTENT, Payload: "hello"})

	chunks, found := StreamCache.GetChunks(user, id)
	if !found {
		t.Fatal("expected chunks found")
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks got %d", len(chunks))
	}
	if chunks[0].Type != EVENT_METADATA || chunks[1].Type != CONTENT {
		t.Fatalf("unexpected chunk types: %v", chunks)
	}

	StreamCache.Delete(user, id)
}

func TestSubscribeReplayAndLive(t *testing.T) {
	user := "subuser"
	id := 99
	StreamCache.Delete(user, id)

	StreamCache.AppendChunk(user, id, StreamChunk{Type: EVENT_METADATA, Payload: "m"})
	ch, cancel := StreamCache.Subscribe(user, id)
	defer cancel()

	select {
	case c, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		if c.Type != EVENT_METADATA {
			t.Fatalf("expected metadata, got %v", c.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial chunk")
	}

	StreamCache.AppendChunk(user, id, StreamChunk{Type: CONTENT, Payload: "live"})
	select {
	case c, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		if c.Type != CONTENT {
			t.Fatalf("expected content, got %v", c.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for live chunk")
	}

	StreamCache.AppendChunk(user, id, StreamChunk{Type: EVENT_COMPLETE, Payload: StreamComplete{UserMessageID: 1, AssistantMessageID: 2}})

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel closed after complete")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close")
	}

	StreamCache.Delete(user, id)
}

func TestSubscribeAfterDoneReplaysAll(t *testing.T) {
	user := "doneuser"
	id := 123
	StreamCache.Delete(user, id)

	StreamCache.AppendChunk(user, id, StreamChunk{Type: EVENT_METADATA, Payload: "m"})
	StreamCache.AppendChunk(user, id, StreamChunk{Type: CONTENT, Payload: "done"})
	StreamCache.AppendChunk(user, id, StreamChunk{Type: EVENT_COMPLETE, Payload: StreamComplete{UserMessageID: 1, AssistantMessageID: 2}})

	ch, cancel := StreamCache.Subscribe(user, id)
	defer cancel()

	received := []StreamChunk{}
	for {
		c, ok := <-ch
		if !ok {
			break
		}
		received = append(received, c)
	}

	if len(received) != 3 {
		t.Fatalf("expected 3 chunks replayed, got %d", len(received))
	}

	StreamCache.Delete(user, id)
}
