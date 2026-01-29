package chat

import (
	"ai-client/cmd/auth"
	"net/http"
)

func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /stream", chatStream)
	mux.HandleFunc("POST /retry/stream", retryStream)
	mux.HandleFunc("POST /update", update)
	mux.HandleFunc("GET /resume", resumeStream)
	// mux.HandleFunc("POST /new", chat) // Temporarily disabled, use /stream instead
	// mux.HandleFunc("POST /retry", retry)

	return http.StripPrefix("/api/chat", auth.Authenticated(mux))
}

func ConvsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET     /", getAllConversations)
	mux.HandleFunc("POST 	/add", saveConversation)
	mux.HandleFunc("GET  	/{id}", getConversation)
	mux.HandleFunc("DELETE  /{id}", deleteConversation)
	mux.HandleFunc("POST 	/{id}/rename", renameConversation)
	mux.HandleFunc("GET 	/{id}/messages", getConversationMessages)

	return http.StripPrefix("/api/conversations", auth.Authenticated(mux))
}
