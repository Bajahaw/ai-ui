package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/auth"
	"github.com/Bajahaw/ai-ui/cmd/chat"
	"github.com/Bajahaw/ai-ui/cmd/data"
	"github.com/Bajahaw/ai-ui/cmd/files"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/secrets"
	"github.com/Bajahaw/ai-ui/cmd/settings"
	"github.com/Bajahaw/ai-ui/cmd/skills"
	"github.com/Bajahaw/ai-ui/cmd/tools"
	"github.com/Bajahaw/ai-ui/cmd/utils"
	"github.com/Bajahaw/ai-ui/cmd/version"

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
	setupSkills()
	setupSecrets()

	startServer()
}

func setupEnv() {
	err := godotenv.Load("./.env")
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

func setupSkills() {
	skills.SetupSkills(log, db)
	log.Info("Skills set up successfully")
}

func setupSecrets() {
	secrets.SetupSecrets(log, db)
	log.Info("Secrets set up successfully")
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
	// if os.Getenv("JWT_SECRET") == "" {
	// 	log.Fatal("JWT_SECRET environment variable is required")
	// }

	auth.Setup(log, db)
	auth.OnRegister = []auth.PostRegisterHook{
		settings.SetDefaults,
		tools.SaveDefaultMCPServer,
	}
}

// spaHandler serves static files and falls back to index.html for unknown
// paths so that client-side routes work on hard refresh.
type spaHandler struct {
	staticDir string
	fs        http.Handler
}

// serveShell writes index.html directly. http.ServeFile redirects any request
// path ending in "/index.html" to "./", and a redirected response handed to a
// navigation request by the service worker makes the browser fail the load
// with ERR_FAILED, so the shell is always served on a path that can't trigger
// that special case.
func (h spaHandler) serveShell(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	shellReq := r.Clone(r.Context())
	shellReq.URL = new(url.URL)
	*shellReq.URL = *r.URL
	shellReq.URL.Path = "/"
	http.ServeFile(w, shellReq, filepath.Join(h.staticDir, "index.html"))
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		h.serveShell(w, r)
		return
	}

	staticAbs, err := filepath.Abs(h.staticDir)
	if err != nil {
		h.serveShell(w, r)
		return
	}

	cleanReqPath := filepath.Clean("/" + r.URL.Path)
	cleanReqPath = strings.TrimPrefix(cleanReqPath, string(filepath.Separator))

	requestedPath, err := filepath.Abs(filepath.Join(staticAbs, cleanReqPath))
	if err != nil || (requestedPath != staticAbs && !strings.HasPrefix(requestedPath, staticAbs+string(filepath.Separator))) {
		// Invalid or escaping static dir – serve SPA shell
		h.serveShell(w, r)
		return
	}

	if _, err := os.Stat(requestedPath); os.IsNotExist(err) {
		// Not a real file – serve the SPA shell
		h.serveShell(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/assets/") {
		// Vite emits content-hashed filenames under /assets, safe to cache forever.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		// sw.js, workbox-*.js, sandbox.html, manifest/icons: always revalidate so a
		// new deploy is picked up on the next load instead of after a stale TTL.
		w.Header().Set("Cache-Control", "no-cache")
	}
	h.fs.ServeHTTP(w, r)
}

func startServer() {

	staticDir := "./static"
	rawFs := http.FileServer(http.Dir(staticDir))
	dataFs := http.FileServer(http.Dir("./data/resources"))
	mux := http.NewServeMux()

	mux.Handle("/", spaHandler{staticDir: staticDir, fs: rawFs})
	mux.Handle("/data/resources/",
		http.StripPrefix(
			"/data/resources/",
			auth.Authenticated(files.ResourcesHandler(dataFs)),
		))

	mux.Handle("/api/chat/", chat.Handler())
	mux.Handle("/api/files/", files.FileHandler())
	mux.Handle("/api/conversations/", chat.ConvsHandler())
	mux.Handle("/api/providers/", providers.Handler())
	mux.Handle("/api/models/", providers.ModelsHandler())
	mux.Handle("/api/tts/", chat.TTSHandler())
	mux.Handle("/api/settings/", settings.SettingsHandler())
	mux.Handle("/api/tools/", tools.Handler())
	mux.Handle("/api/skills/", skills.Handler())
	mux.Handle("/api/secrets/", secrets.Handler())
	mux.Handle("/api/auth/", auth.Handler())
	mux.HandleFunc("/api/version", version.HandleGetVersion)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      utils.Middleware(mux),
		ReadTimeout:  30 * time.Minute,
		WriteTimeout: 0, // 0 = no timeout; required for SSE connections
		IdleTimeout:  30 * time.Minute,
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
