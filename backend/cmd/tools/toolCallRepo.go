package tools

import (
	"database/sql"
)

type ToolCallsRepository interface {
	SaveToolCall(toolCall ToolCall) error
	GetToolCallsByMessageID(messageID int) []*ToolCall
	GetToolCallsByConvID(convID string) []*ToolCall
}

type ToolCallsRepositoryImpl struct {
	db *sql.DB
}

func NewToolCallsRepository(db *sql.DB) ToolCallsRepository {
	return &ToolCallsRepositoryImpl{db: db}
}

func (repo *ToolCallsRepositoryImpl) SaveToolCall(toolCall ToolCall) error {
	query := `INSERT INTO ToolCalls (id, conv_id, message_id, name, args, output) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := repo.db.Exec(query, toolCall.ID, toolCall.ConvID, toolCall.MessageID, toolCall.Name, toolCall.Args, toolCall.Output)
	return err
}

func (repo *ToolCallsRepositoryImpl) GetToolCallsByMessageID(messageID int) []*ToolCall {
	query := `SELECT id, name, args, output FROM ToolCalls WHERE message_id = ?`
	var toolCalls = make([]*ToolCall, 0)

	rows, err := repo.db.Query(query, messageID)
	if err != nil {
		log.Error("Error querying tool calls", "err", err)
		return toolCalls
	}

	defer rows.Close()
	for rows.Next() {
		var toolCall ToolCall
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

func (repo *ToolCallsRepositoryImpl) GetToolCallsByConvID(convID string) []*ToolCall {
	query := `SELECT id, message_id, name, args, output FROM ToolCalls WHERE conv_id = ?`
	var toolCalls = make([]*ToolCall, 0)

	rows, err := repo.db.Query(query, convID)
	if err != nil {
		log.Error("Error querying tool calls", "err", err)
		return toolCalls
	}

	defer rows.Close()
	for rows.Next() {
		var toolCall ToolCall
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
