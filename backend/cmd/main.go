package main

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/chat"
	"ai-client/cmd/data"
	"ai-client/cmd/provider"
	"ai-client/cmd/tools"
	"ai-client/cmd/utils"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	logger "github.com/charmbracelet/log"
	"github.com/joho/godotenv"
)

var log *logger.Logger
var db *sql.DB
var providerClient *provider.Client

func main() {
	setupEnv()
	setupLogger()
	setupUtils()

	startDataSource()

	setupAuth()
	setupProviderClient()
	setupChatClient()
	setupTools()

	startServer()
}

func setupEnv() {
	err := godotenv.Load("../.env")
	if err != nil {
		fmt.Println("No .env file found, proceeding with system environment variables")
	}
}

func setupLogger() {
	log = logger.NewWithOptions(os.Stdout, logger.Options{
		ReportTimestamp: true,
	})

	env := os.Getenv("ENV")
	if env == "dev" {
		log.SetLevel(logger.DebugLevel)
		fmt.Println("--- Development mode: setting log level to DEBUG ---")
	} else {
		log.SetLevel(logger.InfoLevel)
		fmt.Println("--- Production mode: setting log level to INFO ---")
	}
}

func setupTools() {
	tools.SetUpTools(log, db)
	log.Info("Tools set up successfully")
}

func setupUtils() {
	utils.Setup(log)
	log.Info("Utils set up successfully")
}

func setupProviderClient() {
	provider.SetupProviderClient(log, db)
	providerClient = &provider.Client{}
	log.Info("Provider client set up successfully")
}

func setupChatClient() {
	chat.SetupChat(log, db, providerClient)
	log.Info("Chat client set up successfully")
}

func startDataSource() {
	err := data.InitDataSource("./data/ai-ui.db")
	if err != nil {
		log.Fatal("Failed to initialize data source", "err", err)
	}
	db = data.DB
	log.Info("Data source initialized successfully")
}

func setupAuth() {
	auth.Setup(log)
}

func startServer() {

	fs := http.FileServer(http.Dir("./static"))
	dataFs := http.FileServer(http.Dir("./data/resources"))
	mux := http.NewServeMux()

	mux.Handle("/", fs)
	mux.Handle("/data/resources/", http.StripPrefix("/data/resources/", dataFs))

	mux.Handle("/api/chat/", chat.Handler())
	mux.Handle("/api/files/", chat.FileHandler())
	mux.Handle("/api/conversations/", chat.ConvsHandler())
	mux.Handle("/api/providers/", provider.Handler())
	mux.Handle("/api/models/", provider.ModelsHandler())
	mux.Handle("/api/settings/", chat.SettingsHandler())
	mux.Handle("/api/tools/", tools.Handler())

	mux.Handle("POST /api/logout", auth.Logout())
	mux.Handle("POST /api/login", auth.Login())

	server := &http.Server{
		Addr:         ":8080",
		Handler:      utils.Middleware(mux),
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
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
