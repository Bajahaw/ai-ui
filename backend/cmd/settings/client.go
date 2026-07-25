package settings

import (
	"database/sql"

	logger "github.com/charmbracelet/log"
)

var log *logger.Logger
var db *sql.DB
var repo Repository

func SetupSettings(l *logger.Logger, d *sql.DB) {
	log = l
	db = d
	repo = NewRepository(db)
}

// Get returns a single setting value for the user.
func Get(key, user string) (string, error) {
	return repo.Get(key, user)
}
