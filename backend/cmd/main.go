package main

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/chat"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	StartServer()
}

// TODO: Clean up before production use

func StartServer() {

	fs := http.FileServer(http.Dir("./static"))
	mux := http.NewServeMux()

	mux.Handle("/", fs)
	mux.HandleFunc("POST 	  /api/chat", auth.Authenticated(chat.Chat))
	mux.HandleFunc("GET  	  /api/conversations", auth.Authenticated(chat.GetAllConversations))
	mux.HandleFunc("POST 	  /api/conversations/add", auth.Authenticated(chat.AddConversation))
	mux.HandleFunc("GET  	  /api/conversations/{id}", auth.Authenticated(chat.GetConversation))
	mux.HandleFunc("DELETE 	  /api/conversations/{id}", auth.Authenticated(chat.DeleteConversation))
	mux.HandleFunc("POST 	  /api/conversations/{id}/rename", auth.Authenticated(chat.RenameConversation))
	mux.HandleFunc("POST 	  /api/logout", auth.Authenticated(auth.Logout))
	mux.HandleFunc("POST 	  /api/login", auth.Login)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server Failed: %v", err)
		}
	}()

	log.Println("Server started on port 8080")

	<-stop

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed: %v", err)
	}

	log.Println("Server gracefully stopped")
}
