package chat

import (
	"ai-client/cmd/providers"
	"ai-client/cmd/tools"
	"ai-client/cmd/utils"

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
	user := utils.ExtractContextUser(r)
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

		// Broadcast new conversation to other sessions
		sessionID := r.Header.Get("X-Session-ID")
		syncManager.Broadcast(user, sessionID, ConversationEvent{
			Type:           EventConversationCreated,
			ConversationID: conv.ID,
			Conversation:   conv,
		})
	} else {
		// Broadcast update to other sessions to reorder sidebar
		if conv, err := conversations.GetByID(convID, user); err == nil {
			sessionID := r.Header.Get("X-Session-ID")
			syncManager.Broadcast(user, sessionID, ConversationEvent{
				Type:           EventConversationUpdated,
				ConversationID: conv.ID,
				Conversation:   conv,
			})
		}
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
		Status:   "completed",
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
	syncManager.Broadcast(user, r.Header.Get("X-Session-ID"), ConversationEvent{
		Type:           EventMessageSaved,
		ConversationID: convID,
		MessageID:      userMessage.ID,
		Message:        &userMessage,
	})

	// prepare for streaming response
	sc := utils.StreamClient{
		User:      user,
		MessageID: userMessage.ID,
		Writer:    w,
	}
	utils.AddStreamHeaders(sc.Writer)
	_, ok := sc.Writer.(http.Flusher)
	if !ok {
		log.Error("Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	responseMessage := Message{
		ID:        -1,
		ConvID:    convID,
		Role:      "assistant",
		Model:     req.Model,
		Content:   "",
		Reasoning: "",
		Status:    "pending",
		ParentID:  userMessage.ID,
		Children:  []int{},
	}

	// Save assistant message, so /resume can find it even if no content is generated before interruption
	responseMessage.ID, err = saveMessage(responseMessage)
	if err != nil {
		log.Error("Error saving response message", "err", err)
	} else {
		syncManager.Broadcast(user, r.Header.Get("X-Session-ID"), ConversationEvent{
			Type:           EventMessageSaved,
			ConversationID: convID,
			MessageID:      responseMessage.ID,
			Message:        &responseMessage,
		})
	}

	// Send metadata first (conversation ID, user message ID)
	metadata := utils.StreamMetadata{
		ConversationID:     convID,
		UserMessageID:      userMessage.ID,
		AssistantMessageID: responseMessage.ID,
	}

	utils.SendStreamChunk(sc, utils.StreamChunk{
		Type:    utils.EVENT_METADATA,
		Payload: metadata,
	})

	// Build context from user message
	ctx := buildContext(convID, userMessage.ID, user)
	reasoningSetting, _ := settings.Get("reasoningEffort", user)

	providerParams := providers.RequestParams{
		Messages:        ctx,
		Model:           req.Model,
		ReasoningEffort: providers.ReasoningEffort(reasoningSetting),
		User:            user,
		MessageID:       responseMessage.ID,
	}

	var calls []tools.ToolCall
	var isToolsUsed bool
	var streamStats utils.StreamStats

	completion, err := provider.SendChatCompletionStreamRequest(providerParams, sc)
	if err != nil {
		log.Error("Error streaming chat completion", "err", err)
		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.EVENT_ERROR,
			Payload: err.Error(),
		})
		responseMessage.Error = err.Error()
	} else {
		responseMessage.Content = completion.Content
		responseMessage.Reasoning = completion.Reasoning
		streamStats = completion.Stats
		calls = completion.ToolCalls
	}

	isToolsUsed = len(calls) > 0

	if isToolsUsed {
		completion, err = enterAgentLoop(
			calls, providerParams,
			&responseMessage,
			convID,
			user, sc,
		)
		if err != nil {
			responseMessage.Error = err.Error()
		} else {
			responseMessage.Content += completion.Content
			streamStats = completion.Stats
		}
	}

	responseMessage.Status = "completed"
	responseMessage.Speed = streamStats.Speed
	responseMessage.TokenCount = streamStats.CompletionTokens
	responseMessage.ContextSize = streamStats.PromptTokens

	if updatedMsg, updateErr := updateMessage(responseMessage.ID, responseMessage); updateErr != nil {
		log.Error("Error updating assistant message after tool calls", "err", updateErr)
	} else if updatedMsg != nil {
		syncManager.Broadcast(user, r.Header.Get("X-Session-ID"), ConversationEvent{
			Type:           EventMessageUpdated,
			ConversationID: convID,
			MessageID:      updatedMsg.ID,
			Message:        updatedMsg,
		})
	}

	log.Debug("Completed streaming chat response", "responseMessageID", responseMessage.ID)

	// Send completion event with message IDs
	completionData := utils.StreamComplete{
		UserMessageID:      userMessage.ID,
		AssistantMessageID: responseMessage.ID,
		StreamStats:        streamStats,
	}
	utils.SendStreamChunk(sc, utils.StreamChunk{
		Type:    utils.EVENT_COMPLETE,
		Payload: completionData,
	})

}

