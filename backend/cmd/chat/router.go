package chat

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/datasource"
	"ai-client/cmd/utils"
	"net/http"
)

var log = utils.Log
var db = datasource.DB
var repo = newConversationRepository()

func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /new", chat)
	mux.HandleFunc("POST /retry", retry)
	mux.HandleFunc("POST /update", update)

	return http.StripPrefix("/api/chat", auth.Authenticated(mux))
}

func ConvsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET     	/", getAllConversations)
	mux.HandleFunc("POST 	/add", saveConversation)
	mux.HandleFunc("GET  	/{id}", getConversation)
	mux.HandleFunc("DELETE  	/{id}", deleteConversation)
	mux.HandleFunc("POST 	/{id}/rename", renameConversation)
	mux.HandleFunc("GET 	/{id}/messages", getConversationMessages)

	return http.StripPrefix("/api/conversations", auth.Authenticated(mux))
}

func SettingsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET 	/", getAllSettings)
	mux.HandleFunc("POST /update", updateSettings)

	return http.StripPrefix("/api/settings", auth.Authenticated(mux))
}

func FileHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /upload", upload)

	return http.StripPrefix("/api/files", mux)
}
