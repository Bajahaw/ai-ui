package tools

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
	logger "github.com/charmbracelet/log"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func setupMCPResultTest(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Chdir(tmp)

	dbPath := path.Join(tmp, "test.db")
	if err := data.InitDataSource(dbPath); err != nil {
		t.Fatalf("InitDataSource: %v", err)
	}
	t.Cleanup(func() { _ = data.DB.Close() })

	if _, err := data.DB.Exec("INSERT INTO Users (username, pass_hash) VALUES (?, ?)", "u1", "x"); err != nil {
		t.Fatal(err)
	}
	SetUpTools(logger.New(os.Stderr), data.DB)
}

func TestParseMCPToolResult_TextOnly(t *testing.T) {
	setupMCPResultTest(t)

	out := parseMCPToolResult(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "hello"},
			&mcp.TextContent{Text: "world"},
		},
	}, "u1")

	if out.Content != "hello\nworld" {
		t.Fatalf("content: got %q", out.Content)
	}
	if out.FileID != "" {
		t.Fatalf("expected no file, got %q", out.FileID)
	}
}

func TestParseMCPToolResult_ImageSetsFileID(t *testing.T) {
	setupMCPResultTest(t)

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	out := parseMCPToolResult(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "here is a chart"},
			&mcp.ImageContent{Data: png, MIMEType: "image/png"},
		},
	}, "u1")

	if !strings.Contains(out.Content, "here is a chart") {
		t.Fatalf("missing text: %q", out.Content)
	}
	if out.FileID == "" {
		t.Fatal("expected FileID for image")
	}
	got, err := files.GetByIDs([]string{out.FileID}, "u1")
	if err != nil || len(got) != 1 {
		t.Fatalf("saved file lookup: %v %#v", err, got)
	}
	if got[0].Type != "image/png" {
		t.Fatalf("mime: got %q", got[0].Type)
	}
	if strings.Contains(out.Content, "[tool attachment:") {
		t.Fatalf("tool output must not include attachment metadata: %q", out.Content)
	}
	data, err := os.ReadFile(got[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(png) {
		t.Fatal("saved bytes mismatch")
	}
}

func TestParseMCPToolResult_MultipleBinariesKeepsFirstFileID(t *testing.T) {
	setupMCPResultTest(t)

	out := parseMCPToolResult(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{Data: []byte("img1"), MIMEType: "image/png"},
			&mcp.ImageContent{Data: []byte("img2"), MIMEType: "image/jpeg"},
		},
	}, "u1")

	if out.FileID == "" {
		t.Fatal("expected FileID")
	}
	if strings.Contains(out.Content, "[tool attachment:") {
		t.Fatalf("tool output must not include attachment metadata: %q", out.Content)
	}
	// First image should be the FileID; content should mention a second save.
	first, _ := files.GetByIDs([]string{out.FileID}, "u1")
	if len(first) != 1 || first[0].Type != "image/png" {
		t.Fatalf("first file should be png: %#v", first)
	}
}

func TestParseMCPToolResult_EmbeddedResourceTextAndBlob(t *testing.T) {
	setupMCPResultTest(t)

	out := parseMCPToolResult(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "file://report.txt",
					MIMEType: "text/plain",
					Text:     "report body",
				},
			},
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "file://doc.pdf",
					MIMEType: "application/pdf",
					Blob:     []byte("%PDF-1.4"),
				},
			},
		},
	}, "u1")

	if !strings.Contains(out.Content, "report body") {
		t.Fatalf("missing embedded text: %q", out.Content)
	}
	if out.FileID == "" {
		t.Fatal("expected FileID for pdf blob")
	}
	got, _ := files.GetByIDs([]string{out.FileID}, "u1")
	if len(got) != 1 || got[0].Type != "application/pdf" {
		t.Fatalf("pdf file: %#v", got)
	}
}

func TestParseMCPToolResult_ResourceLink(t *testing.T) {
	setupMCPResultTest(t)

	out := parseMCPToolResult(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ResourceLink{
				URI:         "https://example.com/a",
				Name:        "A",
				Title:       "Doc A",
				MIMEType:    "text/html",
				Description: "desc",
			},
		},
	}, "u1")

	if !strings.Contains(out.Content, "https://example.com/a") ||
		!strings.Contains(out.Content, "Doc A") ||
		!strings.Contains(out.Content, "desc") {
		t.Fatalf("link formatting: %q", out.Content)
	}
	if out.FileID != "" {
		t.Fatalf("link should not set FileID, got %q", out.FileID)
	}
}

func TestParseMCPToolResult_StructuredContentAndIsError(t *testing.T) {
	setupMCPResultTest(t)

	out := parseMCPToolResult(&mcp.CallToolResult{
		IsError:           true,
		StructuredContent: map[string]any{"code": 42, "msg": "nope"},
		Content: []mcp.Content{
			&mcp.TextContent{Text: "failed"},
		},
	}, "u1")

	if !strings.HasPrefix(out.Content, "Tool error:") {
		t.Fatalf("expected error prefix: %q", out.Content)
	}
	if !strings.Contains(out.Content, `"code":42`) {
		t.Fatalf("expected structured json in content: %q", out.Content)
	}
	if !strings.Contains(out.Content, "failed") {
		t.Fatalf("expected text: %q", out.Content)
	}
}

func TestParseMCPToolResult_Empty(t *testing.T) {
	setupMCPResultTest(t)

	out := parseMCPToolResult(&mcp.CallToolResult{}, "u1")
	if out.Content != "Tool returned no content." {
		t.Fatalf("got %q", out.Content)
	}
	out = parseMCPToolResult(nil, "u1")
	if out.Content != "Empty tool result." {
		t.Fatalf("nil: got %q", out.Content)
	}
}

func TestExtFromMIME(t *testing.T) {
	if extFromMIME("image/png") != ".png" {
		t.Fatal()
	}
	if extFromMIME("image/jpeg; charset=x") != ".jpg" {
		t.Fatal()
	}
	if extFromMIME("application/pdf") != ".pdf" {
		t.Fatal()
	}
}
