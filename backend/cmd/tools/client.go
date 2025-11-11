package tools

import (
	"database/sql"
	"sync"

	logger "github.com/charmbracelet/log"
)

var (
	log               *logger.Logger
	db                *sql.DB
	mcpRepo           MCPServerRepository
	toolRepo          ToolRepository
	toolCallsRepo     ToolCallsRepository
	mcpSessionManager MCPSessionManager
)

func SetUpTools(l *logger.Logger, database *sql.DB) {
	db = database
	toolCallsRepo = NewToolCallsRepository(db)
	toolRepo = NewToolRepository(db)
	mcpRepo = NewMCPRepository(db, toolRepo)
	mcpSessionManager = MCPSessionManager{
		sessions: sync.Map{},
	}
	log = l
}
