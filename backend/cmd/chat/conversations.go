package chat

import (
	"ai-client/cmd/data"
	"ai-client/cmd/utils"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Conversation struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Title     string    `json:"title,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func saveConversation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Conv Conversation `json:"conversation"`
	}
	err := utils.ExtractJSONBody(r, &req)
	if err != nil {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	conv := &Conversation{
		ID:        uuid.NewString(),
		UserID:    utils.ExtractContextUser(r),
		Title:     req.Conv.Title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// debug
	log.Debug("Adding conversation", "conversation", conv)

	err = conversations.Save(conv)
	if err != nil {
		log.Error("Error adding conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error adding conversation: %v", err), http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, conv, http.StatusCreated)

	// Broadcast the new conversation to other sessions
	sessionID := r.Header.Get("X-Session-ID")
	syncManager.Broadcast(conv.UserID, sessionID, ConversationEvent{
		Type:           EventConversationCreated,
		ConversationID: conv.ID,
		Conversation:   conv,
	})
}

func getConversation(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	convId := r.PathValue("id")
	conv, err := conversations.GetByID(convId, user)
	if err != nil {
		log.Error("Error retrieving conversation", "err", err)
		http.Error(w, "Error retrieving conversation", http.StatusNotFound)
		return
	}
	utils.RespondWithJSON(w, &conv, http.StatusOK)
}

func getAllConversations(writer http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	utils.RespondWithJSON(
		writer,
		conversations.GetAll(user),
		http.StatusOK,
	)
}

func deleteConversation(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	convId := r.PathValue("id")
	err := conversations.DeleteByID(convId, user)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error deleting conversation: %v", err), http.StatusInternalServerError)
		return
	}

	// Broadcast the deletion to other sessions
	sessionID := r.Header.Get("X-Session-ID")
	syncManager.Broadcast(user, sessionID, ConversationEvent{
		Type:           EventConversationDeleted,
		ConversationID: convId,
	})

	w.WriteHeader(http.StatusNoContent)
}

func renameConversation(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	convId := r.PathValue("id")
	var req struct {
		Title string `json:"title"`
	}
	err := utils.ExtractJSONBody(r, &req)
	if err != nil {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	conv, err := conversations.GetByID(convId, user)
	if err != nil {
		log.Error("Error retrieving conversation", "err", err)
		http.Error(w, "Error retrieving conversation", http.StatusNotFound)
		return
	}

	conv.Title = req.Title

	err = conversations.Update(conv)
	if err != nil {
		log.Error("Error updating conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error updating conversation: %v", err), http.StatusInternalServerError)
		return
	}

	// Broadcast the update to other sessions
	sessionID := r.Header.Get("X-Session-ID")
	syncManager.Broadcast(user, sessionID, ConversationEvent{
		Type:           EventConversationUpdated,
		ConversationID: convId,
		Conversation:   conv,
	})

	utils.RespondWithJSON(w, &conv, http.StatusOK)
}

func getConversationMessages(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	convId := r.PathValue("id")
	messages := getAllConversationMessages(convId, user)
	utils.RespondWithJSON(w, &messages, http.StatusOK)
}

type ConversationStats struct {
	TotalTokens        int64 `json:"totalTokens"`
	TotalInputTokens   int64 `json:"totalInputTokens"`
	TotalConversations int64 `json:"totalConversations"`
	TotalMessages      int64 `json:"totalMessages"`
}

func getStats(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)

	query := `
		SELECT
			COUNT(DISTINCT c.id)            AS total_conversations,
			COUNT(m.id)                     AS total_messages,
			COALESCE(SUM(m.token_count),0)  AS total_tokens,
			COALESCE(SUM(m.context_size),0) AS total_input_tokens
		FROM Conversations c
		LEFT JOIN Messages m ON m.conv_id = c.id AND m.role = 'assistant'
		WHERE c.user = ?
	`

	var stats ConversationStats
	err := data.DB.QueryRow(query, user).Scan(
		&stats.TotalConversations,
		&stats.TotalMessages,
		&stats.TotalTokens,
		&stats.TotalInputTokens,
	)
	if err != nil {
		log.Error("Error querying stats", "err", err)
		http.Error(w, "Error querying stats", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, &stats, http.StatusOK)
}
