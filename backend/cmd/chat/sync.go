package chat

import (
	"ai-client/cmd/utils"
	"net/http"
	"sync"
	"time"
)

// Event types
const (
	EventConversationCreated = "conversation_created"
	EventConversationUpdated = "conversation_updated"
	EventConversationDeleted = "conversation_deleted"
)

type ConversationEvent struct {
	Type           string        `json:"type"`
	ConversationID string        `json:"conversationId"`
	Conversation   *Conversation `json:"conversation,omitempty"`
}

type Subscriber struct {
	UserID    string
	SessionID string
	Events    chan ConversationEvent
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
		Events:    make(chan ConversationEvent, 10), // Buffer slightly to avoid blocking
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

func (sm *SyncManager) Broadcast(userID, sourceSessionID string, event ConversationEvent) {
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

	// Session ID is mandatory to distinguish tabs
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	sub := syncManager.Subscribe(userID, sessionID)
	// We don't defer Unsubscribe here because we want to unsubscribe only on return
	defer syncManager.Unsubscribe(userID, sessionID)

	// Set a timeout for the long poll
	// Cloudflare/Proxies usually timeout at 60-100s. We use 45s to be safe.
	timeout := time.NewTimer(45 * time.Second)
	defer timeout.Stop()

	select {
	case event := <-sub.Events:
		utils.RespondWithJSON(w, event, http.StatusOK)
	case <-timeout.C:
		// 204 is good for "no updates"
		w.WriteHeader(http.StatusNoContent)
	case <-r.Context().Done():
		// Client disconnected
		return
	case <-sub.Done:
		// Subscription cancelled (e.g. replaced)
		return
	}
}
