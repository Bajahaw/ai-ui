package chat

import (
	"database/sql"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/data"
)

func setupSearchTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := path.Join(tmpDir, "search.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if err := data.RunMigrations(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed two users with conversations and messages
	if _, err := db.Exec(`INSERT INTO Users (username, pass_hash) VALUES ('alice', 'x'), ('bob', 'y')`); err != nil {
		t.Fatalf("users: %v", err)
	}
	now := time.Now().UTC()
	if _, err := db.Exec(
		`INSERT INTO Conversations (id, user, title, created_at, updated_at) VALUES
			('c-alice', 'alice', 'Alice Chat', ?, ?),
			('c-bob', 'bob', 'Bob Chat', ?, ?)`,
		now, now, now, now,
	); err != nil {
		t.Fatalf("convs: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO Messages (conv_id, role, model, content, status, created_at, updated_at) VALUES
			('c-alice', 'user', '', 'The Euler-Lagrange equation is fundamental', 'completed', ?, ?),
			('c-alice', 'assistant', 'm', 'Here is a derivation of classical mechanics', 'completed', ?, ?),
			('c-bob', 'user', '', 'The Euler-Lagrange equation is private to bob', 'completed', ?, ?)`,
		now, now, now, now, now, now,
	); err != nil {
		t.Fatalf("messages: %v", err)
	}

	data.DB = db
	return db
}

func TestSearchMessagesFTS_UserScoped(t *testing.T) {
	setupSearchTestDB(t)

	hits, err := searchMessagesFTS("alice", "Euler-Lagrange", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit for alice, got %d", len(hits))
	}
	if hits[0].ConversationID != "c-alice" {
		t.Errorf("expected c-alice, got %s", hits[0].ConversationID)
	}
	// Bob's matching message must not leak
	for _, h := range hits {
		if h.ConversationID == "c-bob" {
			t.Error("bob conversation leaked into alice search results")
		}
	}

	bobHits, err := searchMessagesFTS("bob", "Euler-Lagrange", 10)
	if err != nil {
		t.Fatalf("bob search: %v", err)
	}
	if len(bobHits) != 1 || bobHits[0].ConversationID != "c-bob" {
		t.Fatalf("expected only c-bob for bob, got %+v", bobHits)
	}
}

func TestSearchMessagesFTS_EmptyQuery(t *testing.T) {
	setupSearchTestDB(t)
	hits, err := searchMessagesFTS("alice", "   ", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no hits for empty query, got %d", len(hits))
	}
}

func TestSearchMessagesFTS_BackfillAndSnippet(t *testing.T) {
	setupSearchTestDB(t)
	hits, err := searchMessagesFTS("alice", "classical mechanics", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].Snippet == "" {
		t.Error("expected non-empty snippet")
	}
	if hits[0].MessageID == 0 {
		t.Error("expected message id")
	}
}

func TestFts5QuoteChat(t *testing.T) {
	if got := fts5Quote(`say "hi"`); got != `"say ""hi"""` {
		t.Errorf("unexpected quote: %s", got)
	}
}

func TestSearchMessagesFTS_InjectionAttempts(t *testing.T) {
	setupSearchTestDB(t)

	attempts := []string{
		`" OR 1=1 --`,
		`"; DROP TABLE Messages; --`,
		`x" UNION SELECT * FROM Users --`,
		`content: OR *`,
		`"" OR ""=""`,
		`(((((((OR *`,
		`* -nonexistent`,
		`"" OR ""`,
		"` OR 1=1 --`",
		`test -- DROP TABLE`,
		`test; DROP TABLE Messages;`,
		`' OR '1'='1`,
	}

	for _, q := range attempts {
		t.Run(q, func(t *testing.T) {
			hits, err := searchMessagesFTS("alice", q, 10)
			if err != nil {
				t.Fatalf("query %q returned SQL error (possible injection): %v", q, err)
			}
			// Phrase-quoted MATCH must not dump bob's conversation to alice.
			for _, h := range hits {
				if h.ConversationID == "c-bob" {
					t.Errorf("cross-user leak for query %q: %+v", q, h)
				}
			}
		})
	}
}

func TestSearchMessagesFTS_EmptyUser(t *testing.T) {
	setupSearchTestDB(t)
	hits, err := searchMessagesFTS("", "Euler-Lagrange", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected no hits for empty user, got %d", len(hits))
	}
}

func TestSearchMessagesFTS_QueryLengthCap(t *testing.T) {
	setupSearchTestDB(t)
	// Very long input must not error.
	long := strings.Repeat("a", 5000)
	if _, err := searchMessagesFTS("alice", long, 10); err != nil {
		t.Fatalf("long query errored: %v", err)
	}
}
