package tools

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/encryption"
	"golang.org/x/oauth2"
)

const mcpServerColumns = `id, name, endpoint, api_key, headers_json, auth_type, oauth_json`

// scanMCPServer reads a row selected with mcpServerColumns and decrypts its credentials.
func scanMCPServer(row interface{ Scan(...any) error }, server *MCPServer) error {
	var headersJson, oauthJson string
	if err := row.Scan(&server.ID, &server.Name, &server.Endpoint, &server.APIKey, &headersJson, &server.AuthType, &oauthJson); err != nil {
		return err
	}
	decodeServerSecrets(server, headersJson, oauthJson)
	return nil
}

// decodeServerSecrets decrypts the API key, headers and OAuth settings read from the database.
func decodeServerSecrets(server *MCPServer, headersJson, oauthJson string) {
	if err := encryption.DecryptInPlace(&server.APIKey, &headersJson, &oauthJson); err != nil {
		log.Error("Failed to decrypt MCP server credentials", "server", server.ID, "err", err)
		server.APIKey, headersJson, oauthJson = "", "", ""
	}
	var headers map[string]string
	if headersJson != "" {
		_ = json.Unmarshal([]byte(headersJson), &headers)
	}
	if headers == nil {
		headers = make(map[string]string)
	}
	server.Headers = headers
	server.OAuth = nil
	if oauthJson != "" {
		var o MCPOAuth
		if err := json.Unmarshal([]byte(oauthJson), &o); err == nil {
			server.OAuth = &o
		}
	}
	if server.AuthType == AuthTypeOAuth2 && server.OAuth == nil {
		server.OAuth = &MCPOAuth{}
	}
}

type MCPServerRepository interface {
	GetAll(user string) []*MCPServer
	GetByID(id string, user string) (*MCPServer, error)
	Save(server *MCPServer) error
	Update(server *MCPServer) error
	UpdateOAuth(server *MCPServer) error
	UpdateOAuthToken(id, user string, connectedAt time.Time, token *oauth2.Token) error
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
	query := `SELECT ` + mcpServerColumns + ` FROM MCPServers WHERE user = ?`
	rows, err := repo.db.Query(query, user)
	if err != nil {
		log.Error("Error querying MCP servers", "err", err)
		return allServers
	}
	defer rows.Close()

	for rows.Next() {
		var server MCPServer
		if err := scanMCPServer(rows, &server); err != nil {
			log.Error("Error scanning MCP server", "err", err)
			continue
		}
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
	query := `SELECT ` + mcpServerColumns + ` FROM MCPServers WHERE id = ? AND user = ?`
	if err := scanMCPServer(repo.db.QueryRow(query, id, user), &server); err != nil {
		return &server, err
	}
	server.User = user

	tools := repo.toolRepo.GetAll(user)
	for _, tool := range tools {
		if tool.MCPServerID == server.ID {
			server.Tools = append(server.Tools, tool)
		}
	}
	return &server, nil
}

// encryptedSecrets returns the server's API key, headers and OAuth settings encrypted for storage.
func encryptedSecrets(server *MCPServer) (apiKey, headersJson, oauthJson string, err error) {
	if server.Headers == nil {
		server.Headers = make(map[string]string)
	}
	headersBytes, _ := json.Marshal(server.Headers)
	if headersJson, err = encryption.Encrypt(string(headersBytes)); err != nil {
		return
	}
	if oauthJson, err = encryptedOAuth(server.OAuth); err != nil {
		return
	}
	apiKey, err = encryption.Encrypt(server.APIKey)
	return
}

func encryptedOAuth(o *MCPOAuth) (string, error) {
	if o == nil {
		return "", nil
	}
	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	return encryption.Encrypt(string(b))
}

// Update changes the server's connection settings. Tools are not touched.
func (repo *MCPRepositoryImpl) Update(server *MCPServer) error {
	apiKey, headersJson, oauthJson, err := encryptedSecrets(server)
	if err != nil {
		return err
	}
	res, err := repo.db.Exec(
		`UPDATE MCPServers SET name = ?, endpoint = ?, api_key = ?, headers_json = ?, auth_type = ?, oauth_json = ? WHERE id = ? AND user = ?`,
		server.Name, server.Endpoint, apiKey, headersJson, server.AuthType, oauthJson, server.ID, server.User,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateOAuth stores only the OAuth settings and session.
func (repo *MCPRepositoryImpl) UpdateOAuth(server *MCPServer) error {
	oauthJson, err := encryptedOAuth(server.OAuth)
	if err != nil {
		return err
	}
	res, err := repo.db.Exec(`UPDATE MCPServers SET oauth_json = ? WHERE id = ? AND user = ?`, oauthJson, server.ID, server.User)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateOAuthToken stores a refreshed token, unless the session it belongs to
// has since been replaced by a new authorization.
func (repo *MCPRepositoryImpl) UpdateOAuthToken(id, user string, connectedAt time.Time, token *oauth2.Token) error {
	server, err := repo.GetByID(id, user)
	if err != nil {
		return err
	}
	if !server.OAuth.connected() || !server.OAuth.Session.ConnectedAt.Equal(connectedAt) {
		return nil
	}
	server.OAuth.Session.Token = token
	return repo.UpdateOAuth(server)
}

func (repo *MCPRepositoryImpl) Save(server *MCPServer) error {
	apiKey, headersJson, oauthJson, err := encryptedSecrets(server)
	if err != nil {
		return err
	}

	query := `INSERT INTO MCPServers (id, name, endpoint, api_key, user, headers_json, auth_type, oauth_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = repo.db.Exec(query, server.ID, server.Name, server.Endpoint, apiKey, server.User, headersJson, server.AuthType, oauthJson)
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
