package provider

import (
	"ai-client/cmd/utils"
	"context"
	"time"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

var log = utils.GetLogger()

type SimpleMessage struct {
	Role    string
	Content string
	Image   string
}

func SendChatCompletionRequest(messages []SimpleMessage, model string) (*openai.ChatCompletion, error) {
	providerID, model := utils.ExtractProviderID(model)
	provider, err := repo.getProvider(providerID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client := openai.NewClient(
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	)

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: OpenAIMessageParams(messages),
		//Tools: []openai.ChatCompletionToolUnionParam{
		//	{
		//		OfCustom: &openai.ChatCompletionCustomToolParam{
		//			Type: "browser_search",
		//		},
		//	},
		//},
	}

	//
	log.Debug("Sending chat completion request", "params", params)

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
						OfArrayOfContentParts: []openai.ChatCompletionContentPartUnionParam{
							{
								OfText: &openai.ChatCompletionContentPartTextParam{
									Text: msg.Content,
								},
							},
						},
					},
				},
			}
			if msg.Image != "" {
				file := openai.ChatCompletionContentPartUnionParam{
					OfImageURL: &openai.ChatCompletionContentPartImageParam{
						ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
							URL: msg.Image,
						},
					},
				}
				openaiMessages[i].OfUser.Content.OfArrayOfContentParts =
					append(openaiMessages[i].OfUser.Content.OfArrayOfContentParts, file)
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