// retryStream streams an alternative assistant response for a given user parent message.
// It does not create a new user message; it uses the provided ParentID as context root.
func retryStream(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	var req Retry
	err := utils.ExtractJSONBody(r, &req)
	if err != nil || req.ConversationID == "" || req.ParentID <= 0 {
		log.Error("Error unmarshalling retry stream body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure conversation exists and update its timestamp
	if err = conversations.Touch(req.ConversationID, user); err != nil {
		log.Error("Error retrieving conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusNotFound)
		return
	}

	// Broadcast update to other sessions to reorder sidebar
	if conv, err := conversations.GetByID(req.ConversationID, user); err == nil {
		sessionID := r.Header.Get("X-Session-ID")
		syncManager.Broadcast(user, sessionID, ConversationEvent{
			Type:           EventConversationUpdated,
			ConversationID: conv.ID,
			Conversation:   conv,
		})
	}

	// Load parent user message
	parent, err := getMessage(req.ParentID)
	if err != nil || parent.Role != "user" {
		log.Error("Invalid parent message for retry stream", "err", err)
		http.Error(w, "Invalid parent message", http.StatusBadRequest)
		return
	}

	sc := utils.StreamClient{
		User:      user,
		MessageID: parent.ID,
		Writer:    w,
	}

	utils.AddStreamHeaders(sc.Writer)

	_, ok := sc.Writer.(http.Flusher)
	if !ok {
		log.Error("Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	responseMessage := Message{
		ID:        -1,
		ConvID:    req.ConversationID,
		Role:      "assistant",
		Model:     req.Model,
		Content:   "",
		Reasoning: "",
		Status:    "pending",
		ParentID:  parent.ID,
		Children:  []int{},
	}

	responseMessage.ID, err = saveMessage(responseMessage)
	if err != nil {
		log.Error("Error saving retry response message", "err", err)
	} else {
		syncManager.Broadcast(user, r.Header.Get("X-Session-ID"), ConversationEvent{
			Type:           EventMessageSaved,
			ConversationID: req.ConversationID,
			MessageID:      responseMessage.ID,
			Message:        &responseMessage,
		})
	}

	// Metadata: no new user message; client already knows conversation
	metadata := utils.StreamMetadata{
		ConversationID:     req.ConversationID,
		UserMessageID:      parent.ID,
		AssistantMessageID: responseMessage.ID,
	}
	utils.SendStreamChunk(sc, utils.StreamChunk{
		Type:    utils.EVENT_METADATA,
		Payload: metadata,
	})

	// Build context from the parent message
	ctx := buildContext(req.ConversationID, parent.ID, user)
	reasoningSetting, _ := settings.Get("reasoningEffort", user)

	providerParams := providers.RequestParams{
		Messages:        ctx,
		Model:           req.Model,
		ReasoningEffort: providers.ReasoningEffort(reasoningSetting),
		User:            user,
		MessageID:       responseMessage.ID,
	}

	var calls []tools.ToolCall
	var isToolsUsed bool
	var streamStats utils.StreamStats

	// Stream assistant content
	completion, err := provider.SendChatCompletionStreamRequest(providerParams, sc)
	if err != nil {
		log.Error("Error streaming retry completion", "err", err)
		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.EVENT_ERROR,
			Payload: err.Error(),
		})
		responseMessage.Error = err.Error()
	} else {
		responseMessage.Content = completion.Content
		responseMessage.Reasoning = completion.Reasoning
		streamStats = completion.Stats
		calls = completion.ToolCalls
	}

	isToolsUsed = len(calls) > 0
	// if !isToolsUsed {
	// }

	if isToolsUsed {
		completion, err = enterAgentLoop(
			calls, providerParams,
			&responseMessage,
			req.ConversationID,
			user, sc,
		)
		if err != nil {
			responseMessage.Error = err.Error()
		} else {
			responseMessage.Content += completion.Content
			streamStats = completion.Stats
		}
	}

	responseMessage.Status = "completed"
	responseMessage.Speed = streamStats.Speed
	responseMessage.TokenCount = streamStats.CompletionTokens
	responseMessage.ContextSize = streamStats.PromptTokens

	if updatedMsg, updateErr := updateMessage(responseMessage.ID, responseMessage); updateErr != nil {
		log.Error("Error updating assistant message after tool calls", "err", updateErr)
	} else if updatedMsg != nil {
		syncManager.Broadcast(user, r.Header.Get("X-Session-ID"), ConversationEvent{
			Type:           EventMessageUpdated,
			ConversationID: req.ConversationID,
			MessageID:      updatedMsg.ID,
			Message:        updatedMsg,
		})
	}

	// Send completion event with the new assistant message id
	completionData := utils.StreamComplete{
		UserMessageID:      parent.ID,
		AssistantMessageID: responseMessage.ID,
		StreamStats:        streamStats,
	}
	utils.SendStreamChunk(sc, utils.StreamChunk{
		Type:    utils.EVENT_COMPLETE,
		Payload: completionData,
	})
}

func enterAgentLoop(
	calls []tools.ToolCall,
	providerParams providers.RequestParams,
	responseMessage *Message,
	convID, user string,
	sc utils.StreamClient,
) (*providers.ChatCompletionMessage, error) {
	for _, toolCall := range calls {

		providerParams.Messages = append(providerParams.Messages, providers.SimpleMessage{
			Role:     "assistant",
			ToolCall: toolCall,
		})

		toolCall.MessageID = responseMessage.ID
		toolCall.ConvID = convID

		output := tools.ExecuteMCPTool(toolCall, user)
		toolCall.Output = output

		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.TOOL_CALL,
			Payload: toolCall,
		})

		err := toolCalls.Save(&toolCall)
		if err != nil {
			log.Error("Error saving tool call output", "err", err)
		}

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

	}

	completion, err := provider.SendChatCompletionStreamRequest(providerParams, sc)
	if err != nil {
		log.Error("Error streaming chat completion after tool call", "err", err)
		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.EVENT_ERROR,
			Payload: err.Error(),
		})
		return completion, err
	}

	// Accumulate reasoning for all tool calls
	if responseMessage.Reasoning != "" || completion.Reasoning != "" {
		for _, toolCall := range calls {
			responseMessage.Reasoning += "  \n`using tool:" + toolCall.Name + "`  \n"
		}
		responseMessage.Reasoning += completion.Reasoning
	}

	calls = completion.ToolCalls
	if len(calls) > 0 {
		return enterAgentLoop(calls, providerParams, responseMessage, convID, user, sc)
	}

	return completion, err
}

