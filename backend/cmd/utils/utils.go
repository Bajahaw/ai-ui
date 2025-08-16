package utils

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/jsonschema"
	"github.com/openai/openai-go"
	"io"
	"log"
	"net/http"
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
	err := json.NewDecoder(r.Body).Decode(v)
	if err != nil {
		return fmt.Errorf("error decoding JSON body: %w", err)
	}
	if err := r.Body.Close(); err != nil {
		return fmt.Errorf("error closing request body: %w", err)
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

type SimpleMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func OpenAIMessageParams(messages []SimpleMessage) []openai.ChatCompletionMessageParamUnion {
	openaiMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	for i, msg := range messages {
		if msg.Role == "system" {
			openaiMessages[i] = openai.ChatCompletionMessageParamUnion{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: openai.String(msg.Content),
					},
				},
			}
		} else if msg.Role == "user" {
			openaiMessages[i] = openai.ChatCompletionMessageParamUnion{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(msg.Content),
					},
				},
			}
		} else if msg.Role == "assistant" {
			openaiMessages[i] = openai.ChatCompletionMessageParamUnion{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(msg.Content),
					},
				},
			}
		} else {
			log.Printf("Unknown role %s in message, skipping", msg.Role)
			continue
		}
	}
	return openaiMessages
}
