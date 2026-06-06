package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/utils"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

func generateImageTool(args string, user string, convID string) providers.ToolOutput {
	var params struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding arguments: %v", err)}
	}

	if params.Prompt == "" {
		return providers.ToolOutput{Content: "error: prompt is required"}
	}

	imageModel, err := settings.Get("imageModel", user)
	if err != nil || imageModel == "" {
		imageModel = "dall-e-3"
	}

	providerID, modelName := utils.ExtractProviderID(imageModel)

	provider, err := providerRepo.GetByID(providerID, user)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("Error querying provider %s for imageModel: %v. Please select a valid Image Model in settings.", providerID, err)}
	}

	opts := []option.RequestOption{
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	}
	for key, value := range provider.Headers {
		opts = append(opts, option.WithHeader(key, value))
	}

	client := openai.NewClient(opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	res, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: modelName,
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(params.Prompt),
		},
	}, opts...)

	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error generating image: %v", err)}
	}

	if len(res.Output) == 0 {
		return providers.ToolOutput{Content: "error generating image: no image returned"}
	}

	result := res.Output[0].Result
	if result == "" {
		return providers.ToolOutput{Content: "error generating image: empty image data"}
	}

	mimeType := "image/png"
	encoded := result
	if prefix, data, found := strings.Cut(result, ","); found && strings.HasPrefix(prefix, "data:") {
		encoded = data
		if strings.HasPrefix(prefix, "data:image/jpeg") || strings.HasPrefix(prefix, "data:image/jpg") {
			mimeType = "image/jpeg"
		} else if strings.HasPrefix(prefix, "data:image/webp") {
			mimeType = "image/webp"
		} else if strings.HasPrefix(prefix, "data:image/png") {
			mimeType = "image/png"
		}
	}

	imgBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error decoding image: %v", err)}
	}

	ext := ".png"
	switch mimeType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	case "image/png":
		ext = ".png"
	}

	fileName := fmt.Sprintf("generated_image_%s%s", time.Now().Format("20060102_150405"), ext)
	fileData, err := saveGeneratedFile(imgBytes, fileName, user)
	if err != nil {
		return providers.ToolOutput{Content: fmt.Sprintf("error saving image: %v", err)}
	}

	return providers.ToolOutput{
		Content: fmt.Sprintf("Image generated successfully. File ID: %s Name: %s Path: %s",
			fileData.ID, fileData.Name, fileData.Path,
		),
	}
}
