package chat

import (
	"ai-client/cmd/provider"
	"database/sql"

	logger "github.com/charmbracelet/log"
)

var log *logger.Logger
var repo ConversationRepo
var toolsRepo *ToolRepositoryImpl
var providerClient *provider.Client

func SetupChat(l *logger.Logger, db *sql.DB, pc *provider.Client) {
	log = l
	providerClient = pc
	repo = newConversationRepository(db)
	toolsRepo = NewToolRepository(db).(*ToolRepositoryImpl)
	setDefaultSettings()
}
