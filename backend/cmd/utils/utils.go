package utils

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/jsonschema"
	"io"
	"log"
	"net/http"
	"strings"
)

//////////////////////////////////////////////////////////////////////////////////
//////////////////////////////// Helper Functions ////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////

// CORS currently used for local vite server
func CORS(next http.Handler) http.Handler {
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
	// Example: "provider-20250817-025038/whisper-large-v3-turbo" -> "provider-20250817-025038"
	parts := strings.Split(model, "/")
	if len(parts) < 2 {
		return "", ""
	}

	provider := parts[0]
	name := strings.TrimPrefix(model, provider+"/")

	return provider, name
}
