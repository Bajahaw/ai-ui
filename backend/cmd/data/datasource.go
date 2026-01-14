package data

import (
	"database/sql"
	"os"
	"path"

	_ "modernc.org/sqlite"
	// _ "github.com/mattn/go-sqlite3"
	// _ "github.com/tursodatabase/turso-go"
)

var DB *sql.DB

func InitDataSource(dataSourceName string) error {
	var err error
	// validate dataSourceName
	dir := path.Dir(dataSourceName)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Add _pragma=foreign_keys(1) to ensure foreign keys are enabled on every connection
	// This is critical for modernc.org/sqlite with connection pooling
	dsn := dataSourceName + "??_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	DB, err = sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		_ = DB.Close()
		DB = nil
		return err
	}

	// Set connection pool settings first
	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(0)

	// Enable foreign keys - CRITICAL: Must be executed to take effect
	if _, err = DB.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return err
	}

	// Standard optimizations (these are persistent per database file)
	if _, err = DB.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		return err
	}

	if _, err = DB.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		return err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS Users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		pass_hash TEXT NOT NULL
	);
	
	CREATE TABLE IF NOT EXISTS Conversations (
		id TEXT PRIMARY KEY,
		user TEXT,
		title TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user) REFERENCES Users(username) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS Files (
		id TEXT PRIMARY KEY,
		name TEXT,
		type TEXT NOT NULL,
		size INTEGER,
		path TEXT NOT NULL,
		url TEXT NOT NULL,
		content TEXT NOT NULL,
		user TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user) REFERENCES Users(username) ON DELETE CASCADE
	);
	
	CREATE TABLE IF NOT EXISTS Messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conv_id TEXT NOT NULL,
		role TEXT NOT NULL,
		model TEXT NOT NULL,
		parent_id INTEGER,
		content TEXT NOT NULL,
		reasoning TEXT,
		error TEXT,
		FOREIGN KEY (conv_id) REFERENCES Conversations(id) ON DELETE CASCADE
	);
		
	CREATE TABLE IF NOT EXISTS Attachments (
		id TEXT PRIMARY KEY,
		message_id INTEGER NOT NULL,
		file_id TEXT NOT NULL,
		FOREIGN KEY (message_id) REFERENCES Messages(id) ON DELETE CASCADE,
		FOREIGN KEY (file_id) REFERENCES Files(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS ToolCalls (
		id TEXT PRIMARY KEY,
		reference_id TEXT NOT NULL,
		conv_id TEXT NOT NULL,
		message_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		args TEXT NOT NULL,
		output TEXT,
		FOREIGN KEY (conv_id) REFERENCES Conversations(id) ON DELETE CASCADE,
		FOREIGN KEY (message_id) REFERENCES Messages(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS Tools (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT NOT NULL,
		mcp_server_id TEXT NOT NULL,
		input_schema TEXT,
		require_approval BOOLEAN NOT NULL DEFAULT 0,
		is_enabled BOOLEAN NOT NULL DEFAULT 1,
		FOREIGN KEY (mcp_server_id) REFERENCES MCPServers(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS MCPServers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		api_key TEXT NOT NULL, 
		user TEXT NOT NULL,
		FOREIGN KEY (user) REFERENCES Users(username) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS Providers (
		id TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		api_key TEXT NOT NULL, 
		user TEXT NOT NULL, 
		FOREIGN KEY (user) REFERENCES Users(username) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS Models (
		id TEXT PRIMARY KEY,
		provider_id TEXT NOT NULL,
		name TEXT NOT NULL,
		is_enabled BOOLEAN NOT NULL DEFAULT 1,
		FOREIGN KEY (provider_id) REFERENCES Providers(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS Settings (
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		user TEXT NOT NULL,
		PRIMARY KEY (key, user),
		FOREIGN KEY (user) REFERENCES Users(username) ON DELETE CASCADE
	);
	`

	_, err = DB.Exec(schema)
	return err
}
