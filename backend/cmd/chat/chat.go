package chat

import (
	"ai-client/cmd/provider"
	"ai-client/cmd/tools"
	"ai-client/cmd/utils"

	"encoding/json"
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

func chatStream(w http.ResponseWriter, r *http.Request) {
	var req Request
	err := utils.ExtractJSONBody(r, &req)
	if err != nil || req.ConversationID == "" || req.Content == "" {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Find or create conversation
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

	// Save user message
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

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send metadata first (conversation ID, user message ID)
	metadata := provider.StreamMetadata{
		ConversationID: convID,
		UserMessageID:  userMessage.ID,
	}
	metadataJSON, _ := json.Marshal(metadata)
	fmt.Fprintf(w, "event: metadata\ndata: %s\n\n", metadataJSON)
	flusher.Flush()

	ctx := buildContext(convID, userMessage.ID)
	reasoningSetting, _ := getSetting("reasoningEffort")

	providerParams := provider.RequestParams{
		Messages:        ctx,
		Model:           req.Model,
		ReasoningEffort: provider.ReasoningEffort(reasoningSetting),
	}

	responseMessage := Message{
		ID:        -1,
		ConvID:    convID,
		Role:      "assistant",
		Model:     req.Model,
		Content:   "",
		Reasoning: "",
		ParentID:  userMessage.ID,
		Children:  []int{},
	}

	var toolCalls []tools.ToolCall
	var isToolsUsed bool

	completion, err := providerClient.SendChatCompletionStreamRequest(providerParams, w)
	if err != nil {
		log.Error("Error streaming chat completion", "err", err)
		fmt.Fprintf(w, "event: error\ndata: {\"error\": \"%s\"}\n\n", err.Error())
		flusher.Flush()
		responseMessage.Error = err.Error()
	} else {
		responseMessage.Content = completion.Content
		responseMessage.Reasoning = completion.Reasoning
		toolCalls = completion.ToolCalls
		isToolsUsed = len(toolCalls) > 0
	}

	// Save assistant message after streaming completes
	responseMessage.ID, err = saveMessage(responseMessage)
	if err != nil {
		log.Error("Error saving response message", "err", err)
	}

	for len(toolCalls) > 0 {

		toolCall := toolCalls[0]

		providerParams.Messages = append(providerParams.Messages, provider.SimpleMessage{
			Role:     "assistant",
			ToolCall: toolCall,
		})

		toolCall.MessageID = responseMessage.ID
		toolCall.ConvID = convID

		output := tools.ExecuteToolCall(toolCall)
		toolCall.Output = output

		chunk, _ := json.Marshal(provider.StreamChunk{
			ToolCall: toolCall,
		})
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		flusher.Flush()

		// Append tool result message to context for continued completion
		providerParams.Messages = append(providerParams.Messages, provider.SimpleMessage{
			Role: "tool",
			ToolCall: tools.ToolCall{
				ID:          toolCall.ID,
				ReferenceID: toolCall.ReferenceID,
				Name:        toolCall.Name,
				Output:      output,
			},
		})

		toolCalls = toolCalls[1:]
		if len(toolCalls) == 0 {
			completion, err = providerClient.SendChatCompletionStreamRequest(providerParams, w)
			if err != nil {
				log.Error("Error streaming chat completion after tool call", "err", err)
				fmt.Fprintf(w, "event: error\ndata: {\"error\": \"%s\"}\n\n", err.Error())
				flusher.Flush()
				responseMessage.Error = err.Error()
				break
			}
			toolCalls = append(toolCalls, completion.ToolCalls...)
		}

		// Accumulate reasoning for all tool calls
		if responseMessage.Reasoning != "" {
			responseMessage.Reasoning += " \n`using tool:" + toolCall.Name + "`\n " + completion.Reasoning
		}
	}

	// Update assistant message with full content after all tool calls
	if isToolsUsed {
		if err == nil {
			responseMessage.Content = completion.Content
		}
		_, err = updateMessage(responseMessage.ID, responseMessage)
		if err != nil {
			log.Error("Error updating assistant message after tool calls", "err", err)
		}
	}

	log.Debug("Completed streaming chat response", "responseMessageID", responseMessage.ID)

	// Send completion event with message IDs
	completionData := provider.StreamComplete{
		UserMessageID:      userMessage.ID,
		AssistantMessageID: responseMessage.ID,
	}
	completionJSON, _ := json.Marshal(completionData)
	fmt.Fprintf(w, "event: complete\ndata: %s\n\n", completionJSON)
	flusher.Flush()

}

// retryStream streams an alternative assistant response for a given user parent message.
// It does not create a new user message; it uses the provided ParentID as context root.
func retryStream(w http.ResponseWriter, r *http.Request) {
	var req Retry
	err := utils.ExtractJSONBody(r, &req)
	if err != nil || req.ConversationID == "" || req.ParentID <= 0 {
		log.Error("Error unmarshalling retry stream body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure conversation exists
	if err = repo.touchConversation(req.ConversationID); err != nil {
		log.Error("Error retrieving conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusNotFound)
		return
	}

	// Load parent user message
	parent, err := getMessage(req.ParentID)
	if err != nil || parent.Role != "user" {
		log.Error("Invalid parent message for retry stream", "err", err)
		http.Error(w, "Invalid parent message", http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Metadata: no new user message; client already knows conversation
	// We can still send conversationId and echo parent id in userMessageId for consistency if needed.
	metadata := provider.StreamMetadata{
		ConversationID: req.ConversationID,
		UserMessageID:  parent.ID,
	}
	metadataJSON, _ := json.Marshal(metadata)
	fmt.Fprintf(w, "event: metadata\ndata: %s\n\n", metadataJSON)
	flusher.Flush()

	// Build context from the parent message
	ctx := buildContext(req.ConversationID, parent.ID)
	reasoningSetting, _ := getSetting("reasoningEffort")

	providerParams := provider.RequestParams{
		Messages:        ctx,
		Model:           req.Model,
		ReasoningEffort: provider.ReasoningEffort(reasoningSetting),
	}

	// Save assistant message after streaming completes
	responseMessage := Message{
		ID:        -1,
		ConvID:    req.ConversationID,
		Role:      "assistant",
		Model:     req.Model,
		Content:   "",
		Reasoning: "",
		ParentID:  parent.ID,
		Children:  []int{},
	}

	// Stream assistant content
	completion, err := providerClient.SendChatCompletionStreamRequest(providerParams, w)
	if err != nil {
		log.Error("Error streaming retry completion", "err", err)
		fmt.Fprintf(w, "event: error\ndata: {\"error\": \"%s\"}\n\n", err.Error())
		flusher.Flush()
		responseMessage.Error = err.Error()
	} else {
		responseMessage.Content = completion.Content
		responseMessage.Reasoning = completion.Reasoning
	}

	responseID, saveErr := saveMessage(responseMessage)
	if saveErr != nil {
		log.Error("Error saving retry response message", "err", saveErr)
	} else {
		responseMessage.ID = responseID
		// Update parent's children in memory (DB linkage already by parent_id)
		parent.Children = append(parent.Children, responseID)
	}

	// Send completion event with the new assistant message id
	completionData := provider.StreamComplete{
		UserMessageID:      parent.ID,
		AssistantMessageID: responseMessage.ID,
	}
	completionJSON, _ := json.Marshal(completionData)
	fmt.Fprintf(w, "event: complete\ndata: %s\n\n", completionJSON)
	flusher.Flush()
}

func update(W http.ResponseWriter, R *http.Request) {
	var req Update
	err := utils.ExtractJSONBody(R, &req)
	if err != nil || req.ConversationID == "" || req.MessageID < 0 || req.Content == "" {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(W, "Invalid request body", http.StatusBadRequest)
		return
	}

	msg, err := updateMessage(req.MessageID, Message{Content: req.Content})
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
	response.Messages[msg.ID] = msg

	utils.RespondWithJSON(W, &response, http.StatusOK)
}

// 	// Temporarily disabled
// func chat(w http.ResponseWriter, r *http.Request) {
// 	var req Request
// 	err := utils.ExtractJSONBody(r, &req)
// 	if err != nil || req.ConversationID == "" || req.Content == "" {
// 		log.Error("Error unmarshalling request body", "err", err)
// 		http.Error(w, "Invalid request body", http.StatusBadRequest)
// 		return
// 	}

// 	// find or create conversation
// 	convID := req.ConversationID
// 	err = repo.touchConversation(req.ConversationID)
// 	if err != nil {
// 		conv := newConversation("admin")
// 		if err = repo.saveConversation(conv); err != nil {
// 			log.Error("Error creating conversation", "err", err)
// 			http.Error(w, fmt.Sprintf("Error creating conversation: %v", err), http.StatusInternalServerError)
// 			return
// 		}
// 		convID = conv.ID
// 	}

// 	userMessage := Message{
// 		ID:         -1,
// 		ConvID:     convID,
// 		Role:       "user",
// 		Content:    req.Content,
// 		ParentID:   req.ParentID,
// 		Children:   []int{},
// 		Attachment: req.Attachment,
// 	}

// 	userMessage.ID, err = saveMessage(userMessage)
// 	if err != nil {
// 		log.Error("Error saving user message", "err", err)
// 		http.Error(w, fmt.Sprintf("Error saving user message: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	ctx := buildContext(convID, userMessage.ID)
// 	reasoningSetting, _ := getSetting("reasoningEffort")

// 	providerParams := provider.ProviderRequestParams{
// 		Messages:        ctx,
// 		Model:           req.Model,
// 		ReasoningEffort: provider.ReasoningEffort(reasoningSetting),
// 	}

// 	// send to provider
// 	completion, err := providerClient.SendChatCompletionRequest(providerParams)
// 	if err != nil {
// 		log.Error("Error sending chat completion request", "err", err)
// 		http.Error(w, fmt.Sprintf("Chat completion error: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	responseMessage := Message{
// 		ID:        -1,
// 		ConvID:    convID,
// 		Role:      "assistant",
// 		Model:     req.Model,
// 		Content:   completion.Choices[0].Message.Content,
// 		Reasoning: completion.Choices[0].Message.Reasoning,
// 		ParentID:  userMessage.ID,
// 		Children:  []int{},
// 	}

// 	responseMessage.ID, err = saveMessage(responseMessage)
// 	if err != nil {
// 		log.Error("Error saving response message", "err", err)
// 		http.Error(w, fmt.Sprintf("Error saving response message: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	response := &Response{
// 		Messages: make(map[int]*Message),
// 	}
// 	response.Messages[userMessage.ID] = &userMessage
// 	response.Messages[responseMessage.ID] = &responseMessage

// 	utils.RespondWithJSON(w, &response, http.StatusOK)
// }

// // Temporarily disabled
// func retry(w http.ResponseWriter, r *http.Request) {
// 	var req Retry
// 	err := utils.ExtractJSONBody(r, &req)
// 	if err != nil || req.ConversationID == "" {
// 		log.Error("Error unmarshalling request body", "err", err)
// 		http.Error(w, "Invalid request body", http.StatusBadRequest)
// 		return
// 	}

// 	err = repo.touchConversation(req.ConversationID)
// 	if err != nil {
// 		log.Error("Error retrieving conversation", "err", err)
// 		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusNotFound)
// 		return
// 	}

// 	parent, err := getMessage(req.ParentID)
// 	if err != nil || parent.Role != "user" {
// 		log.Error("Error retrieving parent message or invalid role", "err", err)
// 		http.Error(w, "Invalid parent message", http.StatusBadRequest)
// 		return
// 	}

// 	ctx := buildContext(req.ConversationID, parent.ID)
// 	reasoningSetting, _ := getSetting("reasoningEffort")

// 	providerParams := provider.ProviderRequestParams{
// 		Messages:        ctx,
// 		Model:           req.Model,
// 		ReasoningEffort: provider.ReasoningEffort(reasoningSetting),
// 	}

// 	completion, err := providerClient.SendChatCompletionRequest(providerParams)
// 	if err != nil {
// 		log.Error("Error sending chat completion request", "err", err)
// 		http.Error(w, fmt.Sprintf("Chat completion error: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	responseMessage := Message{
// 		ID:        -1,
// 		ConvID:    req.ConversationID,
// 		Model:     req.Model,
// 		Role:      "assistant",
// 		Content:   completion.Choices[0].Message.Content,
// 		Reasoning: completion.Choices[0].Message.Reasoning,
// 		ParentID:  parent.ID,
// 		Children:  []int{},
// 	}

// 	responseMessage.ID, err = saveMessage(responseMessage)
// 	if err != nil {
// 		log.Error("Error saving message", "err", err)
// 		http.Error(w, fmt.Sprintf("Error saving message: %v", err), http.StatusInternalServerError)
// 		return
// 	}

// 	parent.Children = append(parent.Children, responseMessage.ID)

// 	response := &Response{
// 		Messages: make(map[int]*Message),
// 	}

// 	response.Messages[parent.ID] = parent
// 	response.Messages[responseMessage.ID] = &responseMessage

// 	utils.RespondWithJSON(w, &response, http.StatusOK)
// }
