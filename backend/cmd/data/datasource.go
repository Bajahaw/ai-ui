package data

import (
	"database/sql"
	"os"
	"path"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDataSource(dataSourceName string) error {
	var err error
	// validate dataSourceName
	dir := path.Dir(dataSourceName)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	DB, err = sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		_ = DB.Close()
		DB = nil
		return err
	}

	if _, err = DB.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = DB.Close()
		return err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS Users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		token TEXT NOT NULL
	);
	
	CREATE TABLE IF NOT EXISTS Conversations (
		id TEXT PRIMARY KEY,
		user TEXT,
		title TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS Attachments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		url TEXT NOT NULL
	);
	
	CREATE TABLE IF NOT EXISTS Messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conv_id TEXT NOT NULL,
		role TEXT NOT NULL,
		model TEXT NOT NULL,
		parent_id INTEGER,
		attachment TEXT,
		content TEXT NOT NULL,
		FOREIGN KEY (conv_id) REFERENCES Conversations(id) ON DELETE CASCADE
-- 		FOREIGN KEY (parent_id) REFERENCES Messages(id) ON DELETE SET NULL
-- 		FOREIGN KEY (attachment_id) REFERENCES Attachments(id) ON DELETE SET NULL
	);
	
	CREATE TABLE IF NOT EXISTS Providers (
		id TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		api_key TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS Models (
		id TEXT PRIMARY KEY,
		provider_id TEXT NOT NULL,
		name TEXT NOT NULL,
		is_enabled BOOLEAN NOT NULL DEFAULT 1,
		FOREIGN KEY (provider_id) REFERENCES Providers(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS Settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`

	_, err = DB.Exec(schema)

	return err
}
