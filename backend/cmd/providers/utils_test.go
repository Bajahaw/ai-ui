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

// When a tool file resolves to text only (agentic retrieval, or too large to
// inline), the resolved metadata/content must still reach the model: it is
// appended to the tool result since no media follow-up message is emitted.
func TestOpenAIMessageParams_ToolWithoutMediaAppendsResolvedContent(t *testing.T) {
	params := &openai.ChatCompletionNewParams{}
	OpenAIMessageParams(params, []SimpleMessage{
		{
			Role:    "tool",
			Content: "[tool attachment: \nname: report.xlsx\ncontent: Document content page 1: January 5000\n]\n",
			ToolCall: ToolCall{
				ReferenceID: "call_5",
				Name:        "browser_sandbox",
				Output:      "ok: true",
			},
		},
	})
	if len(params.Messages) != 1 {
		t.Fatalf("expected only tool message, got %d", len(params.Messages))
	}
	got := params.Messages[0].OfTool.Content.OfString.Value
	if !strings.HasPrefix(got, "ok: true") || !strings.Contains(got, "January 5000") {
		t.Fatalf("tool content should carry output and resolved content, got %q", got)
	}
}

func TestToChatGPTMessages_ToolWithoutMediaAppendsResolvedContent(t *testing.T) {
	out := toChatGPTMessages([]SimpleMessage{
		{
			Role:    "tool",
			Content: "[tool attachment: \ncontent: page one text\n]\n",
			ToolCall: ToolCall{
				ReferenceID: "c2",
				Name:        "browser_sandbox",
				Output:      "ok: true",
			},
		},
	})
	if len(out) != 1 {
		t.Fatalf("expected only tool message, got %d", len(out))
	}
	if !strings.HasPrefix(out[0].ToolOutput, "ok: true") || !strings.Contains(out[0].ToolOutput, "page one text") {
		t.Fatalf("tool output: %q", out[0].ToolOutput)
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

func TestOpenAIMessageParams_ToolFollowUpUsesRequestMetadata(t *testing.T) {
	params := &openai.ChatCompletionNewParams{}
	OpenAIMessageParams(params, []SimpleMessage{
		{
			Role:    "tool",
			Content: "[tool attachment: \npath: /data/resources/a.xlsx\n]\n",
			ToolCall: ToolCall{
				ReferenceID: "call_4",
				Name:        "officecli",
				Output:      "created",
			},
			Files: []string{"data:application/octet-stream;base64,xx"},
		},
	})
	if len(params.Messages) != 2 {
		t.Fatalf("expected tool + user follow-up, got %d", len(params.Messages))
	}
	parts := params.Messages[1].OfUser.Content.OfArrayOfContentParts
	if parts[0].OfText == nil || !strings.Contains(parts[0].OfText.Text, "path: /data/resources/a.xlsx") {
		t.Fatalf("follow-up text: %+v", parts[0])
	}
	if strings.Contains(params.Messages[0].OfTool.Content.OfString.Value, "[tool attachment:") {
		t.Fatal("stored tool output leaked attachment metadata")
	}
}
