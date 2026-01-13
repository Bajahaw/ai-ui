package tools

import (
	"database/sql"
	"sync"

	logger "github.com/charmbracelet/log"
)

var (
	log               *logger.Logger
	db                *sql.DB
	mcps              MCPServerRepository
	tools             ToolRepository
	toolCalls         ToolCallsRepository
	mcpSessionManager MCPSessionManager
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
		ID:    "default",
		Name:  "Default Server",
		Tools: GetBuiltInTools(),
		User:  user,
	}
	mcps.Save(&defaultServer)
}
