package tools

import "database/sql"

type ToolRepository interface {
	GetAll(user string) []*Tool
	GetAllByMCPServerID(mcpID string) []*Tool
	GetByName(name string) (*Tool, error)
	GetByID(id string) (*Tool, error)
	Save(tool *Tool) error
	SaveAll(tools []*Tool) error
	DeleteByID(id string) error
}

type ToolRepositoryImpl struct {
	db *sql.DB
}

func NewToolRepository(db *sql.DB) ToolRepository {
	return &ToolRepositoryImpl{db: db}
}

func (repo *ToolRepositoryImpl) GetAll(user string) []*Tool {
	var allTools = make([]*Tool, 0)
	sql := `
		SELECT t.id, t.mcp_server_id, t.name, t.description, t.input_schema, t.require_approval, t.is_enabled 
		FROM Tools t
		JOIN MCPServers m ON t.mcp_server_id = m.id
		WHERE m.user = ?
	`
	rows, err := repo.db.Query(sql, user)

	if err != nil {
		log.Error("Error querying tools", "err", err)
		return allTools
	}

	defer rows.Close()
	for rows.Next() {
		var tool Tool
		if err := rows.Scan(
			&tool.ID,
			&tool.MCPServerID,
			&tool.Name,
			&tool.Description,
			&tool.InputSchema,
			&tool.RequireApproval,
			&tool.IsEnabled,
		); err != nil {
			log.Error("Error scanning tool", "err", err)
			continue
		}
		allTools = append(allTools, &tool)
	}

	return allTools
}

func (repo *ToolRepositoryImpl) GetByName(name string) (*Tool, error) {
	var tool Tool
	sql := `SELECT id, mcp_server_id, name, description, input_schema, require_approval, is_enabled FROM Tools WHERE name = ?`
	err := repo.db.QueryRow(sql, name).Scan(
		&tool.ID,
		&tool.MCPServerID,
		&tool.Name,
		&tool.Description,
		&tool.InputSchema,
		&tool.RequireApproval,
		&tool.IsEnabled)
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

func (repo *ToolRepositoryImpl) GetAllByMCPServerID(mcpID string) []*Tool {
	var tools = make([]*Tool, 0)
	sql := `SELECT id, mcp_server_id, name, description, input_schema, require_approval, is_enabled FROM Tools WHERE mcp_server_id = ?`
	rows, err := repo.db.Query(sql, mcpID)
	if err != nil {
		log.Error("Error querying tools by MCPServerID", "err", err)
		return tools
	}
	defer rows.Close()

	for rows.Next() {
		var tool Tool
		if err := rows.Scan(
			&tool.ID,
			&tool.MCPServerID,
			&tool.Name,
			&tool.Description,
			&tool.InputSchema,
			&tool.RequireApproval,
			&tool.IsEnabled,
		); err != nil {
			log.Error("Error scanning tool", "err", err)
			continue
		}
		tools = append(tools, &tool)
	}

	return tools
}

func (repo *ToolRepositoryImpl) GetByID(id string) (*Tool, error) {
	var tool Tool
	sql := `SELECT id, mcp_server_id, name, description, input_schema, require_approval, is_enabled FROM Tools WHERE id = ?`
	err := repo.db.QueryRow(sql, id).Scan(
		&tool.ID,
		&tool.MCPServerID,
		&tool.Name,
		&tool.Description,
		&tool.InputSchema,
		&tool.RequireApproval,
		&tool.IsEnabled)
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

func (repo *ToolRepositoryImpl) Save(tool *Tool) error {
	sql := `INSERT INTO Tools (id, mcp_server_id, name, description, input_schema, require_approval, is_enabled) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := repo.db.Exec(sql, tool.ID, tool.MCPServerID, tool.Name, tool.Description, tool.InputSchema, tool.RequireApproval, tool.IsEnabled)
	if err != nil {
		return err
	}
	return nil
}

func (repo *ToolRepositoryImpl) SaveAll(tools []*Tool) error {
	sql := `
	INSERT INTO Tools (id, mcp_server_id, name, description, input_schema, require_approval, is_enabled)
	VALUES (?, ?, ?, ?, ?, ?, ?) 
	ON CONFLICT(id) DO UPDATE SET require_approval=excluded.require_approval, is_enabled=excluded.is_enabled`

	// TODO: use one query
	for _, tool := range tools {
		if _, err := repo.db.Exec(sql,
			tool.ID,
			tool.MCPServerID,
			tool.Name,
			tool.Description,
			tool.InputSchema,
			tool.RequireApproval,
			tool.IsEnabled,
		); err != nil {
			return err
		}
	}
	return nil
}

func (repo *ToolRepositoryImpl) DeleteByID(id string) error {
	sql := `DELETE FROM Tools WHERE id = ?`
	_, err := repo.db.Exec(sql, id)
	if err != nil {
		return err
	}
	return nil
}
