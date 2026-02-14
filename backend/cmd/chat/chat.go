package chat

import (
	"ai-client/cmd/providers"
	"ai-client/cmd/tools"
	"ai-client/cmd/utils"
	"strconv"
	"time"

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

	// Send metadata first (conversation ID, user message ID)
	metadata := utils.StreamMetadata{
		ConversationID: convID,
		UserMessageID:  userMessage.ID,
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

	// Save assistant message, so /resume can find it even if no content is generated before interruption
	responseMessage.ID, err = saveMessage(responseMessage)
	if err != nil {
		log.Error("Error saving response message", "err", err)
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
	if !isToolsUsed {
		responseMessage.Status = "completed"
		responseMessage.Speed = streamStats.Speed
		responseMessage.TokenCount = streamStats.CompletionTokens
		responseMessage.ContextSize = streamStats.PromptTokens
	}

	for len(calls) > 0 {

		toolCall := calls[0]
		toolCall.MessageID = responseMessage.ID
		toolCall.ConvID = convID

		output := tools.ExecuteMCPTool(toolCall, user)
		toolCall.Output = output

		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.TOOL_CALL,
			Payload: toolCall,
		})

		err = toolCalls.Save(&toolCall)
		if err != nil {
			log.Error("Error saving tool call output", "err", err)
		}

		providerParams.Messages = append(providerParams.Messages, providers.SimpleMessage{
			Role:     "assistant",
			ToolCall: toolCall,
		})

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

		calls = calls[1:]
		if len(calls) == 0 {
			completion, err = provider.SendChatCompletionStreamRequest(providerParams, sc)
			if err != nil {
				log.Error("Error streaming chat completion after tool call", "err", err)
				utils.SendStreamChunk(sc, utils.StreamChunk{
					Type:    utils.EVENT_ERROR,
					Payload: err.Error(),
				})
				responseMessage.Error = err.Error()
				responseMessage.Status = "completed"
				break
			}
			calls = append(calls, completion.ToolCalls...)
		}

		// Accumulate reasoning for all tool calls
		if responseMessage.Reasoning != "" || completion.Reasoning != "" {
			responseMessage.Reasoning += "  \n`using tool:" + toolCall.Name + "`  \n" + completion.Reasoning
		}
	}

	// Update assistant message with full content after all tool calls
	if isToolsUsed && err == nil {
		responseMessage.Content += completion.Content
		responseMessage.Status = "completed"
		responseMessage.Speed = completion.Stats.Speed
		responseMessage.TokenCount = completion.Stats.CompletionTokens
		responseMessage.ContextSize = completion.Stats.PromptTokens
		streamStats = completion.Stats
	}

	_, err = updateMessage(responseMessage.ID, responseMessage)
	if err != nil {
		log.Error("Error updating assistant message after tool calls", "err", err)
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

	// Metadata: no new user message; client already knows conversation
	metadata := utils.StreamMetadata{
		ConversationID: req.ConversationID,
		UserMessageID:  parent.ID,
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

	responseMessage.ID, err = saveMessage(responseMessage)
	if err != nil {
		log.Error("Error saving retry response message", "err", err)
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
	if !isToolsUsed {
		responseMessage.Status = "completed"
		responseMessage.Speed = streamStats.Speed
		responseMessage.TokenCount = streamStats.CompletionTokens
		responseMessage.ContextSize = streamStats.PromptTokens
	}

	for len(calls) > 0 {

		toolCall := calls[0]

		providerParams.Messages = append(providerParams.Messages, providers.SimpleMessage{
			Role:     "assistant",
			ToolCall: toolCall,
		})

		toolCall.MessageID = responseMessage.ID
		toolCall.ConvID = req.ConversationID

		output := tools.ExecuteMCPTool(toolCall, user)
		toolCall.Output = output

		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.TOOL_CALL,
			Payload: toolCall,
		})

		err = toolCalls.Save(&toolCall)
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

		calls = calls[1:]
		if len(calls) == 0 {
			completion, err = provider.SendChatCompletionStreamRequest(providerParams, sc)
			if err != nil {
				log.Error("Error streaming chat completion after tool call", "err", err)
				utils.SendStreamChunk(sc, utils.StreamChunk{
					Type:    utils.EVENT_ERROR,
					Payload: err.Error(),
				})
				responseMessage.Error = err.Error()
				responseMessage.Status = "completed"
				break
			}
			calls = append(calls, completion.ToolCalls...)
		}

		// Accumulate reasoning for all tool calls
		if responseMessage.Reasoning != "" || completion.Reasoning != "" {
			responseMessage.Reasoning += "  \n`using tool:" + toolCall.Name + "`  \n" + completion.Reasoning
		}
	}

	// Update assistant message with full content after all tool calls
	if isToolsUsed && err == nil {
		responseMessage.Content = completion.Content
		responseMessage.Status = "completed"
		responseMessage.Speed = completion.Stats.Speed
		responseMessage.TokenCount = completion.Stats.CompletionTokens
		responseMessage.ContextSize = completion.Stats.PromptTokens
		streamStats = completion.Stats
	}

	_, err = updateMessage(responseMessage.ID, responseMessage)
	if err != nil {
		log.Error("Error updating assistant message after tool calls", "err", err)
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

	msg, err := updateMessage(req.MessageID, Message{Content: req.Content})
	if err != nil {
		log.Error("Error updating message", "err", err)
		http.Error(w, fmt.Sprintf("Error updating message: %v", err), http.StatusInternalServerError)
		return
	}

	response := &Response{
		Messages: make(map[int]*Message),
	}
	response.Messages[msg.ID] = msg

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func resumeStream(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	convID := r.URL.Query().Get("convId")
	param := r.URL.Query().Get("msgId")
	msgID, err := strconv.Atoi(param)
	if err != nil || msgID <= 0 {
		log.Error("Invalid parentId query parameter", "err", err)
		http.Error(w, "Invalid parentId query parameter", http.StatusBadRequest)
		return
	}

	sc := utils.StreamClient{
		User:      user,
		MessageID: msgID,
		Writer:    w,
	}

	utils.AddStreamHeaders(sc.Writer)
	_, ok := sc.Writer.(http.Flusher)
	if !ok {
		log.Error("Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	if chunks, found := utils.StreamCache.GetChunks(user, msgID); found && len(chunks) > 0 {
		ch, cancel := utils.StreamCache.Subscribe(user, msgID)
		defer cancel()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-ch:
				if !ok {
					return
				}
				if err := utils.ReplayChunk(sc, chunk); err != nil {
					log.Error("Error writing replayed chunk", "err", err)
					return
				}
			}
		}
	}

	// this should not happen often, because /resume be called while cache has data
	// but if not, this probably means the message is completed already
	time.Sleep(1000 * time.Millisecond)

	conv, err := conversations.GetByID(convID, user)
	if err != nil {
		log.Error("Error getting conversation", "err", err)
		http.Error(w, "Error getting conversation", http.StatusNotFound)
		return
	}

	msg, err := getMessage(msgID)
	if err != nil {
		log.Error("Error getting message", "err", err)
		http.Error(w, "Error getting message", http.StatusNotFound)
		return
	}

	if msg.ConvID != conv.ID || conv.UserID != user || msg.Role != "assistant" {
		log.Error("Message does not belong to user or conversation")
		http.Error(w, "Message does not belong to user or conversation", http.StatusUnauthorized)
		return
	}

	if msg.Status == "completed" {
		metadata := utils.StreamMetadata{
			ConversationID: conv.ID,
			UserMessageID:  msg.ParentID,
		}
		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.EVENT_METADATA,
			Payload: metadata,
		})

		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.REASONING,
			Payload: msg.Reasoning,
		})

		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type:    utils.CONTENT,
			Payload: msg.Content,
		})

		if msg.Error != "" {
			utils.SendStreamChunk(sc, utils.StreamChunk{
				Type:    utils.EVENT_ERROR,
				Payload: msg.Error,
			})
		}

		for _, call := range msg.Tools {
			utils.SendStreamChunk(sc, utils.StreamChunk{
				Type:    utils.TOOL_CALL,
				Payload: call,
			})
		}

		utils.SendStreamChunk(sc, utils.StreamChunk{
			Type: utils.EVENT_COMPLETE,
			Payload: utils.StreamComplete{
				UserMessageID:      msg.ParentID,
				AssistantMessageID: msg.ID,
				StreamStats: utils.StreamStats{
					PromptTokens:     msg.ContextSize,
					CompletionTokens: msg.TokenCount,
					Speed:            msg.Speed,
				},
			},
		})

		return
	}

	http.Error(w, "No active stream found for message", http.StatusNotFound)
}
