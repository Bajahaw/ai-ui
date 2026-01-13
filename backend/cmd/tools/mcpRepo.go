package tools

import "database/sql"

type MCPServerRepository interface {
	GetAll(user string) []*MCPServer
	GetByID(id string, user string) (*MCPServer, error)
	Save(server *MCPServer) error
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
	query := `SELECT id, name, endpoint, api_key FROM MCPServers WHERE user = ?`
	rows, err := repo.db.Query(query, user)
	if err != nil {
		log.Error("Error querying MCP servers", "err", err)
		return allServers
	}
	defer rows.Close()

	for rows.Next() {
		var server MCPServer
		if err := rows.Scan(&server.ID, &server.Name, &server.Endpoint, &server.APIKey); err != nil {
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
	query := `SELECT id, name, endpoint, api_key FROM MCPServers WHERE id = ? AND user = ?`
	row := repo.db.QueryRow(query, id, user)
	if err := row.Scan(&server.ID, &server.Name, &server.Endpoint, &server.APIKey); err != nil {
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

func (repo *MCPRepositoryImpl) Save(server *MCPServer) error {
	query := `INSERT INTO MCPServers (id, name, endpoint, api_key, user) VALUES (?, ?, ?, ?, ?)`
	_, err := repo.db.Exec(query, server.ID, server.Name, server.Endpoint, server.APIKey, server.User)
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
