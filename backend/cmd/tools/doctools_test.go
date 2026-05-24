package tools

import (
	"database/sql"
	"encoding/json"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/data"
	logger "github.com/charmbracelet/log"
)

func setupRealDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := path.Join(tmpDir, "test.db")

	err := data.InitDataSource(dbPath)
	if err != nil {
		t.Fatalf("Failed to init data source: %v", err)
	}

	// We need a user to satisfy constraints
	_, err = data.DB.Exec("INSERT INTO Users (username, pass_hash) VALUES (?, ?)", "admin", "hash")
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	return data.DB
}

func TestDocumentTools_EndToEnd(t *testing.T) {
	db := setupRealDB(t)
	defer db.Close()

	// Initialize the tools package using the real db
	l := logger.New(os.Stderr)
	SetUpTools(l, db)

	// Since SetUpTools sets package-level 'files', we are good to go.
	// But let's change dir so files are stored in temp dir instead of real workspace
	tmpDir := t.TempDir()
	originalWD, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWD)

	tests := []struct {
		name        string
		format      string
		action      string // "create", "write", "delete"
		parts       map[string]string
		partPath    string
		partContent string
		wantErr     bool
		errMsg      string
	}{
		{
			name:   "Create Valid Minimal Document",
			format: "docx",
			action: "create",
			parts: map[string]string{
				"word/document.xml": `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"></w:document>`,
			},
			wantErr: false,
		},
		{
			name:   "Create XML Syntax Error",
			format: "docx",
			action: "create",
			parts: map[string]string{
				"word/document.xml": `<?xml version="1.0"?><w:document><unclosed>`,
			},
			wantErr: true,
			errMsg:  "XML syntax error",
		},
		{
			name:   "Create Missing Content Type Part",
			format: "docx",
			action: "create",
			parts: map[string]string{
				"[Content_Types].xml": `<?xml version="1.0"?>
<Types><Override PartName="/missing.xml" ContentType="foo"/></Types>`,
			},
			wantErr: true,
			errMsg:  "reference error in '[Content_Types].xml'",
		},
		{
			name:     "Write Subdirectory Relationship Validation",
			action:   "write",
			format:   "docx",
			partPath: "word/_rels/document.xml.rels",
			partContent: `<?xml version="1.0"?>
<Relationships><Relationship Id="r1" Target="missing.xml"/></Relationships>`,
			wantErr: true,
			errMsg:  "reference error in 'word/_rels/document.xml.rels'",
		},
		{
			name:     "Delete Critical Part",
			action:   "delete",
			format:   "docx",
			partPath: "word/styles.xml", // deleting this breaks word/_rels/document.xml.rels which references styles.xml
			wantErr:  true,
			errMsg:   "reference error in 'word/_rels/document.xml.rels'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := "admin"

			// We always want to create a document first for "write" and "delete"
			createArgs := map[string]any{
				"file_name": "test.docx",
				"format":    "docx",
			}
			if tt.action == "create" {
				createArgs["format"] = tt.format
				if tt.parts != nil {
					createArgs["parts"] = tt.parts
				}
			}

			createJSON, _ := json.Marshal(createArgs)
			createOut := createDocumentTool(string(createJSON), user)

			if tt.action == "create" {
				if tt.wantErr {
					if !strings.Contains(createOut.Content, "Validation failed") {
						t.Errorf("Expected validation failure, got: %s", createOut.Content)
					}
					if tt.errMsg != "" && !strings.Contains(createOut.Content, tt.errMsg) {
						t.Errorf("Expected error to contain %q, but got: %s", tt.errMsg, createOut.Content)
					}
				} else {
					if strings.Contains(createOut.Content, "Validation failed") || strings.Contains(createOut.Content, "error") {
						t.Errorf("Expected success, got: %s", createOut.Content)
					}
				}
				return
			}

			// For write/delete, we need the fileID we just created
			re := regexp.MustCompile(`File ID: ([a-f0-9-]+)`)
			match := re.FindStringSubmatch(createOut.Content)
			if len(match) < 2 {
				t.Fatalf("Failed to extract file ID from create document output: %s", createOut.Content)
			}
			fileID := match[1]

			// Give DB some time for tests locally if needed, but it shouldn't be req.
			time.Sleep(10 * time.Millisecond)

			if tt.action == "write" {
				writeArgs := map[string]any{
					"file_id":   fileID,
					"part_path": tt.partPath,
					"content":   tt.partContent,
				}
				writeJSON, _ := json.Marshal(writeArgs)
				writeOut := writeDocumentPartTool(string(writeJSON), user)

				if tt.wantErr {
					if !strings.Contains(writeOut.Content, "Validation failed") && !strings.Contains(writeOut.Content, "error") {
						t.Errorf("Expected failure, got: %s", writeOut.Content)
					}
					if tt.errMsg != "" && !strings.Contains(writeOut.Content, tt.errMsg) {
						t.Errorf("Expected error to contain %q, but got: %s", tt.errMsg, writeOut.Content)
					}
				} else {
					if strings.Contains(writeOut.Content, "Validation failed") || strings.Contains(writeOut.Content, "error") {
						t.Errorf("Expected success, got: %s", writeOut.Content)
					}
				}
			} else if tt.action == "delete" {
				deleteArgs := map[string]any{
					"file_id":   fileID,
					"part_path": tt.partPath,
				}
				deleteJSON, _ := json.Marshal(deleteArgs)
				deleteOut := deleteDocumentPartTool(string(deleteJSON), user)

				if tt.wantErr {
					if !strings.Contains(deleteOut.Content, "Validation failed") && !strings.Contains(deleteOut.Content, "error") {
						t.Errorf("Expected failure, got: %s", deleteOut.Content)
					}
				} else {
					if strings.Contains(deleteOut.Content, "Validation failed") || strings.Contains(deleteOut.Content, "error") {
						t.Errorf("Expected success, got: %s", deleteOut.Content)
					}
				}
			}
		})
	}
}

func TestValidateOfficeZip_NotAZip(t *testing.T) {
	err := validateOfficeZip([]byte("not a zip file"))
	if err == nil {
		t.Error("Expected error for invalid zip archive, got nil")
	} else if !strings.Contains(err.Error(), "invalid zip archive") {
		t.Errorf("Expected 'invalid zip archive' error, got: %v", err)
	}
}
