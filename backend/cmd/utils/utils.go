package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	url2 "net/url"
	"os"
	"strings"
	"time"

	logger "github.com/charmbracelet/log"
)

var log *logger.Logger
var ServerURL = ""

func Setup(l *logger.Logger) {
	log = l
}

//////////////////////////////////////////////////////////////////////////////////
//////////////////////////////// Helper Functions ////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////

// corsMiddleware currently used for local vite server
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ExtractJSONBody(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err != nil {
		return err
	}
	if err := r.Body.Close(); err != nil {
		return err
	}
	return nil
}

func RespondWithJSON(w http.ResponseWriter, data any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")

	buf, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "failed to encode JSON", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(statusCode)
	_, err = w.Write(buf)
	if err != nil {
		log.Error("failed to write response:", err)
	}
}

// func Structure(t any) string {
// 	reflector := jsonschema.Reflector{}
// 	schema := reflector.Reflect(t)
// 	str, _ := json.MarshalIndent(schema, "", "  ")
// 	//fmt.Println("Structure:", string(str))
// 	return string(str)
// }

func ExtractProviderID(model string) (string, string) {
	// Example: "provider-5dx6/whisper-large-v3-turbo" -> "provider-5dx6", "whisper-large-v3-turbo"
	// provider-1234/open-ai/gpt-4 -> "provider-1234", "open-ai/gpt-4"
	parts := strings.Split(model, "/")
	if len(parts) < 2 {
		return "", ""
	}

	provider := parts[0]
	name := strings.TrimPrefix(model, provider+"/")

	return provider, name
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher to support streaming responses
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		elapsed := time.Since(start)
		durationStr := fmt.Sprintf("%.2fms", float64(elapsed.Microseconds())/1000)

		var level logger.Level
		switch {
		case recorder.status >= 500:
			level = logger.ErrorLevel
		case recorder.status >= 400:
			level = logger.WarnLevel
		default:
			level = logger.InfoLevel
		}
		log.Log(level, "Received request",
			"status", recorder.status,
			"method", r.Method,
			"duration", durationStr,
			"path", r.URL.Path,
		)
	})
}

func Middleware(next http.Handler) http.Handler {
	var middlewares []func(http.Handler) http.Handler

	if os.Getenv("ENV") == "dev" {
		log.Debug("Development mode CORS active")
		middlewares = append(middlewares, corsMiddleware)
	}

	middlewares = append(middlewares, logMiddleware)

	for _, m := range middlewares {
		next = m(next)
	}
	return next
}

func GetServerURL(r *http.Request) string {
	if ServerURL != "" {
		return ServerURL
	}
	scheme := "https"
	if os.Getenv("ENV") == "dev" {
		scheme = "http"
	}
	// if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
	// 	scheme = "https"
	// }
	ServerURL = scheme + "://" + r.Host
	return ServerURL
}

func ExtractProviderName(url string) string {
	// "https://api.openai.com/v1" -> "openai"
	parsed, err := url2.Parse(url)
	if err != nil || parsed.Host == "" {
		return "provider"
	}
	host := parsed.Hostname()
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return "provider"
	}
	name := parts[len(parts)-2]
	return name
}

func SqlPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	placeholders := make([]string, n)
	for i := 0; i < n; i++ {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}

func ExtractContextUser(r *http.Request) string {
	user := r.Context().Value("user").(string)
	return user
}
