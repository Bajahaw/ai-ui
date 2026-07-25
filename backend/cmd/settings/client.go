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
// Safe when SetupSettings has not run (e.g. unit tests): returns ("", err).
func Get(key, user string) (string, error) {
	if repo == nil {
		return "", sql.ErrNoRows
	}
	return repo.Get(key, user)
}