func update(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	var req Update
	err := utils.ExtractJSONBody(r, &req)
	if err != nil || req.ConversationID == "" || req.MessageID < 0 || req.Content == "" {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = conversations.Touch(req.ConversationID, user)
	if err != nil {
		log.Error("Error updating conversation", "err", err)
		http.Error(w, fmt.Sprintf("Error updating conversation: %v", err), http.StatusNotFound)
		return
	}

	// Broadcast update to other sessions to reorder sidebar
	if conv, err := conversations.GetByID(req.ConversationID, user); err == nil {
		sessionID := r.Header.Get("X-Session-ID")
		syncManager.Broadcast(user, sessionID, ConversationEvent{
			Type:           EventConversationUpdated,
			ConversationID: conv.ID,
			Conversation:   conv,
		})
	}

	msg, err := updateMessage(req.MessageID, Message{Content: req.Content})
	if err != nil {
		log.Error("Error updating message", "err", err)
		http.Error(w, fmt.Sprintf("Error updating message: %v", err), http.StatusInternalServerError)
		return
	}
	syncManager.Broadcast(user, r.Header.Get("X-Session-ID"), ConversationEvent{
		Type:           EventMessageUpdated,
		ConversationID: req.ConversationID,
		MessageID:      msg.ID,
		Message:        msg,
	})

	response := &Response{
		Messages: make(map[int]*Message),
	}
	response.Messages[msg.ID] = msg

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func cancelStream(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)

	messageIDStr := r.URL.Query().Get("messageId")
	if messageIDStr == "" {
		log.Error("Missing messageId parameter")
		http.Error(w, "Missing messageId parameter", http.StatusBadRequest)
		return
	}

	var messageID int
	_, err := fmt.Sscanf(messageIDStr, "%d", &messageID)
	if err != nil || messageID <= 0 {
		log.Error("Invalid messageId parameter", "err", err)
		http.Error(w, "Invalid messageId parameter", http.StatusBadRequest)
		return
	}

	cancelled := providers.CancelStream(messageID, user)
	if cancelled {
		log.Debug("Cancelling stream", "messageID", messageID)
	} else {
		log.Warn("Stream not found for cancellation or unauthorized", "messageID", messageID)
	}

	// Force status to completed so the frontend never gets stuck on pending
	msg, err := getMessage(messageID)
	if err != nil {
		log.Error("Failed to fetch message after cancel", "err", err)
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	if msg.Status == "pending" {
		msg.Status = "completed"
		updated, updateErr := updateMessage(messageID, *msg)
		if updateErr != nil {
			log.Error("Failed to force-complete message after cancel", "err", updateErr)
		} else if updated != nil {
			msg = updated
			syncManager.Broadcast(user, r.Header.Get("X-Session-ID"), ConversationEvent{
				Type:           EventMessageUpdated,
				ConversationID: msg.ConvID,
				MessageID:      msg.ID,
				Message:        msg,
			})
		}
	}

	utils.RespondWithJSON(w, msg, http.StatusOK)
}
