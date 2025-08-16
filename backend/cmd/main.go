package main

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/chat"
	"ai-client/cmd/provider"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func main() {
	StartServer()
}

// TODO: Clean up before production use

func StartServer() {

	fs := http.FileServer(http.Dir("./static"))
	mux := http.NewServeMux()

	mux.Handle("/", fs)
	mux.Handle("/api/chat/", http.StripPrefix("/api/chat", chat.Handler()))
	mux.Handle("/api/conversations/", http.StripPrefix("/api/conversations", chat.ConvsHandler()))
	mux.Handle("/api/providers/", http.StripPrefix("/api/providers", provider.Handler()))
	mux.Handle("POST /api/logout", auth.Logout())
	mux.Handle("POST /api/login", auth.Login())

	server := &http.Server{
		Addr:         ":8080",
		Handler:      logMiddleware(mux),
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

func (r *statusRecorder) writeHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		log.Printf("Received request: %d %s %s", recorder.status, r.Method, r.URL.Path)
	})
}
