package chat

import (
	fs "ai-client/cmd/files"
	"ai-client/cmd/providers"
	stngs "ai-client/cmd/settings"
	"ai-client/cmd/tools"
	"database/sql"

	logger "github.com/charmbracelet/log"
)

var log *logger.Logger
var conversations ConversationRepo
var toolCalls tools.ToolCallsRepository
var provider providers.Client
var settings stngs.Repository
var files fs.Repository

func SetupChat(
	l *logger.Logger,
	db *sql.DB,
	p providers.Client,
) {
	log = l
	provider = p
	conversations = NewRepository(db)
	toolCalls = tools.NewToolCallsRepository(db)
	settings = stngs.NewRepository(db)
	files = fs.NewRepository(db)
}
