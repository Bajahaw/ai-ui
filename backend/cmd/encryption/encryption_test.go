package encryption

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bajahaw/ai-ui/cmd/data"
	_ "modernc.org/sqlite"
)

func newKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, keySize)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func useKey(t *testing.T, key string) {
	t.Helper()
	if err := Init(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { aead = nil })
}

func TestRoundTrip(t *testing.T) {
	useKey(t, newKey(t))

	enc, err := Encrypt("sk-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(enc) || strings.Contains(enc, "sk-secret") {
		t.Fatalf("not encrypted: %q", enc)
	}
	again, _ := Encrypt("sk-secret")
	if again == enc {
		t.Fatal("expected random nonce to produce distinct ciphertexts")
	}
	got, err := Decrypt(enc)
	if err != nil || got != "sk-secret" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestEmptyAndLegacyPlaintext(t *testing.T) {
	useKey(t, newKey(t))

	if enc, err := Encrypt(""); err != nil || enc != "" {
		t.Fatalf("empty: %q %v", enc, err)
	}
	if got, err := Decrypt("legacy-plain"); err != nil || got != "legacy-plain" {
		t.Fatalf("legacy: %q %v", got, err)
	}
}

func TestWrongKeyAndTamper(t *testing.T) {
	useKey(t, newKey(t))
	enc, _ := Encrypt("value")

	tampered := enc[:len(enc)-2] + "AA"
	if _, err := Decrypt(tampered); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("tampered: %v", err)
	}

	useKey(t, newKey(t))
	if _, err := Decrypt(enc); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("wrong key: %v", err)
	}
}

func TestInvalidKeys(t *testing.T) {
	for _, k := range []string{"", "short", base64.StdEncoding.EncodeToString(make([]byte, 16))} {
		if err := Init(k); !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("key %q: %v", k, err)
		}
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Cleanup(func() { aead = nil })

	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_FILE", "")
	if err := LoadFromEnv(); !errors.Is(err, ErrMissingKey) {
		t.Fatalf("missing: %v", err)
	}

	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte(newKey(t)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENCRYPTION_KEY_FILE", path)
	if err := LoadFromEnv(); err != nil {
		t.Fatalf("file: %v", err)
	}
}

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := data.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestVerifyKey(t *testing.T) {
	db := setupDB(t)
	useKey(t, newKey(t))

	if err := VerifyKey(db); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := VerifyKey(db); err != nil {
		t.Fatalf("same key: %v", err)
	}
	useKey(t, newKey(t))
	if err := VerifyKey(db); !errors.Is(err, ErrKeyMismatch) {
		t.Fatalf("other key: %v", err)
	}
}

func TestEncryptLegacyRows(t *testing.T) {
	db := setupDB(t)
	useKey(t, newKey(t))

	stmts := []string{
		`INSERT INTO Users (username, pass_hash) VALUES ('u', 'x')`,
		`INSERT INTO Providers (id, url, api_key, user, headers_json, oauth_json) VALUES ('p', 'http://a', 'sk-1', 'u', '{"X":"y"}', '')`,
		`INSERT INTO MCPServers (id, name, endpoint, api_key, user, headers_json) VALUES ('m', 'n', 'http://b', 'mk', 'u', '{}')`,
		`INSERT INTO UserSecrets (id, name, value, user) VALUES ('s', 'TOKEN', 'ghp', 'u')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}

	n, err := EncryptLegacyRows(db)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("encrypted %d values, want 5", n)
	}
	if n, _ := EncryptLegacyRows(db); n != 0 {
		t.Fatalf("second run encrypted %d values, want 0", n)
	}

	var apiKey, oauth, secret string
	_ = db.QueryRow(`SELECT api_key, oauth_json FROM Providers WHERE id = 'p'`).Scan(&apiKey, &oauth)
	_ = db.QueryRow(`SELECT value FROM UserSecrets WHERE id = 's'`).Scan(&secret)
	if oauth != "" {
		t.Fatalf("empty value should stay empty, got %q", oauth)
	}
	for want, enc := range map[string]string{"sk-1": apiKey, "ghp": secret} {
		if got, err := Decrypt(enc); err != nil || !IsEncrypted(enc) || got != want {
			t.Fatalf("want %q, got %q (stored %q) err=%v", want, got, enc, err)
		}
	}
}
