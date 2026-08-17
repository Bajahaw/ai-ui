package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
	fs "github.com/Bajahaw/ai-ui/cmd/files"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	"github.com/Bajahaw/ai-ui/cmd/tools"
)

func TestEmbeddedMetadataIncludesPublicPath(t *testing.T) {
	got := embeddedMetadata(fs.Attachment{
		File: fs.File{
			ID:   "fid",
			Name: "report.xlsx",
			Type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			Size: 12,
			Path: "./data/resources/fid.xlsx",
		},
	})
	for _, want := range []string{
		"[user attachment:",
		"id: fid",
		"name: report.xlsx",
		"path: /data/resources/fid.xlsx",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestResolveToolFileMedia_Image(t *testing.T) {
	teardown := setupTest(t, &mockProviderSuccess{})
	defer teardown()

	tmp := t.TempDir()
	imgPath := filepath.Join(tmp, "shot.png")
	if err := os.WriteFile(imgPath, []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}

	fileID := "tool-img-1"
	if err := files.Save(fs.File{
		ID:   fileID,
		Name: "shot.png",
		Type: "image/png",
		Size: 4,
		Path: imgPath,
		URL:  "/data/resources/shot.png",
		User: "test-user",
	}); err != nil {
		t.Fatalf("save file: %v", err)
	}

	images, nonImages := resolveToolFileMedia(fileID, "test-user")
	if len(nonImages) != 0 {
		t.Fatalf("expected no non-image media, got %v", nonImages)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image data URL, got %d", len(images))
	}
	if !strings.HasPrefix(images[0], "data:image/png;base64,") {
		t.Fatalf("unexpected data URL: %s", images[0][:min(40, len(images[0]))])
	}
}

func TestResolveToolFileMedia_NonImage(t *testing.T) {
	teardown := setupTest(t, &mockProviderSuccess{})
	defer teardown()

	tmp := t.TempDir()
	pdfPath := filepath.Join(tmp, "doc.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4"), 0o644); err != nil {
		t.Fatal(err)
	}

	fileID := "tool-pdf-1"
	if err := files.Save(fs.File{
		ID:   fileID,
		Name: "doc.pdf",
		Type: "application/pdf",
		Size: 8,
		Path: pdfPath,
		URL:  "/data/resources/doc.pdf",
		User: "test-user",
	}); err != nil {
		t.Fatalf("save file: %v", err)
	}

	images, nonImages := resolveToolFileMedia(fileID, "test-user")
	if len(images) != 0 {
		t.Fatalf("expected no images, got %v", images)
	}
	if len(nonImages) != 1 || !strings.HasPrefix(nonImages[0], "data:application/pdf;base64,") {
		t.Fatalf("expected pdf data URL, got %v", nonImages)
	}
}

func TestResolveToolFileMedia_EmptyAndMissing(t *testing.T) {
	teardown := setupTest(t, &mockProviderSuccess{})
	defer teardown()

	imgs, filesOut := resolveToolFileMedia("", "test-user")
	if imgs != nil || filesOut != nil {
		t.Fatalf("empty id should return nils")
	}
	imgs, filesOut = resolveToolFileMedia("missing", "test-user")
	if imgs != nil || filesOut != nil {
		t.Fatalf("missing id should return nils")
	}
}

func TestBuildContext_ToolFileIDNotMutatedToDataURL(t *testing.T) {
	teardown := setupTest(t, &mockProviderSuccess{})
	defer teardown()

	tmp := t.TempDir()
	imgPath := filepath.Join(tmp, "shot.png")
	if err := os.WriteFile(imgPath, []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}
	fileID := "ctx-img-1"
	if err := files.Save(fs.File{
		ID:   fileID,
		Name: "shot.png",
		Type: "image/png",
		Size: 4,
		Path: imgPath,
		URL:  "/x",
		User: "test-user",
	}); err != nil {
		t.Fatal(err)
	}

	// Conversation + assistant message + tool call with FileID
	if _, err := data.DB.Exec(
		`INSERT INTO Conversations (id, user, title) VALUES (?, ?, ?)`,
		"conv1", "test-user", "t",
	); err != nil {
		t.Fatal(err)
	}
	res, err := data.DB.Exec(
		`INSERT INTO Messages (conv_id, role, model, parent_id, content, reasoning, error, status) VALUES (?, 'assistant', 'm', 0, 'hi', '', '', 'completed')`,
		"conv1",
	)
	if err != nil {
		t.Fatal(err)
	}
	msgID, _ := res.LastInsertId()

	tc := &providers.ToolCall{
		ID:          "tc1",
		ReferenceID: "ref1",
		ConvID:      "conv1",
		MessageID:   int(msgID),
		Name:        "view_document_page",
		Args:        `{}`,
		Output:      "rendered",
		FileID:      fileID,
	}
	if err := tools.NewToolCallsRepository(data.DB).Save(tc); err != nil {
		t.Fatalf("save tool call: %v", err)
	}

	msgs := buildContext("conv1", int(msgID), "test-user")

	var toolMsg *providers.SimpleMessage
	for i := range msgs {
		if msgs[i].Role == "tool" {
			toolMsg = &msgs[i]
			break
		}
	}
	if toolMsg == nil {
		t.Fatal("expected tool message in context")
	}
	if toolMsg.ToolCall.FileID != fileID {
		t.Fatalf("FileID must stay as id, got %q", toolMsg.ToolCall.FileID)
	}
	if strings.HasPrefix(toolMsg.ToolCall.FileID, "data:") {
		t.Fatal("FileID was mutated to data URL")
	}
	if len(toolMsg.Images) != 1 || !strings.HasPrefix(toolMsg.Images[0], "data:image/png;base64,") {
		t.Fatalf("expected resolved image on SimpleMessage.Images, got %v", toolMsg.Images)
	}
}

func TestToolCallRepo_FileIDRoundTrip(t *testing.T) {
	teardown := setupTest(t, &mockProviderSuccess{})
	defer teardown()

	if _, err := data.DB.Exec(
		`INSERT INTO Conversations (id, user, title) VALUES (?, ?, ?)`,
		"conv2", "test-user", "t",
	); err != nil {
		t.Fatal(err)
	}
	res, err := data.DB.Exec(
		`INSERT INTO Messages (conv_id, role, model, parent_id, content, reasoning, error, status) VALUES (?, 'assistant', 'm', 0, 'x', '', '', 'completed')`,
		"conv2",
	)
	if err != nil {
		t.Fatal(err)
	}
	msgID, _ := res.LastInsertId()

	// FK requires file row when file_id set
	tmp := t.TempDir()
	p := filepath.Join(tmp, "f.bin")
	_ = os.WriteFile(p, []byte("x"), 0o644)
	if err := files.Save(fs.File{
		ID: "fid-rt", Name: "f.bin", Type: "application/octet-stream",
		Size: 1, Path: p, URL: "/f", User: "test-user",
	}); err != nil {
		t.Fatal(err)
	}

	repo := tools.NewToolCallsRepository(data.DB)
	in := &providers.ToolCall{
		ID: "tc-rt", ReferenceID: "r", ConvID: "conv2", MessageID: int(msgID),
		Name: "t", Args: `{}`, Output: "out", FileID: "fid-rt",
	}
	if err := repo.Save(in); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := repo.GetAllByMessageID(int(msgID))
	if len(got) != 1 {
		t.Fatalf("got %d tool calls", len(got))
	}
	if got[0].FileID != "fid-rt" {
		t.Fatalf("FileID roundtrip: got %q", got[0].FileID)
	}
	byConv := repo.GetAllByConvID("conv2")
	if len(byConv) != 1 || byConv[0].FileID != "fid-rt" {
		t.Fatalf("GetAllByConvID FileID: %+v", byConv)
	}
}
