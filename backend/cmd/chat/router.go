package chat

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/provider"
	"ai-client/cmd/utils"
	"fmt"
	"github.com/charmbracelet/log"
	"net/http"
	"time"
)

var repo = NewInMemoryConversationRepo()

type Request struct {
	ConversationID string `json:"conversationId"`
	ActiveMessage  int    `json:"activeMessageId"`
	Model          string `json:"model"`
	Content        string `json:"content"`
	WebSearch      bool   `json:"webSearch,omitempty"`
}

func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/chat", chat)

	return auth.Authenticated(mux)
}

func SettingsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", getAllSettings)
	mux.HandleFunc("POST /update", updateSettings)

	return http.StripPrefix("/api/settings", auth.Authenticated(mux))
}

func ConvsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET     /", GetAllConversations)
	mux.HandleFunc("POST 	  /add", AddConversation)
	mux.HandleFunc("GET  	  /{id}", GetConversation)
	mux.HandleFunc("DELETE  /{id}", DeleteConversation)
	mux.HandleFunc("POST 	  /{id}/rename", RenameConversation)

	return http.StripPrefix("/api/conversations", auth.Authenticated(mux))
}

func chat(w http.ResponseWriter, r *http.Request) {
	var req Request
	err := utils.ExtractJSONBody(r, &req)
	if err != nil || req.ConversationID == "" || req.Content == "" {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// find or create conversation
	conv, err := repo.GetConversation(req.ConversationID)
	if err != nil {
		id := req.ConversationID
		if id == "" {
			id = fmt.Sprintf("conv-%s", time.Now().Format("20060102-150405"))
		}
		conv = NewConversation(id)
		if err := repo.AddConversation(conv); err != nil {
			log.Error("Error creating conversation", "err", err)
			http.Error(w, fmt.Sprintf("Error creating conversation: %v", err), http.StatusInternalServerError)
			return
		}
	}

	conv.ActiveMessage = req.ActiveMessage

	userMessage := Message{
		ID:       -1,
		Role:     "user",
		Content:  req.Content,
		ParentID: conv.ActiveMessage,
		Children: []int{},
	}

	userMessage.ID = conv.AppendMessage(userMessage)

	// build context
	var path []int
	var current = userMessage.ID
	log.Debug("Current message ID", "id", current)
	for {
		leaf, err := conv.GetMessage(current)
		if err != nil {
			break
		}
		path = append(path, current)
		current = leaf.ParentID
	}

	var messages []provider.SimpleMessage

	messages = append(messages, provider.SimpleMessage{
		Role:    "system",
		Content: settings["systemPrompt"],
	})

	for i := len(path) - 1; i >= 0; i-- {
		msg, err := conv.GetMessage(path[i])
		if err != nil {
			break
		}
		messages = append(messages, provider.SimpleMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	//debug
	log.Debug("Path", "path", path)
	log.Debug("Messages", "messages", messages)
	//

	completion, err := provider.SendChatCompletionRequest(messages, req.Model)
	if err != nil {
		log.Error("Error sending chat completion request", "err", err)
		http.Error(w, fmt.Sprintf("Chat completion error: %v", err), http.StatusInternalServerError)
		return
	}

	responseMessage := Message{
		ID:       -1,
		Role:     "assistant",
		Content:  completion.Choices[0].Message.Content,
		ParentID: 0,
		Children: []int{},
	}

	responseMessage.ID = conv.AppendMessage(responseMessage)

	err = repo.UpdateConversation(conv)
	if err != nil {
		log.Error("Error updating conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error updating conversation: %v", err), http.StatusInternalServerError)
		return
	}

	response := struct {
		Messages map[int]*Message `json:"messages"`
	}{}
	response.Messages = make(map[int]*Message)
	response.Messages[userMessage.ID], _ = conv.GetMessage(userMessage.ID)
	response.Messages[responseMessage.ID], _ = conv.GetMessage(responseMessage.ID)

	utils.RespondWithJSON(w, response, http.StatusOK)
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

func getAllSettings(w http.ResponseWriter, _ *http.Request) {
	response := Settings{settings}
	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func updateSettings(w http.ResponseWriter, r *http.Request) {
	var request Settings
	err := utils.ExtractJSONBody(r, &request)
	if err != nil {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	for key, value := range request.Settings {

		if key == "" {
			log.Error("Empty setting key", "key", key, "value", value)
			http.Error(w, "Invalid setting key", http.StatusBadRequest)
			return
		}

		settings[key] = value
	}

	response := Settings{settings}

	utils.RespondWithJSON(w, &response, http.StatusOK)
}
