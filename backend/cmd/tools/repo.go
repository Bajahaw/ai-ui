package tools

import "database/sql"

type ToolRepository interface {
	GetAllTools() []Tool
	GetToolsByMCPServerID(mcpID string) []Tool
	GetTool(id string) (Tool, error)
	SaveTool(tool Tool) error
	SaveListOfTools(tools []Tool) error
	DeleteTool(id string) error
}

type ToolRepositoryImpl struct {
	db *sql.DB
}

func NewToolRepository(db *sql.DB) ToolRepository {
	return &ToolRepositoryImpl{db: db}
}

func (repo *ToolRepositoryImpl) GetAllTools() []Tool {
	var allTools = make([]Tool, 0)
	sql := `SELECT id, mcp_server_id, name, description, input_schema, require_approval, is_enabled FROM Tools`
	rows, err := repo.db.Query(sql)

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
		allTools = append(allTools, tool)
	}

	return allTools
}

func (repo *ToolRepositoryImpl) GetToolsByMCPServerID(mcpID string) []Tool {
	var tools = make([]Tool, 0)
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
		tools = append(tools, tool)
	}

	return tools
}

func (repo *ToolRepositoryImpl) GetTool(id string) (Tool, error) {
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
		return Tool{}, err
	}
	return tool, nil
}

func (repo *ToolRepositoryImpl) SaveTool(tool Tool) error {
	sql := `INSERT INTO Tools (id, mcp_server_id, name, description, input_schema, require_approval, is_enabled) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := repo.db.Exec(sql, tool.ID, tool.MCPServerID, tool.Name, tool.Description, tool.InputSchema, tool.RequireApproval, tool.IsEnabled)
	if err != nil {
		return err
	}
	return nil
}

func (repo *ToolRepositoryImpl) SaveListOfTools(tools []Tool) error {
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

func (repo *ToolRepositoryImpl) DeleteTool(id string) error {
	sql := `DELETE FROM Tools WHERE id = ?`
	_, err := repo.db.Exec(sql, id)
	if err != nil {
		return err
	}
	return nil
}
