package provider

import (
	"ai-client/cmd/utils"
	"context"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"log"
	"time"
)

type SimpleMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var client = openai.NewClient()

func SendChatCompletionRequest(messages []SimpleMessage, model string) (*openai.ChatCompletion, error) {
	providerID, model := utils.ExtractProviderID(model)
	provider, err := repo.getProvider(providerID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client.Options = []option.RequestOption{
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	}

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: OpenAIMessageParams(messages),
	}

	completion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}
	return completion, nil
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
