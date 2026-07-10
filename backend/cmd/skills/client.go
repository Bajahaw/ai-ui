package skills

import (
	"database/sql"

	logger "github.com/charmbracelet/log"
)

var (
	log  *logger.Logger
	db   *sql.DB
	repo Repository
)

func SetupSkills(l *logger.Logger, database *sql.DB) {
	log = l
	db = database
	repo = NewRepository(db)
}

// GetAll returns all skills for a user (including content).
// Used by the built-in AI tools to list/read skills.
func GetAll(user string) []*Skill {
	return repo.GetAll(user)
}

// GetByName returns a single skill by name for a user.
func GetByName(name, user string) (*Skill, error) {
	return repo.GetByName(name, user)
}
