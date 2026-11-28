package tools

import (
	"strings"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
	fs "github.com/Bajahaw/ai-ui/cmd/files"
)

func TestDocumentToolsEnforceOwnership(t *testing.T) {
	setupSandboxTest(t)
	if _, err := data.DB.Exec("INSERT INTO Users (username, pass_hash) VALUES ('u2', 'x')"); err != nil {
		t.Fatal(err)
	}
	for _, file := range []fs.File{
		{ID: "alice-doc", Name: "alice.txt", User: "u1"},
		{ID: "bob-doc", Name: "bob.txt", User: "u2"},
	} {
		if err := files.Save(file); err != nil {
			t.Fatal(err)
		}
	}
	if err := files.SavePages([]fs.FilePage{
		{ID: "alice-page", FileID: "alice-doc", PageNumber: 1, Content: "alice searchable content"},
		{ID: "bob-page", FileID: "bob-doc", PageNumber: 1, Content: "bob secret searchable content"},
	}); err != nil {
		t.Fatal(err)
	}

	for name, call := range map[string]func(args, user string) string{
		"search": func(args, user string) string { return searchDocumentTool(args, user).Content },
		"read":   func(args, user string) string { return readDocumentPageTool(args, user).Content },
	} {
		args := `{"file_id":"bob-doc","query":"searchable","start_page":1,"end_page":1}`
		if got := call(args, "u2"); !strings.Contains(got, "bob secret") || !strings.Contains(got, "searchable") {
			t.Fatalf("%s: owner could not read document: %s", name, got)
		}
		if got := call(args, "u1"); strings.Contains(got, "bob secret") || !strings.Contains(got, "Unable to find document") {
			t.Fatalf("%s: cross-user read was not rejected: %s", name, got)
		}
	}
}
