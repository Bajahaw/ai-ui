package providers

import (
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"
)

func TestOpenAIMessageParams_ToolMediaUsesMessageImagesNotFileID(t *testing.T) {
	params := &openai.ChatCompletionNewParams{}
	messages := []SimpleMessage{
		{
			Role: "assistant",
			ToolCall: ToolCall{
				ReferenceID: "call_1",
				Name:        "view_document_page",
				Args:        `{}`,
			},
		},
		{
			Role: "tool",
			ToolCall: ToolCall{
				ReferenceID: "call_1",
				Name:        "view_document_page",
				Output:      "Rendered page",
				FileID:      "file-id-must-not-appear-as-url",
			},
			Images: []string{"data:image/png;base64,abc"},
		},
	}

	OpenAIMessageParams(params, messages)

	if len(params.Messages) != 3 {
		t.Fatalf("expected 3 messages (assistant, tool, user-with-image), got %d", len(params.Messages))
	}

	toolMsg := params.Messages[1]
	if toolMsg.OfTool == nil {
		t.Fatal("expected second message to be tool")
	}
	if toolMsg.OfTool.Content.OfString.Value != "Rendered page" {
		t.Fatalf("tool content: got %q", toolMsg.OfTool.Content.OfString.Value)
	}

	userMsg := params.Messages[2]
	if userMsg.OfUser == nil {
		t.Fatal("expected third message to be user attachment follow-up")
	}
	parts := userMsg.OfUser.Content.OfArrayOfContentParts
	if len(parts) != 2 {
		t.Fatalf("expected text + image parts, got %d", len(parts))
	}
	if parts[1].OfImageURL == nil {
		t.Fatal("expected image_url part")
	}
	url := parts[1].OfImageURL.ImageURL.URL
	if url != "data:image/png;base64,abc" {
		t.Fatalf("image url: got %q", url)
	}
	if strings.Contains(url, "file-id-must-not-appear") {
		t.Fatal("FileID leaked into image URL")
	}
}

func TestOpenAIMessageParams_ToolFilePart(t *testing.T) {
	params := &openai.ChatCompletionNewParams{}
	messages := []SimpleMessage{
		{
			Role: "tool",
			ToolCall: ToolCall{
				ReferenceID: "call_2",
				Name:        "some_tool",
				Output:      "ok",
			},
			Files: []string{"data:application/pdf;base64,xyz"},
		},
	}

	OpenAIMessageParams(params, messages)

	if len(params.Messages) != 2 {
		t.Fatalf("expected tool + user follow-up, got %d", len(params.Messages))
	}
	userMsg := params.Messages[1]
	if userMsg.OfUser == nil {
		t.Fatal("expected user follow-up")
	}
	parts := userMsg.OfUser.Content.OfArrayOfContentParts
	foundFile := false
	for _, p := range parts {
		if p.OfFile != nil {
			foundFile = true
			if p.OfFile.File.FileData.Value != "data:application/pdf;base64,xyz" {
				t.Fatalf("file data: got %q", p.OfFile.File.FileData.Value)
			}
		}
	}
	if !foundFile {
		t.Fatal("expected file content part")
	}
}

func TestOpenAIMessageParams_ToolWithoutMedia(t *testing.T) {
	params := &openai.ChatCompletionNewParams{}
	messages := []SimpleMessage{
		{
			Role: "tool",
			ToolCall: ToolCall{
				ReferenceID: "call_3",
				Name:        "search",
				Output:      "hits",
				FileID:      "", // no media
			},
		},
	}

	OpenAIMessageParams(params, messages)

	if len(params.Messages) != 1 {
		t.Fatalf("expected only tool message, got %d", len(params.Messages))
	}
}

func TestToChatGPTMessages_ToolImagesBecomeUserFollowUp(t *testing.T) {
	out := toChatGPTMessages([]SimpleMessage{
		{
			Role: "tool",
			ToolCall: ToolCall{
				ReferenceID: "c1",
				Name:        "view_document_page",
				Output:      "done",
				FileID:      "keep-as-id-only",
			},
			Images: []string{"data:image/png;base64,zz"},
		},
	})

	if len(out) != 2 {
		t.Fatalf("expected tool + user, got %d", len(out))
	}
	if out[0].Role != "tool" || out[0].ToolOutput != "done" {
		t.Fatalf("tool msg: %+v", out[0])
	}
	if out[0].Images != nil {
		t.Fatal("tool chatgpt message should not carry images")
	}
	if out[1].Role != "user" || len(out[1].Images) != 1 || out[1].Images[0] != "data:image/png;base64,zz" {
		t.Fatalf("user follow-up: %+v", out[1])
	}
}
