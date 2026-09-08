package tools

import (
	"database/sql"
	"sync"

	fs "github.com/Bajahaw/ai-ui/cmd/files"
	providers "github.com/Bajahaw/ai-ui/cmd/providers"
	stngs "github.com/Bajahaw/ai-ui/cmd/settings"
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
	settings          stngs.Repository
	providerRepo      providers.Repository
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
	settings = stngs.NewRepository(db)
	providerRepo = providers.NewRepository(db)
}

func SaveDefaultMCPServer(user string) {
	serverID := "default-" + user
	// Built-in tools must use the real server ID (default-{user}), not the
	// placeholder "default". A mismatch fails the Tools FK insert, so the
	// server appears with no tools until the user hits Refresh.
	builtInTools := GetBuiltInTools()
	for _, t := range builtInTools {
		t.MCPServerID = serverID
	}
	defaultServer := MCPServer{
		ID:    serverID,
		Name:  "Default Server",
		Tools: builtInTools,
		User:  user,
	}
	if err := mcps.Save(&defaultServer); err != nil {
		log.Error("Error saving default MCP server", "err", err, "user", user)
	}
}
