package main

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/chat"
	"ai-client/cmd/provider"
	"ai-client/cmd/utils"
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var log = utils.Log

func main() {
	StartServer()
}

func StartServer() {

	fs := http.FileServer(http.Dir("./static"))
	dataFs := http.FileServer(http.Dir("./data"))
	mux := http.NewServeMux()

	mux.Handle("/", fs)
	mux.Handle("/data/", auth.Authenticated(http.StripPrefix("/data/", dataFs)))

	mux.Handle("/api/chat/", chat.Handler())
	mux.Handle("/api/files/", chat.FileHandler())
	mux.Handle("/api/conversations/", chat.ConvsHandler())
	mux.Handle("/api/providers/", provider.Handler())
	mux.Handle("/api/settings/", chat.SettingsHandler())

	mux.Handle("POST /api/logout", auth.Logout())
	mux.Handle("POST /api/login", auth.Login())

	server := &http.Server{
		Addr:         ":8080",
		Handler:      utils.Middleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("Server Failed", "err", err)
		}
	}()

	log.Info("Server started on port 8080")

	<-stop

	log.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown Failed", "err", err)
	}

	log.Info("Server gracefully stopped")
}
