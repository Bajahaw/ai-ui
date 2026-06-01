package files

import (
	"database/sql"
	"fmt"
	"path"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
)

// setupTestDB creates a temporary database with all migrations applied,
// inserts a test user, and returns a Repository backed by it.
func setupTestDB(t *testing.T) (Repository, *sql.DB) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := path.Join(tmpDir, "test.db")

	if err := data.InitDataSource(dbPath); err != nil {
		t.Fatalf("Failed to init data source: %v", err)
	}
	db := data.DB

	t.Cleanup(func() {
		db.Close()
	})

	if _, err := db.Exec("INSERT INTO Users (username, pass_hash) VALUES (?, ?)", "testuser", "hash"); err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	return NewRepository(db), db
}

func seedPage(t *testing.T, repo Repository, fileID, content string, pageNum int) {
	t.Helper()
	err := repo.SavePages([]FilePage{{
		ID:         fmt.Sprintf("%s-page-%d", fileID, pageNum),
		FileID:     fileID,
		PageNumber: pageNum,
		Content:    content,
	}})
	if err != nil {
		t.Fatalf("Failed to save page: %v", err)
	}
}

func seedFile(t *testing.T, db *sql.DB, fileID string) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO Files (id, name, type, size, path, url, content, user) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		fileID, "test.pdf", "application/pdf", 100, "/tmp/test.pdf", "/files/test.pdf", "", "testuser",
	)
	if err != nil {
		t.Fatalf("Failed to insert file: %v", err)
	}
}

// ---------- fts5Quote unit tests ----------

func TestFts5Quote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple word", "hello", `"hello"`},
		{"phrase", "hello world", `"hello world"`},
		{"hyphenated", "Euler-Lagrange", `"Euler-Lagrange"`},
		{"hyphenated phrase", "Euler-Lagrange equation", `"Euler-Lagrange equation"`},
		{"embedded quotes", `say "hello"`, `"say ""hello"""`},
		{"leading/trailing spaces", "  hello  ", `"hello"`},
		{"empty", "", `""`},
		{"only spaces", "   ", `""`},
		{"special fts5 chars", `term:column OR 1=1`, `"term:column OR 1=1"`},
		{"parentheses", `foo(bar)`, `"foo(bar)"`},
		{"asterisk wildcard", `test*`, `"test*"`},
		{"negation attempt", `a -b`, `"a -b"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fts5Quote(tt.input)
			if got != tt.want {
				t.Errorf("fts5Quote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- Integration tests with real FTS5 ----------

func TestSearchPages_BasicQuery(t *testing.T) {
	repo, db := setupTestDB(t)
	fileID := "file-1"
	seedFile(t, db, fileID)
	seedPage(t, repo, fileID, "The Euler-Lagrange equation is fundamental to classical mechanics.", 1)
	seedPage(t, repo, fileID, "Newton's laws of motion describe the relationship between force and mass.", 2)

	pages, err := repo.SearchPages(fileID, "Euler-Lagrange", 10)
	if err != nil {
		t.Fatalf("SearchPages failed: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 result, got %d", len(pages))
	}
	if pages[0].PageNumber != 1 {
		t.Errorf("expected page 1, got page %d", pages[0].PageNumber)
	}
}

func TestSearchPages_PhraseQuery(t *testing.T) {
	repo, db := setupTestDB(t)
	fileID := "file-2"
	seedFile(t, db, fileID)
	seedPage(t, repo, fileID, "The Euler-Lagrange equation is fundamental to classical mechanics.", 1)

	pages, err := repo.SearchPages(fileID, "Euler-Lagrange equation", 10)
	if err != nil {
		t.Fatalf("SearchPages failed: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 result, got %d", len(pages))
	}
}

func TestSearchPages_MultipleMatches(t *testing.T) {
	repo, db := setupTestDB(t)
	fileID := "file-3"
	seedFile(t, db, fileID)
	seedPage(t, repo, fileID, "Quantum mechanics uses the Schrödinger equation.", 1)
	seedPage(t, repo, fileID, "The Dirac equation extends quantum mechanics to relativistic particles.", 2)
	seedPage(t, repo, fileID, "Classical mechanics is governed by Newtonian physics.", 3)

	pages, err := repo.SearchPages(fileID, "equation", 10)
	if err != nil {
		t.Fatalf("SearchPages failed: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected 2 results, got %d", len(pages))
	}
}

func TestSearchPages_NoMatch(t *testing.T) {
	repo, db := setupTestDB(t)
	fileID := "file-4"
	seedFile(t, db, fileID)
	seedPage(t, repo, fileID, "The quick brown fox jumps over the lazy dog.", 1)

	pages, err := repo.SearchPages(fileID, "nonexistent", 10)
	if err != nil {
		t.Fatalf("SearchPages failed: %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("expected 0 results, got %d", len(pages))
	}
}

func TestSearchPages_LimitRespected(t *testing.T) {
	repo, db := setupTestDB(t)
	fileID := "file-5"
	seedFile(t, db, fileID)
	for i := 1; i <= 5; i++ {
		seedPage(t, repo, fileID, "This page contains the keyword quantum.", i)
	}

	pages, err := repo.SearchPages(fileID, "quantum", 2)
	if err != nil {
		t.Fatalf("SearchPages failed: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected 2 results due to limit, got %d", len(pages))
	}
}

func TestSearchPages_ScopedToFile(t *testing.T) {
	repo, db := setupTestDB(t)
	seedFile(t, db, "file-a")
	seedFile(t, db, "file-b")
	seedPage(t, repo, "file-a", "Content about gravity and spacetime curvature.", 1)
	seedPage(t, repo, "file-b", "Content about cooking recipes and pasta.", 1)

	pages, err := repo.SearchPages("file-a", "gravity", 10)
	if err != nil {
		t.Fatalf("SearchPages failed: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 result, got %d", len(pages))
	}
	if pages[0].FileID != "file-a" {
		t.Errorf("expected file-a, got %s", pages[0].FileID)
	}
}

// ---------- SQL / FTS5 injection attempts ----------

func TestSearchPages_InjectionAttempts(t *testing.T) {
	repo, db := setupTestDB(t)
	fileID := "file-inj"
	seedFile(t, db, fileID)
	seedPage(t, repo, fileID, "Normal page content about mathematics.", 1)

	attempts := []struct {
		name  string
		query string
	}{
		{"classic OR injection", `" OR 1=1 --`},
		{"drop table", `"; DROP TABLE FilePages; --`},
		{"union select", `x" UNION SELECT * FROM Users --`},
		{"fts5 column filter abuse", `content: OR *`},
		{"nested quotes", `"" OR ""=""`},
		{"parentheses bomb", `(((((((OR *`},
		{"negation to dump all", `* -nonexistent`},
		{"empty injection", `"" OR ""`},
		{"backtick injection", "` OR 1=1 --`"},
		{"double hyphen comment", `test -- DROP TABLE`},
		{"semicolon injection", `test; DROP TABLE Files;`},
		{"single quote escape", `' OR '1'='1`},
	}

	for _, tt := range attempts {
		t.Run(tt.name, func(t *testing.T) {
			// Must not panic or return SQL-level errors.
			// It's OK to return zero results or results — the key is no SQL error.
			_, err := repo.SearchPages(fileID, tt.query, 10)
			if err != nil {
				t.Errorf("query %q returned a SQL error (possible injection): %v", tt.query, err)
			}
		})
	}
}
