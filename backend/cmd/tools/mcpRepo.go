package tools

import (
	"database/sql"
	"encoding/json"

	"github.com/Bajahaw/ai-ui/cmd/encryption"
)

// decodeServerSecrets decrypts the API key and headers read from the database.
func decodeServerSecrets(server *MCPServer, headersJson string) {
	if err := encryption.DecryptInPlace(&server.APIKey, &headersJson); err != nil {
		log.Error("Failed to decrypt MCP server credentials", "server", server.ID, "err", err)
		server.APIKey, headersJson = "", ""
	}
	var headers map[string]string
	if headersJson != "" {
		_ = json.Unmarshal([]byte(headersJson), &headers)
	}
	if headers == nil {
		headers = make(map[string]string)
	}
	server.Headers = headers
}

type MCPServerRepository interface {
	GetAll(user string) []*MCPServer
	GetByID(id string, user string) (*MCPServer, error)
	Save(server *MCPServer) error
	Update(server *MCPServer) error
	DeleteByID(id string, user string) error
}

type MCPRepositoryImpl struct {
	db       *sql.DB
	toolRepo ToolRepository
}

func NewMCPRepository(db *sql.DB, toolRepo ToolRepository) MCPServerRepository {
	return &MCPRepositoryImpl{db: db, toolRepo: toolRepo}
}

func (repo *MCPRepositoryImpl) GetAll(user string) []*MCPServer {
	var allServers = make([]*MCPServer, 0)
	query := `SELECT id, name, endpoint, api_key, headers_json FROM MCPServers WHERE user = ?`
	rows, err := repo.db.Query(query, user)
	if err != nil {
		log.Error("Error querying MCP servers", "err", err)
		return allServers
	}
	defer rows.Close()

	for rows.Next() {
		var server MCPServer
		var headersJson string
		if err := rows.Scan(&server.ID, &server.Name, &server.Endpoint, &server.APIKey, &headersJson); err != nil {
			log.Error("Error scanning MCP server", "err", err)
			continue
		}
		decodeServerSecrets(&server, headersJson)
		server.User = user
		allServers = append(allServers, &server)
	}

	tools := repo.toolRepo.GetAll(user)
	toolMap := make(map[string][]*Tool)
	for _, tool := range tools {
		if tool.MCPServerID != "" {
			toolMap[tool.MCPServerID] = append(toolMap[tool.MCPServerID], tool)
		}
	}

	for i := range allServers {
		allServers[i].Tools = toolMap[allServers[i].ID]
	}
	return allServers
}

func (repo *MCPRepositoryImpl) GetByID(id string, user string) (*MCPServer, error) {
	var server MCPServer
	var headersJson string
	query := `SELECT id, name, endpoint, api_key, headers_json FROM MCPServers WHERE id = ? AND user = ?`
	row := repo.db.QueryRow(query, id, user)
	if err := row.Scan(&server.ID, &server.Name, &server.Endpoint, &server.APIKey, &headersJson); err != nil {
		return &server, err
	}
	decodeServerSecrets(&server, headersJson)
	server.User = user

	tools := repo.toolRepo.GetAll(user)
	for _, tool := range tools {
		if tool.MCPServerID == server.ID {
			server.Tools = append(server.Tools, tool)
		}
	}
	return &server, nil
}

// encryptedSecrets returns the server's API key and headers encrypted for storage.
func encryptedSecrets(server *MCPServer) (apiKey, headersJson string, err error) {
	if server.Headers == nil {
		server.Headers = make(map[string]string)
	}
	headersBytes, _ := json.Marshal(server.Headers)
	if headersJson, err = encryption.Encrypt(string(headersBytes)); err != nil {
		return
	}
	apiKey, err = encryption.Encrypt(server.APIKey)
	return
}

// Update changes the server's connection settings. Tools are not touched.
func (repo *MCPRepositoryImpl) Update(server *MCPServer) error {
	apiKey, headersJson, err := encryptedSecrets(server)
	if err != nil {
		return err
	}
	res, err := repo.db.Exec(
		`UPDATE MCPServers SET name = ?, endpoint = ?, api_key = ?, headers_json = ? WHERE id = ? AND user = ?`,
		server.Name, server.Endpoint, apiKey, headersJson, server.ID, server.User,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (repo *MCPRepositoryImpl) Save(server *MCPServer) error {
	apiKey, headersJson, err := encryptedSecrets(server)
	if err != nil {
		return err
	}

	query := `INSERT INTO MCPServers (id, name, endpoint, api_key, user, headers_json) VALUES (?, ?, ?, ?, ?, ?)`
	_, err = repo.db.Exec(query, server.ID, server.Name, server.Endpoint, apiKey, server.User, headersJson)
	if err != nil {
		return err
	}

	// Save associated tools
	err = repo.toolRepo.SaveAll(server.Tools)
	if err != nil {
		return err
	}

	return nil
}

func (repo *MCPRepositoryImpl) DeleteByID(id string, user string) error {
	_, err := repo.db.Exec(`DELETE FROM MCPServers WHERE id = ? AND user = ?`, id, user)
	return err
}
