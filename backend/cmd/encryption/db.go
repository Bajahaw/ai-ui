package encryption

import (
	"database/sql"
	"errors"
	"fmt"
)

const (
	canaryKey   = "encryption_canary"
	canaryPlain = "ai-ui-encryption-check"
)

// ErrKeyMismatch means the database was encrypted with a different key.
var ErrKeyMismatch = errors.New("ENCRYPTION_KEY does not match the database")

// sensitiveColumns lists every column holding credentials that must be encrypted at rest.
var sensitiveColumns = []struct {
	table   string
	columns []string
}{
	{"Providers", []string{"api_key", "headers_json", "oauth_json"}},
	{"MCPServers", []string{"api_key", "headers_json", "oauth_json"}},
	{"UserSecrets", []string{"value"}},
}

// VerifyKey fails fast when the loaded key cannot decrypt this database.
// On first run it stores an encrypted canary for future checks.
func VerifyKey(db *sql.DB) error {
	var stored string
	err := db.QueryRow(`SELECT value FROM AppMeta WHERE key = ?`, canaryKey).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		enc, err := Encrypt(canaryPlain)
		if err != nil {
			return err
		}
		_, err = db.Exec(`INSERT INTO AppMeta (key, value) VALUES (?, ?)`, canaryKey, enc)
		return err
	}
	if err != nil {
		return err
	}
	if plain, err := Decrypt(stored); err != nil || plain != canaryPlain {
		return ErrKeyMismatch
	}
	return nil
}

// EncryptLegacyRows encrypts plaintext values written before encryption was
// introduced. It is idempotent and runs in a single transaction.
func EncryptLegacyRows(db *sql.DB) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	total := 0
	for _, t := range sensitiveColumns {
		for _, col := range t.columns {
			n, err := encryptColumn(tx, t.table, col)
			if err != nil {
				return 0, fmt.Errorf("%s.%s: %w", t.table, col, err)
			}
			total += n
		}
	}
	return total, tx.Commit()
}

func encryptColumn(tx *sql.Tx, table, col string) (int, error) {
	rows, err := tx.Query(fmt.Sprintf(
		`SELECT rowid, %[1]s FROM %[2]s WHERE %[1]s IS NOT NULL AND %[1]s != '' AND substr(%[1]s, 1, %[3]d) != '%[4]s'`,
		col, table, len(Prefix), Prefix,
	))
	if err != nil {
		return 0, err
	}
	type row struct {
		id    int64
		value string
	}
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.value); err != nil {
			_ = rows.Close()
			return 0, err
		}
		pending = append(pending, r)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	update := fmt.Sprintf(`UPDATE %s SET %s = ? WHERE rowid = ?`, table, col)
	for _, r := range pending {
		enc, err := Encrypt(r.value)
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(update, enc, r.id); err != nil {
			return 0, err
		}
	}
	return len(pending), nil
}
