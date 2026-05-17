package tools

import (
	"database/sql"

	"github.com/Bajahaw/ai-ui/cmd/providers"
)

type ToolCallsRepository interface {
	Save(toolCall *providers.ToolCall) error
	GetAllByMessageID(messageID int) []*providers.ToolCall
	GetAllByConvID(convID string) []*providers.ToolCall
}

type ToolCallsRepositoryImpl struct {
	db *sql.DB
}

func NewToolCallsRepository(db *sql.DB) ToolCallsRepository {
	return &ToolCallsRepositoryImpl{db: db}
}

func (repo *ToolCallsRepositoryImpl) Save(toolCall *providers.ToolCall) error {
	query := `INSERT INTO ToolCalls (id, reference_id, conv_id, message_id, name, args, output, file_id, token_count, context_size) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := repo.db.Exec(query, toolCall.ID, toolCall.ReferenceID, toolCall.ConvID, toolCall.MessageID, toolCall.Name, toolCall.Args, toolCall.Output, toolCall.File, toolCall.TokenCount, toolCall.ContextSize)
	return err
}

func (repo *ToolCallsRepositoryImpl) GetAllByMessageID(messageID int) []*providers.ToolCall {
	query := `SELECT id, reference_id, name, args, output, file_id, token_count, context_size FROM ToolCalls WHERE message_id = ?`
	var toolCalls = make([]*providers.ToolCall, 0)

	rows, err := repo.db.Query(query, messageID)
	if err != nil {
		log.Error("Error querying tool calls", "err", err)
		return toolCalls
	}

	defer rows.Close()
	for rows.Next() {
		var toolCall providers.ToolCall
		if err := rows.Scan(
			&toolCall.ID,
			&toolCall.ReferenceID,
			&toolCall.Name,
			&toolCall.Args,
			&toolCall.Output,
			&toolCall.File,
			&toolCall.TokenCount,
			&toolCall.ContextSize,
		); err != nil {
			log.Error("Error scanning tool call", "err", err)
			return toolCalls
		}

		toolCalls = append(toolCalls, &toolCall)
	}
	return toolCalls
}

func (repo *ToolCallsRepositoryImpl) GetAllByConvID(convID string) []*providers.ToolCall {
	query := `SELECT id, reference_id, message_id, name, args, output, file_id, token_count, context_size FROM ToolCalls WHERE conv_id = ?`
	var toolCalls = make([]*providers.ToolCall, 0)

	rows, err := repo.db.Query(query, convID)
	if err != nil {
		log.Error("Error querying tool calls", "err", err)
		return toolCalls
	}

	defer rows.Close()
	for rows.Next() {
		var toolCall providers.ToolCall
		if err := rows.Scan(
			&toolCall.ID,
			&toolCall.ReferenceID,
			&toolCall.MessageID,
			&toolCall.Name,
			&toolCall.Args,
			&toolCall.Output,
			&toolCall.File,
			&toolCall.TokenCount,
			&toolCall.ContextSize,
		); err != nil {
			log.Error("Error scanning tool call", "err", err)
			return toolCalls
		}

		toolCalls = append(toolCalls, &toolCall)
	}
	return toolCalls
}
