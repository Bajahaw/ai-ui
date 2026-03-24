package chat

import (
	"ai-client/cmd/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Event types
const (
	EventConversationCreated = "conversation_created"
	EventConversationUpdated = "conversation_updated"
	EventConversationDeleted = "conversation_deleted"
	EventMessageSaved        = "message_saved"
	EventMessageUpdated      = "message_updated"
)

type SyncEvent struct {
	Type           string        `json:"type"`
	ConversationID string        `json:"conversationId"`
	Conversation   *Conversation `json:"conversation,omitempty"`
	MessageID      int           `json:"messageId,omitempty"`
	Message        *Message      `json:"message,omitempty"`
}

type Subscriber struct {
	UserID    string
	SessionID string
	Events    chan SyncEvent
	Done      chan struct{}
}

type SyncManager struct {
	subscribers map[string]map[string]*Subscriber // userId -> sessionId -> subscriber
	mu          sync.RWMutex
}

var syncManager = &SyncManager{
	subscribers: make(map[string]map[string]*Subscriber),
}

func (sm *SyncManager) Subscribe(userID, sessionID string) *Subscriber {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.subscribers[userID]; !ok {
		sm.subscribers[userID] = make(map[string]*Subscriber)
	}

	// If a subscription already exists for this session, close it and replace it
	// This handles cases where a previous long-poll request was interrupted but not cleaned up
	if sub, ok := sm.subscribers[userID][sessionID]; ok {
		close(sub.Done)
		delete(sm.subscribers[userID], sessionID)
	}

	sub := &Subscriber{
		UserID:    userID,
		SessionID: sessionID,
		Events:    make(chan SyncEvent, 10), // Buffer slightly to avoid blocking
		Done:      make(chan struct{}),
	}

	sm.subscribers[userID][sessionID] = sub
	return sub
}

func (sm *SyncManager) Unsubscribe(userID, sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if userSubs, ok := sm.subscribers[userID]; ok {
		if sub, ok := userSubs[sessionID]; ok {
			close(sub.Done)
			delete(userSubs, sessionID)
		}
		if len(userSubs) == 0 {
			delete(sm.subscribers, userID)
		}
	}
}

func (sm *SyncManager) Broadcast(userID, sourceSessionID string, event SyncEvent) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	userSubs, ok := sm.subscribers[userID]
	if !ok {
		return
	}

	for sessionID, sub := range userSubs {
		if sessionID != sourceSessionID {
			select {
			case sub.Events <- event:
			default:
				// If channel is full, we skip. Client should refresh on reconnect or timeout.
				log.Warn("Subscriber channel full, skipping event", "userId", userID, "sessionId", sessionID)
			}
		}
	}
}

func syncHandler(w http.ResponseWriter, r *http.Request) {
	userID := utils.ExtractContextUser(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Session ID comes from query param — EventSource cannot send custom headers
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Require flusher support — same pattern used by the rest of the streaming code
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// SSE headers — must be set before the first Write
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	sub := syncManager.Subscribe(userID, sessionID)
	defer syncManager.Unsubscribe(userID, sessionID)

	// Send a heartbeat every 30 s to keep proxies and load balancers alive
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case event := <-sub.Events:
			data, err := json.Marshal(event)
			if err != nil {
				log.Warn("Failed to marshal sync event", "err", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			// Client disconnected
			return

		case <-sub.Done:
			// Subscription replaced (same session reconnected); client will auto-reconnect
			return
		}
	}
}
