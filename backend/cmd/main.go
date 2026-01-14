package main

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/chat"
	"ai-client/cmd/data"
	"ai-client/cmd/files"
	"ai-client/cmd/providers"
	"ai-client/cmd/settings"
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
var provider providers.Client

func main() {
	setupEnv()
	setupLogger()
	setupUtils()

	startDataSource()

	setupAuth()
	setupProviderClient()
	setupSettings()
	setupFiles()
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
	providers.SetupProviderClient(log, db)
	provider = providers.NewClient()
	log.Info("Provider client set up successfully")
}

func setupChatClient() {
	chat.SetupChat(log, db, provider)
	log.Info("Chat client set up successfully")
}

func setupSettings() {
	settings.SetupSettings(log, db)
	log.Info("Settings set up successfully")
}

func setupFiles() {
	files.SetupFiles(log, db, provider)
	log.Info("Files set up successfully")
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
	auth.Setup(log, db)
	auth.OnRegister = []auth.PostRegisterHook{
		settings.SetDefaults,
		tools.SaveDefaultMCPServer,
	}
}

func startServer() {

	fs := http.FileServer(http.Dir("./static"))
	dataFs := http.FileServer(http.Dir("./data/resources"))
	mux := http.NewServeMux()

	mux.Handle("/", fs)
	mux.Handle("/data/resources/", http.StripPrefix("/data/resources/", auth.Authenticated(dataFs)))

	mux.Handle("/api/chat/", chat.Handler())
	mux.Handle("/api/files/", files.FileHandler())
	mux.Handle("/api/conversations/", chat.ConvsHandler())
	mux.Handle("/api/providers/", providers.Handler())
	mux.Handle("/api/models/", providers.ModelsHandler())
	mux.Handle("/api/settings/", settings.SettingsHandler())
	mux.Handle("/api/tools/", tools.Handler())
	mux.Handle("/api/auth/", auth.Handler())

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
