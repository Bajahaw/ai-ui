package tools

import "database/sql"

type MCPServerRepository interface {
	GetAllMCPServers() []MCPServer
	GetMCPServer(id string) (MCPServer, error)
	SaveMCPServer(server MCPServer) error
	DeleteMCPServer(id string) error
}

type MCPRepositoryImpl struct {
	db       *sql.DB
	toolRepo ToolRepository
}

func NewMCPRepository(db *sql.DB, toolRepo ToolRepository) MCPServerRepository {
	return &MCPRepositoryImpl{db: db, toolRepo: toolRepo}
}

func (repo *MCPRepositoryImpl) GetAllMCPServers() []MCPServer {
	var allServers = make([]MCPServer, 0)
	query := `SELECT id, name, endpoint, api_key FROM MCPServers`
	rows, err := repo.db.Query(query)
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
		allServers = append(allServers, server)
	}

	tools := repo.toolRepo.GetAllTools()
	toolMap := make(map[string][]Tool)
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

func (repo *MCPRepositoryImpl) GetMCPServer(id string) (MCPServer, error) {
	var server MCPServer
	query := `SELECT id, name, endpoint, api_key FROM MCPServers WHERE id = ?`
	err := repo.db.QueryRow(query, id).Scan(&server.ID, &server.Name, &server.Endpoint, &server.APIKey)
	if err != nil {
		return MCPServer{}, err
	}

	server.Tools = repo.toolRepo.GetToolsByMCPServerID(server.ID)

	return server, nil
}

func (repo *MCPRepositoryImpl) SaveMCPServer(server MCPServer) error {
	query := `INSERT INTO MCPServers (id, name, endpoint, api_key) VALUES (?, ?, ?, ?)`
	_, err := repo.db.Exec(query, server.ID, server.Name, server.Endpoint, server.APIKey)
	if err != nil {
		return err
	}

	// Save associated tools
	for _, tool := range server.Tools {
		if err := repo.toolRepo.SaveTool(tool); err != nil {
			return err
		}
	}

	return nil
}

func (repo *MCPRepositoryImpl) DeleteMCPServer(id string) error {
	query := `DELETE FROM MCPServers WHERE id = ?`
	_, err := repo.db.Exec(query, id)
	if err != nil {
		return err
	}
	return nil
}
