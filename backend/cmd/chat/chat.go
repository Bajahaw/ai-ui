package chat

import (
	"ai-client/cmd/utils"
	"context"
	"encoding/json"
	"fmt"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"net/http"
	"os"
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

func Chat(w http.ResponseWriter, r *http.Request) {
	var req Request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.ConversationID == "" || req.Content == "" {
		fmt.Println("Error unmarshalling request body:", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	client := openai.NewClient(
		option.WithBaseURL("https://api.groq.com/openai/v1"),
		option.WithAPIKey(os.Getenv("AI_KEY")),
	)

	//get conversation
	conv, err := repo.GetConversation(req.ConversationID)
	if err != nil {
		id := req.ConversationID
		if id == "" {
			// id should be as follows: "conv-20250815-182253" with current date and time
			id = fmt.Sprintf("conv-%s", time.Now().Format("20060102-150405"))
		}
		conv = NewConversation(id)
		if err := repo.AddConversation(conv); err != nil {
			http.Error(w, fmt.Sprintf("Error creating conversation: %v", err), http.StatusInternalServerError)
			return
		}
	}

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
	fmt.Println("Current message ID:", current)
	for {
		leaf, err := conv.GetMessage(current)
		if err != nil {
			break
		}
		path = append(path, current)
		current = leaf.ParentID
	}

	var messages []utils.SimpleMessage
	for i := len(path) - 1; i >= 0; i-- {
		msg, err := conv.GetMessage(path[i])
		if err != nil {
			break
		}
		messages = append(messages, utils.SimpleMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	//debug
	fmt.Println("Path:", path)
	fmt.Println("Messages:", messages)
	//

	params := openai.ChatCompletionNewParams{
		Model:    req.Model,
		Messages: utils.OpenAIMessageParams(messages),
	}

	completion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error generating completion: %v", err), http.StatusInternalServerError)
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
		fmt.Println("Error updating conversation:", err)
		http.Error(w, fmt.Sprintf("Error updating conversation: %v", err), http.StatusInternalServerError)
		return
	}

	response := struct {
		Messages map[int]*Message `json:"messages"`
	}{}
	response.Messages = make(map[int]*Message)
	response.Messages[userMessage.ID], _ = conv.GetMessage(userMessage.ID)
	response.Messages[responseMessage.ID], _ = conv.GetMessage(responseMessage.ID)

	utils.RespondWithJSON(w, response)
}

func AddConversation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Conv Conversation `json:"conversation"`
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		fmt.Println("Error unmarshalling request body:", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	conv := &req.Conv

	// debug
	fmt.Println("Adding conversation:", conv)

	err = repo.AddConversation(conv)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error adding conversation: %v", err), http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, conv)
}

func GetConversation(w http.ResponseWriter, r *http.Request) {
	convId := r.PathValue("id")
	conv, err := repo.GetConversation(convId)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, &conv)
}

func GetAllConversations(w http.ResponseWriter, _ *http.Request) {
	conversations, err := repo.GetAllConversations()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving conversations: %v", err), http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, conversations)
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
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		fmt.Println("Error unmarshalling request body:", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	conv, err := repo.GetConversation(convId)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving conversation: %v", err), http.StatusInternalServerError)
		return
	}

	conv.Title = req.Title

	err = repo.UpdateConversation(conv)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error updating conversation: %v", err), http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, conv)
}
