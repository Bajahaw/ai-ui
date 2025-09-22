package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/jsonschema"
	logger "github.com/charmbracelet/log"
	"github.com/google/uuid"
)

var Log = logger.NewWithOptions(os.Stdout, logger.Options{
	Level:           loglevel,
	ReportTimestamp: true,
})

var ServerURL = ""

var loglevel = func() logger.Level {
	if os.Getenv("ENV") == "dev" {
		fmt.Println("--- Development mode: setting log level to DEBUG ---")
		return logger.DebugLevel
	}
	fmt.Println("--- Production mode: setting log level to INFO ---")
	return logger.InfoLevel
}()

//////////////////////////////////////////////////////////////////////////////////
//////////////////////////////// Helper Functions ////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////

// corsMiddleware currently used for local vite server
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ExtractBody(r *http.Request) ([]byte, error) {
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Error closing request body:", err)
		}
	}(r.Body)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func ExtractJSONBody(r *http.Request, v interface{}) error {
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

func RespondWithJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")

	buf, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "failed to encode JSON", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(statusCode)
	_, err = w.Write(buf)
	if err != nil {
		log.Println("failed to write response:", err)
	}
}

func Structure(t interface{}) string {
	reflector := jsonschema.Reflector{}
	schema := reflector.Reflect(t)
	str, _ := json.MarshalIndent(schema, "", "  ")
	//fmt.Println("Structure:", string(str))
	return string(str)
}

func ExtractProviderID(model string) (string, string) {
	// Example: "provider-5dx6/whisper-large-v3-turbo" -> "provider-5dx6", "whisper-large-v3-turbo"
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

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		var level logger.Level
		switch {
		case recorder.status >= 500:
			level = logger.ErrorLevel
		case recorder.status >= 400:
			level = logger.WarnLevel
		default:
			level = logger.InfoLevel
		}
		Log.Log(level, "Received request", "status", recorder.status, "method", r.Method, "path", r.URL.Path)
	})
}

func Middleware(next http.Handler) http.Handler {
	middlewares := []func(http.Handler) http.Handler{}
	middlewares = append(middlewares, logMiddleware)

	if os.Getenv("ENV") == "dev" {
		Log.Debug("Development mode CORS active")
		middlewares = append(middlewares, corsMiddleware)
	}

	for _, m := range middlewares {
		next = m(next)
	}
	return next
}

func SaveUploadedFile(file multipart.File, handler *multipart.FileHeader) (string, error) {
	const maxUploadSize = 10 << 20 // 10 MB
	defer file.Close()

	if handler.Size > 0 && handler.Size > maxUploadSize {
		return "", fmt.Errorf("file too large: %d bytes (max %d)", handler.Size, maxUploadSize)
	}

	uploadDir := filepath.Join(".", "data", "resources")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", err
	}

	fileName := uuid.New().String() + filepath.Ext(handler.Filename)
	filePath := filepath.Join(uploadDir, fileName)

	dst, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	limitedReader := io.LimitReader(file, maxUploadSize+1)
	n, err := io.Copy(dst, limitedReader)
	if err != nil {
		_ = os.Remove(filePath)
		return "", err
	}
	if n > maxUploadSize {
		_ = os.Remove(filePath)
		return "", fmt.Errorf("file too large after copy: %d bytes (max %d)", n, maxUploadSize)
	}

	return filePath, nil
}

func GetServerURL(r *http.Request) string {
	if ServerURL != "" {
		return ServerURL
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	ServerURL = scheme + "://" + r.Host
	return ServerURL
}

func ExtractProviderName(url string) string {
	// "https://api.openai.com/v1" -> "openai"
	parts := strings.Split(url, ".")
	if len(parts) < 2 {
		return "provider"
	}
	name := parts[len(parts)-2]
	return name
}

func ExtractModelName(id string) string {
	// "openai/gpt-4-turbo" -> "gpt-4-turbo"
	parts := strings.Split(id, "/")
	if len(parts) < 2 {
		return id
	}
	return parts[len(parts)-1]
}
