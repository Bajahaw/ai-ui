package providers

import (
	"context"
	"sync"
)

// generation tracks a full assistant response lifecycle: provider streams,
// tool execution, and follow-up streams after tool calls.
type generation struct {
	ctx    context.Context
	cancel context.CancelFunc
	userID string
}

var (
	generations   = make(map[int]*generation)
	generationsMu sync.Mutex
)

// StartGeneration registers a cancellable generation for an assistant message.
// CancelStream cancels this context for the entire response, including tool
// execution gaps between provider streams. Call EndGeneration when finished.
func StartGeneration(messageID int, userID string) context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	generationsMu.Lock()
	if prev, ok := generations[messageID]; ok {
		prev.cancel()
	}
	generations[messageID] = &generation{
		ctx:    ctx,
		cancel: cancel,
		userID: userID,
	}
	generationsMu.Unlock()

	return ctx
}

// EndGeneration unregisters a generation. Safe to call multiple times.
func EndGeneration(messageID int) {
	generationsMu.Lock()
	if gen, ok := generations[messageID]; ok {
		gen.cancel()
		delete(generations, messageID)
	}
	generationsMu.Unlock()
}

// CancelStream cancels an in-flight generation for the given user.
// Returns true if a matching generation was found and cancelled.
func CancelStream(messageID int, userID string) bool {
	generationsMu.Lock()
	gen, exists := generations[messageID]
	generationsMu.Unlock()

	if !exists || gen.userID != userID {
		return false
	}
	gen.cancel()
	return true
}

// IsGenerationCancelled reports whether the generation was cancelled.
// Returns false if no generation is registered for messageID.
func IsGenerationCancelled(messageID int) bool {
	generationsMu.Lock()
	gen, ok := generations[messageID]
	generationsMu.Unlock()
	if !ok {
		return false
	}
	return gen.ctx.Err() != nil
}
