// Command ai-ui-app wraps the AI UI HTTP backend in a Wails v3 shell so it can
// ship as a desktop app and as an Android APK from one codebase.
//
// The embedded React UI is served by the Wails asset server. The JSON API is
// served by a loopback HTTP server (127.0.0.1:8080) running in the same
// process: on Android the WebView asset bridge only supports GET requests
// without a body, so POST/PUT/DELETE APIs cannot go through it. The frontend
// (see frontend/src/lib/wails-loopback.ts) redirects /api/* and /data/*
// fetch/EventSource calls to the loopback server when running on
// wails.localhost. <img>/<video> tags keep using relative /data/* URLs,
// which the asset middleware serves from disk (GET works fine there).
package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
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
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// loopbackAddr is the in-process API server. The frontend hardcodes the same
// address (frontend/src/lib/wails-loopback.ts) when on wails.localhost.
const loopbackAddr = "127.0.0.1:8080"

var log *logger.Logger
var db *sql.DB
var provider providers.Client

func main() {
	if runtime.GOOS == "android" {
		chdirToWritableDir()
	}
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

	loopback := startLoopbackServer()

	app := application.New(application.Options{
		Name:        "AI UI",
		Description: "Lightweight AI chat interface",
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: assetMiddleware(),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "AI UI",
		Width:            1000,
		Height:           618,
		BackgroundColour: application.NewRGB(25, 25, 26),
		URL:              "/",
	})

	app.OnShutdown(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = loopback.Shutdown(ctx)
	})

	if err := app.Run(); err != nil {
		log.Fatal("Application failed", "err", err)
	}
}

// chdirToWritableDir moves the process into a writable app directory on
// Android so the existing relative ./data/... paths keep working unchanged.
func chdirToWritableDir() {
	candidates := []string{}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, "ai-ui"))
	}
	candidates = append(candidates,
		"/data/user/0/com.wails.app/files/ai-ui",
		"/data/data/com.wails.app/files/ai-ui",
		filepath.Join(os.TempDir(), "ai-ui"),
	)
	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		probe, err := os.CreateTemp(dir, ".writetest-*")
		if err != nil {
			continue
		}
		name := probe.Name()
		_ = probe.Close()
		_ = os.Remove(name)
		if err := os.Chdir(dir); err == nil {
			fmt.Println("AI UI data dir:", dir)
			return
		}
	}
	fmt.Println("WARNING: no writable data dir found, staying in", mustGetwd())
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// buildMux registers the same routes as the standalone backend server,
// minus the static SPA handler (the Wails asset server serves the UI).
func buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("/data/resources/", resourcesRoute())
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

	return mux
}

func resourcesRoute() http.Handler {
	dataFs := http.FileServer(http.Dir("./data/resources"))
	return http.StripPrefix(
		"/data/resources/",
		auth.Authenticated(files.ResourcesHandler(dataFs)),
	)
}

// startLoopbackServer serves the API on 127.0.0.1 for the WebView frontend.
func startLoopbackServer() *http.Server {
	handler := loopbackCORS(utils.Middleware(buildMux()))
	server := &http.Server{
		Addr:         loopbackAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Minute,
		WriteTimeout: 0, // 0 = no timeout; required for SSE connections
		IdleTimeout:  30 * time.Minute,
	}
	go func() {
		log.Info("Loopback API server started", "addr", loopbackAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("Loopback server failed", "err", err)
		}
	}()
	return server
}

// loopbackCORS allows the wails.localhost WebView origin to call the loopback
// API. The server only listens on 127.0.0.1, so this is not externally reachable.
func loopbackCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// assetMiddleware serves user files for <img>/<video> tags (plain GETs work
// through the asset bridge) and adds SPA fallback for client-side routes.
func assetMiddleware() application.Middleware {
	res := resourcesRoute()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := r.URL.Path
			if strings.HasPrefix(p, "/data/resources/") {
				res.ServeHTTP(w, r)
				return
			}
			if p == "/wails/" || strings.HasPrefix(p, "/wails/") {
				next.ServeHTTP(w, r)
				return
			}
			// Client-side routes (/c/:id) have no file extension; serve the
			// SPA shell for them. The asset server has no fallback of its own.
			if p != "/" && !strings.Contains(path.Base(p), ".") {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/index.html"
				next.ServeHTTP(w, r2)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func setupEnv() {
	err := godotenv.Load("./.env")
	if err != nil {
		fmt.Println("No .env file found, proceeding with system environment variables")
	}
	// The app binary is a local single-device app: passwordless profiles
	// unless explicitly overridden (e.g. AUTH_MODE=password in .env).
	if os.Getenv("AUTH_MODE") == "" {
		os.Setenv("AUTH_MODE", "profiles")
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
	auth.Setup(log, db)
	auth.OnRegister = []auth.PostRegisterHook{
		settings.SetDefaults,
		tools.SaveDefaultMCPServer,
	}
}
