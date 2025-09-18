package chat

import (
	"ai-client/cmd/utils"
	"fmt"
	"net/http"
	"time"
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

	conv := &req.Conv

	// debug
	log.Debug("Adding conversation", "conversation", conv)

	err = repo.addConversation(conv)
	if err != nil {
		log.Error("Error adding conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error adding conversation: %v", err), http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, conv, http.StatusCreated)
}

func getConversation(w http.ResponseWriter, r *http.Request) {
	convId := r.PathValue("id")
	conv, err := repo.getConversation(convId)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, &conv, http.StatusOK)
}

func getAllConversations(w http.ResponseWriter, _ *http.Request) {
	conversations, err := repo.getAllConversations()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving conversations: %v", err), http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, conversations, http.StatusOK)
}

func deleteConversation(w http.ResponseWriter, r *http.Request) {
	convId := r.PathValue("id")
	err := repo.deleteConversation(convId)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error deleting conversation: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func renameConversation(w http.ResponseWriter, r *http.Request) {
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

	conv, err := repo.getConversation(convId)
	if err != nil {
		log.Error("Error retrieving conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusInternalServerError)
		return
	}

	conv.Title = req.Title

	err = repo.updateConversation(conv)
	if err != nil {
		log.Error("Error updating conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error updating conversation: %v", err), http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, &conv, http.StatusOK)
}

func getConversationMessages(w http.ResponseWriter, r *http.Request) {
	convId := r.PathValue("id")
	messages := getAllConversationMessages(convId)
	utils.RespondWithJSON(w, &messages, http.StatusOK)
}
