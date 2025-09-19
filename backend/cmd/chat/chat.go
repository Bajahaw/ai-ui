package chat

import (
	"ai-client/cmd/provider"
	"ai-client/cmd/utils"
	"fmt"
	"net/http"
)

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

func chat(w http.ResponseWriter, r *http.Request) {
	var req Request
	err := utils.ExtractJSONBody(r, &req)
	if err != nil || req.ConversationID == "" || req.Content == "" {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// find or create conversation
	convID := req.ConversationID
	err = repo.touchConversation(req.ConversationID)
	if err != nil {
		conv := newConversation("admin")
		if err = repo.saveConversation(conv); err != nil {
			log.Error("Error creating conversation", "err", err)
			http.Error(w, fmt.Sprintf("Error creating conversation: %v", err), http.StatusInternalServerError)
			return
		}
		convID = conv.ID
	}

	userMessage := Message{
		ID:         -1,
		ConvID:     convID,
		Role:       "user",
		Content:    req.Content,
		ParentID:   req.ParentID,
		Children:   []int{},
		Attachment: req.Attachment,
	}

	userMessage.ID, err = saveMessage(userMessage)
	if err != nil {
		log.Error("Error saving user message", "err", err)
		http.Error(w, fmt.Sprintf("Error saving user message: %v", err), http.StatusInternalServerError)
		return
	}

	// build context
	convMessages := getAllConversationMessages(convID) // todo: cache or something
	var path []int
	var current = userMessage.ID
	log.Debug("Current message ID", "id", current)
	for {
		leaf, ok := convMessages[current]
		if !ok {
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
		msg, ok := convMessages[path[i]]
		if !ok {
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

	responseMessage.ID, err = saveMessage(responseMessage)
	if err != nil {
		log.Error("Error saving response message", "err", err)
		http.Error(w, fmt.Sprintf("Error saving response message: %v", err), http.StatusInternalServerError)
		return
	}

	response := struct {
		Messages map[int]*Message `json:"messages"`
	}{}
	response.Messages = make(map[int]*Message)
	response.Messages[userMessage.ID] = &userMessage
	response.Messages[responseMessage.ID] = &responseMessage

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

	err = repo.touchConversation(req.ConversationID)
	if err != nil {
		log.Error("Error retrieving conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusNotFound)
		return
	}

	parent, err := getMessage(req.ParentID)
	if err != nil || parent.Role != "user" {
		log.Error("Error retrieving parent message or invalid role", "err", err)
		http.Error(w, "Invalid parent message", http.StatusBadRequest)
		return
	}

	var convMessages = getAllConversationMessages(req.ConversationID) // todo: cache or something
	var path []int
	var current = parent.ID
	log.Debug("Current message ID", "id", current)
	for {
		leaf, ok := convMessages[current]
		if !ok {
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
		msg, ok := convMessages[path[i]]
		if !ok {
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

	responseMessage.ID, err = saveMessage(responseMessage)
	if err != nil {
		log.Error("Error saving message", "err", err)
		http.Error(w, fmt.Sprintf("Error saving message: %v", err), http.StatusInternalServerError)
		return
	}

	parent.Children = append(parent.Children, responseMessage.ID)

	response := &Response{
		Messages: make(map[int]*Message),
	}

	response.Messages[parent.ID] = parent
	response.Messages[responseMessage.ID] = &responseMessage

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

	err = updateMessage(req.MessageID, Message{Content: req.Content})
	if err != nil {
		log.Error("Error updating message", "err", err)
		http.Error(W, fmt.Sprintf("Error updating message: %v", err), http.StatusInternalServerError)
		return
	}

	err = repo.touchConversation(req.ConversationID)
	if err != nil {
		log.Error("Error touching conversation", "err", err)
	}

	response := &Response{
		Messages: make(map[int]*Message),
	}
	response.Messages[req.MessageID], err = getMessage(req.MessageID)
	if err != nil {
		log.Error("Error retrieving updated message", "err", err)
		http.Error(W, fmt.Sprintf("Error retrieving updated message: %v", err), http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(W, &response, http.StatusOK)
}
