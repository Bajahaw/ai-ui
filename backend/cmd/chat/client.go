package chat

import (
	"ai-client/cmd/provider"
	"ai-client/cmd/tools"
	"database/sql"

	logger "github.com/charmbracelet/log"
)

var log *logger.Logger
var repo ConversationRepo
var toolCallsRepo tools.ToolCallsRepository
var providerClient provider.Client

func SetupChat(l *logger.Logger, db *sql.DB, pc provider.Client) {
	log = l
	providerClient = pc
	repo = newConversationRepository(db)
	toolCallsRepo = tools.NewToolCallsRepository(db)
	// SetDefaultSettings("admin")
}
