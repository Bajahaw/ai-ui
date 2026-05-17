package data

import (
	"database/sql"
	"os"
	"path"
	"testing"
	"time"
)

func TestRunMigrations_FreshDB(t *testing.T) {
	// Create a temporary file for the database
	tmpDir := t.TempDir()
	dbPath := path.Join(tmpDir, "test.db")

	// Create an empty DB connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()
	defer os.RemoveAll(tmpDir)

	// Give a little time buffer for database unlocking
	defer time.Sleep(100 * time.Millisecond)

	// Run migrations
	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify user_version is 2
	var userVersion int
	err = db.QueryRow("PRAGMA user_version;").Scan(&userVersion)
	if err != nil {
		t.Fatalf("Failed to get user_version: %v", err)
	}

	if userVersion != 5 {
		t.Errorf("Expected user_version to be 5, got %d", userVersion)
	}

	// Verify new columns exist
	var hasHeadersJson bool
	err = db.QueryRow("SELECT COUNT(*) > 0 FROM pragma_table_info('Providers') WHERE name='headers_json';").Scan(&hasHeadersJson)
	if err != nil {
		t.Fatalf("Failed to check Providers column: %v", err)
	}
	if !hasHeadersJson {
		t.Error("Expected headers_json column in Providers table, but it was not found")
	}

	err = db.QueryRow("SELECT COUNT(*) > 0 FROM pragma_table_info('MCPServers') WHERE name='headers_json';").Scan(&hasHeadersJson)
	if err != nil {
		t.Fatalf("Failed to check MCPServers column: %v", err)
	}
	if !hasHeadersJson {
		t.Error("Expected headers_json column in MCPServers table, but it was not found")
	}
}

func TestRunMigrations_UpgradeFromV1(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := path.Join(tmpDir, "test_v1.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()
	defer os.RemoveAll(tmpDir)

	// Simulate a v1 database setup manually
	schemaV1 := `
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
		status TEXT NOT NULL DEFAULT 'pending',
		speed REAL DEFAULT 0,
		token_count INTEGER DEFAULT 0,
		context_size INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
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
		token_count INTEGER DEFAULT 0,
		context_size INTEGER DEFAULT 0,
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


	PRAGMA user_version = 1;
	`
	if _, err := db.Exec(schemaV1); err != nil {
		t.Fatalf("Failed to manually create v1 schema: %v", err)
	}

	// Insert dummy data
	if _, err := db.Exec("INSERT INTO Providers (id, url, api_key, user) VALUES ('1', 'http://a', 'key', 'u')"); err != nil {
		t.Fatalf("Failed to insert dummy v1 data: %v", err)
	}

	// Run migrations (should only execute v2 path)
	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("Failed to upgrade database: %v", err)
	}

	// Check updated version
	var userVersion int
	if err := db.QueryRow("PRAGMA user_version;").Scan(&userVersion); err != nil {
		t.Fatalf("Failed to retrieve user version: %v", err)
	}
	if userVersion != 5 {
		t.Errorf("Expected bumped version to be 5, got %d", userVersion)
	}

	// Verify headers_json was added and old data is intact
	var headers string
	var id string
	if err := db.QueryRow("SELECT id, headers_json FROM Providers WHERE id = '1'").Scan(&id, &headers); err != nil {
		t.Fatalf("Failed to query migrated Providers table: %v", err)
	}
	if headers != "{}" {
		t.Errorf("Expected '{}' as DEFAULT for headers_json, got %q", headers)
	}
}
