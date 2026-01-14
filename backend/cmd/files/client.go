package files

import (
	"ai-client/cmd/providers"
	stngs "ai-client/cmd/settings"
	"database/sql"

	logger "github.com/charmbracelet/log"
)

var log *logger.Logger
var db *sql.DB
var provider providers.Client
var repo Repository
var settings stngs.Repository

func SetupFiles(
	l *logger.Logger,
	d *sql.DB,
	pc providers.Client,
) {
	log = l
	db = d
	provider = pc
	settings = stngs.NewRepository(db)
	repo = NewRepository(db)
}
