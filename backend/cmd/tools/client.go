package tools

import (
	"database/sql"

	logger "github.com/charmbracelet/log"
)

var (
	log           *logger.Logger
	db            *sql.DB
	mcpRepo       MCPServerRepository
	toolRepo      ToolRepository
	toolCallsRepo ToolCallsRepository
)

func SetUpTools(l *logger.Logger, database *sql.DB) {
	db = database
	toolCallsRepo = NewToolCallsRepository(db)
	toolRepo = NewToolRepository(db)
	mcpRepo = NewMCPRepository(db, toolRepo)
	log = l
}
