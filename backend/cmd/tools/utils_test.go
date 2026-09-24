package tools

import (
	"os"
	"strings"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
	fs "github.com/Bajahaw/ai-ui/cmd/files"
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

func setupSaveBinaryTest(t *testing.T) {
	t.Helper()
	setupSandboxTest(t)
	l := logger.New(os.Stderr)
	l.SetLevel(logger.ErrorLevel)
	fs.SetupFiles(l, data.DB, nil)
}

// Tool-produced retrievable docs must be page-indexed like uploads when
// agentic retrieval is on, so the chat layer can embed text instead of a
// file part and search_document/read_document_page work on them.
func TestSaveBinaryFile_IndexesRetrievableDocWhenEnabled(t *testing.T) {
	setupSaveBinaryTest(t)
	if err := settings.Save(map[string]string{"agenticDocumentRetrieval": "true"}, "u1"); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	saved, err := saveBinaryFile(minimalPDF, "application/pdf", "hello.pdf", "u1")
	if err != nil {
		t.Fatalf("saveBinaryFile: %v", err)
	}
	if !strings.Contains(saved.Content, "Hello") {
		t.Fatalf("expected extracted content, got %q", saved.Content)
	}

	persisted, err := files.GetByIDs([]string{saved.ID}, "u1")
	if err != nil || len(persisted) != 1 {
		t.Fatalf("GetByIDs: %v %v", persisted, err)
	}
	if !strings.Contains(persisted[0].Content, "Hello") {
		t.Fatalf("persisted content: %q", persisted[0].Content)
	}
	page, err := files.GetPage(saved.ID, 1)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if !strings.Contains(page.Content, "Hello") {
		t.Fatalf("page content: %q", page.Content)
	}
}

func TestSaveBinaryFile_NoIndexWhenRetrievalOff(t *testing.T) {
	setupSaveBinaryTest(t)

	saved, err := saveBinaryFile(minimalPDF, "application/pdf", "hello.pdf", "u1")
	if err != nil {
		t.Fatalf("saveBinaryFile: %v", err)
	}
	if saved.Content != "" {
		t.Fatalf("expected no extraction with retrieval off, got %q", saved.Content)
	}
	if _, err := files.GetPage(saved.ID, 1); err == nil {
		t.Fatal("expected no indexed pages")
	}
}

// Extraction failure must not lose the generated file: the user still gets
// the download, the model just gets metadata only.
func TestSaveBinaryFile_IndexFailureIsNonFatal(t *testing.T) {
	setupSaveBinaryTest(t)
	if err := settings.Save(map[string]string{"agenticDocumentRetrieval": "true"}, "u1"); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	saved, err := saveBinaryFile([]byte("not really a pdf"), "application/pdf", "broken.pdf", "u1")
	if err != nil {
		t.Fatalf("saveBinaryFile should succeed even when indexing fails: %v", err)
	}
	if saved.ID == "" || saved.Content != "" {
		t.Fatalf("expected saved file without content, got %+v", saved)
	}
}
