package chat

import (
	fs "github.com/Bajahaw/ai-ui/cmd/files"
	"github.com/Bajahaw/ai-ui/cmd/providers"
	stngs "github.com/Bajahaw/ai-ui/cmd/settings"
	"github.com/Bajahaw/ai-ui/cmd/tools"
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
