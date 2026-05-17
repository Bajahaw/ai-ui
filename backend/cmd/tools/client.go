package tools

import (
	"database/sql"
	"sync"

	fs "github.com/Bajahaw/ai-ui/cmd/files"
	logger "github.com/charmbracelet/log"
)

var (
	log               *logger.Logger
	db                *sql.DB
	mcps              MCPServerRepository
	tools             ToolRepository
	toolCalls         ToolCallsRepository
	mcpSessionManager MCPSessionManager
	files             fs.Repository
)

func SetUpTools(l *logger.Logger, database *sql.DB) {
	db = database
	toolCalls = NewToolCallsRepository(db)
	tools = NewToolRepository(db)
	mcps = NewMCPRepository(db, tools)
	mcpSessionManager = MCPSessionManager{
		sessions: sync.Map{},
	}
	log = l
	files = fs.NewRepository(db)

	// // might get unique constraint error but that's fine
	// _ = mcpRepo.SaveMCPServer(MCPServer{
	// 	ID:    "default",
	// 	Name:  "Default Server",
	// 	Tools: GetBuiltInTools(),
	// 	User:  "admin",
	// })
}

func SaveDefaultMCPServer(user string) {
	defaultServer := MCPServer{
		ID:    "default-" + user,
		Name:  "Default Server",
		Tools: GetBuiltInTools(),
		User:  user,
	}
	mcps.Save(&defaultServer)
}
