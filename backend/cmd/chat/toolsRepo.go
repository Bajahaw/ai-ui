package chat

import (
	"ai-client/cmd/provider"
	"database/sql"
)

type ToolRepository interface {
	SaveToolCall(toolCall provider.ToolCall) error
	GetToolCallsByMessageID(messageID int) []*provider.ToolCall
	GetToolCallsByConvID(convID string) []*provider.ToolCall
}

type ToolRepositoryImpl struct {
	db *sql.DB
}

func NewToolRepository(db *sql.DB) ToolRepository {
	return &ToolRepositoryImpl{db: db}
}

func (repo *ToolRepositoryImpl) SaveToolCall(toolCall provider.ToolCall) error {
	sql := `INSERT INTO ToolCalls (id, conv_id, message_id, name, args, output) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := repo.db.Exec(sql, toolCall.ID, toolCall.ConvID, toolCall.MessageID, toolCall.Name, toolCall.Args, toolCall.Output)
	return err
}

func (repo *ToolRepositoryImpl) GetToolCallsByMessageID(messageID int) []*provider.ToolCall {
	sql := `SELECT id, name, args, output FROM ToolCalls WHERE message_id = ?`
	var toolCalls = make([]*provider.ToolCall, 0)

	rows, err := repo.db.Query(sql, messageID)
	if err != nil {
		log.Error("Error querying tool calls", "err", err)
		return toolCalls
	}

	defer rows.Close()
	for rows.Next() {
		var toolCall provider.ToolCall
		if err := rows.Scan(
			&toolCall.ID,
			&toolCall.Name,
			&toolCall.Args,
			&toolCall.Output,
		); err != nil {
			log.Error("Error scanning tool call", "err", err)
			return toolCalls
		}

		toolCalls = append(toolCalls, &toolCall)
	}
	return toolCalls
}

func (repo *ToolRepositoryImpl) GetToolCallsByConvID(convID string) []*provider.ToolCall {
	sql := `SELECT id, message_id, name, args, output FROM ToolCalls WHERE conv_id = ?`
	var toolCalls = make([]*provider.ToolCall, 0)

	rows, err := repo.db.Query(sql, convID)
	if err != nil {
		log.Error("Error querying tool calls", "err", err)
		return toolCalls
	}

	defer rows.Close()
	for rows.Next() {
		var toolCall provider.ToolCall
		if err := rows.Scan(
			&toolCall.ID,
			&toolCall.MessageID,
			&toolCall.Name,
			&toolCall.Args,
			&toolCall.Output,
		); err != nil {
			log.Error("Error scanning tool call", "err", err)
			return toolCalls
		}

		toolCalls = append(toolCalls, &toolCall)
	}
	return toolCalls
}
