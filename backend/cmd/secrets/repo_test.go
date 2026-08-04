package secrets

import (
	"database/sql"
	"path"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := path.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := data.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO Users (username, pass_hash) VALUES ('alice', 'x')`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSecretCRUDAndExpand(t *testing.T) {
	db := setupTestDB(t)
	SetupSecrets(nil, db)

	created, err := Create("alice", "github_token", "ghp_secret")
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "GITHUB_TOKEN" {
		t.Fatalf("name %s", created.Name)
	}

	// List has no values
	meta := ListMeta("alice")
	if len(meta) != 1 || meta[0].Name != "GITHUB_TOKEN" {
		t.Fatalf("%+v", meta)
	}

	// Expand
	out, err := ExpandForUser(`Bearer $secrets.GITHUB_TOKEN$`, "alice")
	if err != nil || out != "Bearer ghp_secret" {
		t.Fatalf("got %q err=%v", out, err)
	}

	// Update name keep value
	updated, err := Update(created.ID, "alice", "GH_TOKEN", "")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "GH_TOKEN" {
		t.Fatal(updated.Name)
	}
	out, err = ExpandForUser(`$secrets.GH_TOKEN$`, "alice")
	if err != nil || out != "ghp_secret" {
		t.Fatalf("got %q err=%v", out, err)
	}

	// Update value
	if _, err := Update(created.ID, "alice", "GH_TOKEN", "newval"); err != nil {
		t.Fatal(err)
	}
	out, err = ExpandForUser(`$secrets.GH_TOKEN$`, "alice")
	if err != nil || out != "newval" {
		t.Fatalf("got %q", out)
	}

	// Isolation
	if _, err := db.Exec(`INSERT INTO Users (username, pass_hash) VALUES ('bob', 'x')`); err != nil {
		t.Fatal(err)
	}
	if len(ListMeta("bob")) != 0 {
		t.Fatal("bob should have no secrets")
	}

	if err := Delete(created.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if len(ListMeta("alice")) != 0 {
		t.Fatal("expected empty")
	}
}
