package chat

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/providers"
	"ai-client/cmd/tools"
	"ai-client/cmd/utils"

	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

type Request struct {
	ConversationID  string   `json:"conversationId"`
	ParentID        int      `json:"parentId"`
	Model           string   `json:"model"`
	Content         string   `json:"content"`
	WebSearch       bool     `json:"webSearch,omitempty"`
	AttachedFileIDs []string `json:"attachedFileIds,omitempty"`
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
	user := auth.GetUsername(r)
	var req Request
	err := utils.ExtractJSONBody(r, &req)
	if err != nil || req.ConversationID == "" || req.Content == "" {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Find or create conversation
	convID := req.ConversationID
	err = conversations.Touch(req.ConversationID, user)
	if err != nil {
		conv := newConversation(user)
		if err = conversations.Save(conv); err != nil {
			log.Error("Error creating conversation", "err", err)
			http.Error(w, fmt.Sprintf("Error creating conversation: %v", err), http.StatusBadRequest)
			return
		}
		convID = conv.ID
	}

	attachedFiles, err := files.GetByIDs(req.AttachedFileIDs, user)
	if err != nil {
		log.Error("Error getting files data", "err", err)
		http.Error(w, fmt.Sprintf("Error getting files data: %v", err), http.StatusBadRequest)
		return
	}

	// Save user message
	userMessage := Message{
		ID:       -1,
		ConvID:   convID,
		Role:     "user",
		Content:  req.Content,
		ParentID: req.ParentID,
		Children: []int{},
	}

	userMessage.Attachments = make([]Attachment, 0)
	for _, file := range attachedFiles {
		attachment := Attachment{
			ID:        uuid.NewString(),
			MessageID: -1, // will be updated with correct message ID when saving
			File:      file,
		}
		userMessage.Attachments = append(userMessage.Attachments, attachment)
	}

	userMessage.ID, err = saveMessage(userMessage)
	if err != nil {
		log.Error("Error saving user message", "err", err)
		http.Error(w, fmt.Sprintf("Error saving user message: %v", err), http.StatusBadRequest)
		return
	}

	utils.AddStreamHeaders(w)

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send metadata first (conversation ID, user message ID)
	metadata := providers.StreamMetadata{
		ConversationID: convID,
		UserMessageID:  userMessage.ID,
	}
	metadataJSON, _ := json.Marshal(metadata)
	fmt.Fprintf(w, "event: metadata\ndata: %s\n\n", metadataJSON)
	flusher.Flush()

	// Build context from user message
	ctx := buildContext(convID, userMessage.ID, user)
	reasoningSetting, _ := settings.Get("reasoningEffort", user)

	providerParams := providers.RequestParams{
		Messages:        ctx,
		Model:           req.Model,
		ReasoningEffort: providers.ReasoningEffort(reasoningSetting),
		User:            user,
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

	completion, err := provider.SendChatCompletionStreamRequest(providerParams, w)
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

		providerParams.Messages = append(providerParams.Messages, providers.SimpleMessage{
			Role:     "assistant",
			ToolCall: toolCall,
		})

		toolCall.MessageID = responseMessage.ID
		toolCall.ConvID = convID

		output := tools.ExecuteToolCall(toolCall, user)
		toolCall.Output = output

		chunk, _ := json.Marshal(providers.StreamChunk{
			ToolCall: toolCall,
		})
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		flusher.Flush()

		// Append tool result message to context for continued completion
		providerParams.Messages = append(providerParams.Messages, providers.SimpleMessage{
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
			completion, err = provider.SendChatCompletionStreamRequest(providerParams, w)
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
		if responseMessage.Reasoning != "" || completion.Reasoning != "" {
			responseMessage.Reasoning += "  \n`using tool:" + toolCall.Name + "`  \n" + completion.Reasoning
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
	completionData := providers.StreamComplete{
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
	user := auth.GetUsername(r)
	var req Retry
	err := utils.ExtractJSONBody(r, &req)
	if err != nil || req.ConversationID == "" || req.ParentID <= 0 {
		log.Error("Error unmarshalling retry stream body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure conversation exists
	if err = conversations.Touch(req.ConversationID, user); err != nil {
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

	utils.AddStreamHeaders(w)

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Metadata: no new user message; client already knows conversation
	// We can still send conversationId and echo parent id in userMessageId for consistency if needed.
	metadata := providers.StreamMetadata{
		ConversationID: req.ConversationID,
		UserMessageID:  parent.ID,
	}
	metadataJSON, _ := json.Marshal(metadata)
	fmt.Fprintf(w, "event: metadata\ndata: %s\n\n", metadataJSON)
	flusher.Flush()

	// Build context from the parent message
	ctx := buildContext(req.ConversationID, parent.ID, user)
	reasoningSetting, _ := settings.Get("reasoningEffort", user)

	providerParams := providers.RequestParams{
		Messages:        ctx,
		Model:           req.Model,
		ReasoningEffort: providers.ReasoningEffort(reasoningSetting),
		User:            user,
	}

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

	var toolCalls []tools.ToolCall
	var isToolsUsed bool

	// Stream assistant content
	completion, err := provider.SendChatCompletionStreamRequest(providerParams, w)
	if err != nil {
		log.Error("Error streaming retry completion", "err", err)
		fmt.Fprintf(w, "event: error\ndata: {\"error\": \"%s\"}\n\n", err.Error())
		flusher.Flush()
		responseMessage.Error = err.Error()
	} else {
		responseMessage.Content = completion.Content
		responseMessage.Reasoning = completion.Reasoning
		toolCalls = completion.ToolCalls
		isToolsUsed = len(toolCalls) > 0
	}

	responseID, saveErr := saveMessage(responseMessage)
	if saveErr != nil {
		log.Error("Error saving retry response message", "err", saveErr)
	} else {
		responseMessage.ID = responseID
		// Update parent's children in memory (DB linkage already by parent_id)
		parent.Children = append(parent.Children, responseID)
	}

	for len(toolCalls) > 0 {

		toolCall := toolCalls[0]

		providerParams.Messages = append(providerParams.Messages, providers.SimpleMessage{
			Role:     "assistant",
			ToolCall: toolCall,
		})

		toolCall.MessageID = responseMessage.ID
		toolCall.ConvID = req.ConversationID

		output := tools.ExecuteToolCall(toolCall, user)
		toolCall.Output = output

		chunk, _ := json.Marshal(providers.StreamChunk{
			ToolCall: toolCall,
		})
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		flusher.Flush()

		// Append tool result message to context for continued completion
		providerParams.Messages = append(providerParams.Messages, providers.SimpleMessage{
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
			completion, err = provider.SendChatCompletionStreamRequest(providerParams, w)
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
		if responseMessage.Reasoning != "" || completion.Reasoning != "" {
			responseMessage.Reasoning += "  \n`using tool:" + toolCall.Name + "`  \n" + completion.Reasoning
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

	// Send completion event with the new assistant message id
	completionData := providers.StreamComplete{
		UserMessageID:      parent.ID,
		AssistantMessageID: responseMessage.ID,
	}
	completionJSON, _ := json.Marshal(completionData)
	fmt.Fprintf(w, "event: complete\ndata: %s\n\n", completionJSON)
	flusher.Flush()
}

func update(W http.ResponseWriter, R *http.Request) {
	user := auth.GetUsername(R)
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

	err = conversations.Touch(req.ConversationID, user)
	if err != nil {
		log.Error("Error touching conversation", "err", err)
	}

	response := &Response{
		Messages: make(map[int]*Message),
	}
	response.Messages[msg.ID] = msg

	utils.RespondWithJSON(W, &response, http.StatusOK)
}
