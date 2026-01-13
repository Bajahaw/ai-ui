package chat

import (
	"ai-client/cmd/provider"
	"ai-client/cmd/tools"
	"database/sql"

	logger "github.com/charmbracelet/log"
)

var log *logger.Logger
var conversations ConversationRepo
var toolCalls tools.ToolCallsRepository
var providerClient provider.Client

func SetupChat(l *logger.Logger, db *sql.DB, pc provider.Client) {
	log = l
	providerClient = pc
	conversations = newConversationRepository(db)
	toolCalls = tools.NewToolCallsRepository(db)
	// SetDefaultSettings("admin")
}
