package chat

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/provider"
	"ai-client/cmd/utils"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var log = utils.Log
var repo = NewInMemoryConversationRepo()

type Request struct {
	ConversationID string `json:"conversationId"`
	ParentID       int    `json:"parentId"`
	Model          string `json:"model"`
	Content        string `json:"content"`
	WebSearch      bool   `json:"webSearch,omitempty"`
	Attachment     string `json:"attachment,omitempty"`
}

type Retry struct {
	ConversationID string `json:"conversationId"`
	ParentID       int    `json:"parentId"`
	Model          string `json:"model"`
}

type Update struct {
	ConversationID string `json:"conversationId"`
	MessageID      int    `json:"messageId"`
	Content        string `json:"content"`
}

type Response struct {
	Messages map[int]*Message `json:"messages"`
}

func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /new", chat)
	mux.HandleFunc("POST /retry", retry)
	mux.HandleFunc("POST /update", update)

	return http.StripPrefix("/api/chat", auth.Authenticated(mux))
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

	userMessage := Message{
		ID:         -1,
		Role:       "user",
		Content:    req.Content,
		ParentID:   req.ParentID,
		Children:   []int{},
		Attachment: req.Attachment,
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
			Image:   msg.Attachment,
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
		ParentID: userMessage.ID,
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

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func retry(w http.ResponseWriter, r *http.Request) {
	var req Retry
	err := utils.ExtractJSONBody(r, &req)
	if err != nil || req.ConversationID == "" {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	conv, err := repo.GetConversation(req.ConversationID)
	if err != nil {
		log.Error("Error retrieving conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusInternalServerError)
		return
	}

	parent, err := conv.GetMessage(req.ParentID)
	if err != nil || parent.Role != "user" {
		log.Error("Error retrieving parent message or invalid role", "err", err)
		http.Error(w, "Invalid parent message", http.StatusBadRequest)
		return
	}

	var path []int
	var current = parent.ID
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
		ParentID: parent.ID,
		Children: []int{},
	}

	responseMessage.ID = conv.AppendMessage(responseMessage)
	err = repo.UpdateConversation(conv)
	if err != nil {
		log.Error("Error updating conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error updating conversation: %v", err), http.StatusInternalServerError)
		return
	}

	response := &Response{
		Messages: make(map[int]*Message),
	}

	response.Messages[parent.ID], _ = conv.GetMessage(parent.ID)
	response.Messages[responseMessage.ID], _ = conv.GetMessage(responseMessage.ID)

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func update(W http.ResponseWriter, R *http.Request) {
	var req Update
	err := utils.ExtractJSONBody(R, &req)
	if err != nil || req.ConversationID == "" || req.MessageID < 0 || req.Content == "" {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(W, "Invalid request body", http.StatusBadRequest)
		return
	}

	conv, err := repo.GetConversation(req.ConversationID)
	if err != nil {
		log.Error("Error retrieving conversation", "err", err)
		http.Error(W, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusInternalServerError)
		return
	}

	msg, err := conv.GetMessage(req.MessageID)
	if err != nil {
		log.Error("Error retrieving message", "err", err)
		http.Error(W, fmt.Sprintf("Error retrieving message: %v", err), http.StatusInternalServerError)
		return
	}

	msg.Content = req.Content
	err = conv.UpdateMessage(msg.ID, *msg)
	if err != nil {
		log.Error("Error updating message", "err", err)
		http.Error(W, fmt.Sprintf("Error updating message: %v", err), http.StatusInternalServerError)
		return
	}

	err = repo.UpdateConversation(conv)
	if err != nil {
		log.Error("Error updating conversation", "err", err)
		http.Error(W, fmt.Sprintf("Error updating conversation: %v", err), http.StatusInternalServerError)
		return
	}

	response := &Response{
		Messages: make(map[int]*Message),
	}
	response.Messages[msg.ID] = msg

	utils.RespondWithJSON(W, &response, http.StatusOK)
}

func FileHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /upload", upload)

	return http.StripPrefix("/api/files", mux)
}

func upload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // limit to 10MB
	if err != nil {
		log.Error("Error parsing multipart form", "err", err)
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		log.Error("Error retrieving file from form data", "err", err)
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}

	defer file.Close()

	filePath, err := utils.SaveUploadedFile(file, handler)
	if err != nil {
		log.Error("Error saving uploaded file", "err", err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	filePath = strings.TrimPrefix(filePath, ".")

	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}

	if !strings.HasPrefix(filePath, "/data/uploads/") {
		log.Debug("Adjusting file path", "original", filePath)
		filePath = "/data/uploads/" + strings.TrimPrefix(filePath, "/")
	}

	fileUrl := utils.GetServerURL(r) + filePath

	utils.RespondWithJSON(w, map[string]string{"fileUrl": fileUrl}, http.StatusOK)
}
