package files

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	logger "github.com/charmbracelet/log"
)

// minimalPDF is a one-page PDF containing the word "Hello".
var minimalPDF = []byte(`%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
endobj
4 0 obj
<< /Length 44 >>
stream
BT /F1 24 Tf 100 700 Td (Hello) Tj ET
endstream
endobj
5 0 obj
<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000266 00000 n 
0000000359 00000 n 
trailer
<< /Size 6 /Root 1 0 R >>
startxref
428
%%EOF
`)

func setupUploadEnv(t *testing.T) {
	t.Helper()
	_, db := setupTestDB(t)

	l := logger.New(os.Stderr)
	l.SetLevel(logger.ErrorLevel)
	SetupFiles(l, db, nil)

	// saveUploadedFile writes under ./data/resources relative to cwd
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})
}

func multipartPDF(t *testing.T, name string, data []byte) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	header := &multipart.FileHeader{
		Filename: name,
		Size:     int64(len(data)),
		Header:   make(textproto.MIMEHeader),
	}
	header.Header.Set("Content-Type", "application/pdf")
	return &virtualFile{Reader: bytes.NewReader(data)}, header
}

func TestResourcePath(t *testing.T) {
	if got := ResourcePath(File{Path: "./data/resources/abc.xlsx"}); got != "/data/resources/abc.xlsx" {
		t.Fatalf("dot path: %q", got)
	}
	if got := ResourcePath(File{Path: `data\resources\abc.xlsx`}); got != "/data/resources/abc.xlsx" {
		t.Fatalf("windows path: %q", got)
	}
	if got := ResourcePath(File{Path: "/tmp/shot.png"}); got != "/data/resources/shot.png" {
		t.Fatalf("basename fallback: %q", got)
	}
	if got := ResourcePath(File{URL: "/data/resources/abc.xlsx"}); got != "" {
		t.Fatalf("must ignore deprecated URL: %q", got)
	}
	if got := ResourcePath(File{}); got != "" {
		t.Fatalf("empty: %q", got)
	}
}

func TestSavePages_RequiresParentFile(t *testing.T) {
	repo, _ := setupTestDB(t)

	err := repo.SavePages([]FilePage{{
		ID:         "orphan-page-1",
		FileID:     "missing-file",
		PageNumber: 1,
		Content:    "orphan",
	}})
	if err == nil {
		t.Fatal("expected FK failure when parent file is missing")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "constraint") {
		t.Fatalf("expected constraint error, got: %v", err)
	}
}

func TestSavePages_AfterFile_Succeeds(t *testing.T) {
	repo, db := setupTestDB(t)
	fileID := "file-ok"
	seedFile(t, db, fileID)
	seedPage(t, repo, fileID, "page content about gravity", 1)

	page, err := repo.GetPage(fileID, 1)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if page.Content != "page content about gravity" {
		t.Fatalf("unexpected content: %q", page.Content)
	}

	hits, err := repo.SearchPages(fileID, "gravity", 10)
	if err != nil {
		t.Fatalf("SearchPages: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 search hit, got %d", len(hits))
	}
}

func TestSaveUploadedFile_PDFAgenticRetrieval_E2E(t *testing.T) {
	setupUploadEnv(t)

	if err := settings.Save(map[string]string{
		"agenticDocumentRetrieval": "true",
	}, "testuser"); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	file, header := multipartPDF(t, "hello.pdf", minimalPDF)
	got, err := saveUploadedFile(file, header, "testuser")
	if err != nil {
		t.Fatalf("saveUploadedFile: %v", err)
	}

	if got.ID == "" {
		t.Fatal("expected file id")
	}
	if got.Type != "application/pdf" {
		t.Fatalf("type = %q, want application/pdf", got.Type)
	}
	if got.Content == "" {
		t.Fatal("expected extracted content on file")
	}
	if !strings.Contains(got.Content, "Hello") && !strings.Contains(got.Content, "Document content page 1") {
		t.Fatalf("content missing expected markers: %q", got.Content)
	}

	// Parent row persisted
	files, err := repo.GetByIDs([]string{got.ID}, "testuser")
	if err != nil || len(files) != 1 {
		t.Fatalf("GetByIDs: files=%d err=%v", len(files), err)
	}
	if files[0].Content == "" {
		t.Fatal("persisted file content empty")
	}

	// Pages indexed under the saved file id (FK + FTS path)
	page, err := repo.GetPage(got.ID, 1)
	if err != nil {
		t.Fatalf("GetPage after upload: %v", err)
	}
	if !strings.Contains(page.Content, "Hello") {
		t.Fatalf("page content missing Hello: %q", page.Content)
	}

	hits, err := repo.SearchPages(got.ID, "Hello", 10)
	if err != nil {
		t.Fatalf("SearchPages after upload: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 FTS hit, got %d", len(hits))
	}

	// Physical file written
	if _, err := os.Stat(got.Path); err != nil {
		t.Fatalf("physical file missing at %s: %v", got.Path, err)
	}
	// cleanup path is under temp cwd
	_ = filepath.Dir(got.Path)
}

func TestSaveUploadedFile_TextNoExtraction_E2E(t *testing.T) {
	setupUploadEnv(t)

	data := []byte("plain note content")
	header := &multipart.FileHeader{
		Filename: "note.txt",
		Size:     int64(len(data)),
		Header:   make(textproto.MIMEHeader),
	}
	header.Header.Set("Content-Type", "text/plain")
	file := &virtualFile{Reader: bytes.NewReader(data)}

	got, err := saveUploadedFile(file, header, "testuser")
	if err != nil {
		t.Fatalf("saveUploadedFile: %v", err)
	}
	if got.Content != "" {
		t.Fatalf("expected empty content without OCR flags, got %q", got.Content)
	}
	if !strings.HasPrefix(got.Type, "text/") {
		t.Fatalf("type = %q, want text/*", got.Type)
	}
	if got.URL != "" {
		t.Fatalf("URL must stay empty, got %q", got.URL)
	}

	files, err := repo.GetByIDs([]string{got.ID}, "testuser")
	if err != nil || len(files) != 1 {
		t.Fatalf("GetByIDs: files=%d err=%v", len(files), err)
	}
}
