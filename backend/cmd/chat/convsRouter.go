package chat

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/utils"
	"fmt"
	"net/http"
)

func ConvsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET     /", GetAllConversations)
	mux.HandleFunc("POST 	  /add", AddConversation)
	mux.HandleFunc("GET  	  /{id}", GetConversation)
	mux.HandleFunc("DELETE  /{id}", DeleteConversation)
	mux.HandleFunc("POST 	  /{id}/rename", RenameConversation)

	return http.StripPrefix("/api/conversations", auth.Authenticated(mux))
}

func AddConversation(w http.ResponseWriter, r *http.Request) {
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

	err = repo.AddConversation(conv)
	if err != nil {
		log.Error("Error adding conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error adding conversation: %v", err), http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, conv, http.StatusCreated)
}

func GetConversation(w http.ResponseWriter, r *http.Request) {
	convId := r.PathValue("id")
	conv, err := repo.GetConversation(convId)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, &conv, http.StatusOK)
}

func GetAllConversations(w http.ResponseWriter, _ *http.Request) {
	conversations, err := repo.GetAllConversations()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving conversations: %v", err), http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, conversations, http.StatusOK)
}

func DeleteConversation(w http.ResponseWriter, r *http.Request) {
	convId := r.PathValue("id")
	err := repo.DeleteConversation(convId)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error deleting conversation: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func RenameConversation(w http.ResponseWriter, r *http.Request) {
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

	conv, err := repo.GetConversation(convId)
	if err != nil {
		log.Error("Error retrieving conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusInternalServerError)
		return
	}

	conv.Title = req.Title

	err = repo.UpdateConversation(conv)
	if err != nil {
		log.Error("Error updating conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error updating conversation: %v", err), http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, &conv, http.StatusOK)
}
