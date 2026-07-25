package skills

import (
	"database/sql"
	"sort"
	"strings"

	"github.com/Bajahaw/ai-ui/cmd/settings"
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
	// Warm the builtin cache so first chat doesn't pay parse cost.
	_ = loadBuiltins()
}

// builtinsEnabled reports whether this user has built-in skills turned on.
// Default is enabled when the setting is missing (matches SetDefaults).
func builtinsEnabled(user string) bool {
	v, err := settings.Get("enableBuiltinSkills", user)
	if err != nil || v == "" {
		return true
	}
	return !strings.EqualFold(v, "false")
}

// GetAvailable returns skills the model may use for this user:
// user skills always; builtins only when enableBuiltinSkills is true;
// user skill with the same name shadows the builtin entirely.
func GetAvailable(user string) []*Skill {
	userSkills := repo.GetAll(user)
	byName := make(map[string]*Skill, len(userSkills)+8)

	if builtinsEnabled(user) {
		for _, s := range loadBuiltins() {
			cp := *s
			cp.Active = true
			byName[cp.Name] = &cp
		}
	}

	for _, s := range userSkills {
		s.Builtin = false
		s.Active = true
		byName[s.Name] = s
	}

	return mapToSortedSlice(byName)
}

// GetAll is the settings UI list: user skills plus all builtins (even when
// disabled), so the user can see what the toggle controls. User skills with
// the same name still replace the builtin entry.
func GetAll(user string) []*Skill {
	userSkills := repo.GetAll(user)
	enabled := builtinsEnabled(user)
	byName := make(map[string]*Skill, len(userSkills)+8)

	for _, s := range loadBuiltins() {
		cp := *s
		cp.Active = enabled
		byName[cp.Name] = &cp
	}

	for _, s := range userSkills {
		s.Builtin = false
		s.Active = true
		byName[s.Name] = s
	}

	return mapToSortedSlice(byName)
}

// GetByName returns a skill usable by the model (respects builtin toggle).
func GetByName(name, user string) (*Skill, error) {
	if s, err := repo.GetByName(name, user); err == nil {
		s.Builtin = false
		s.Active = true
		return s, nil
	}

	if !builtinsEnabled(user) {
		return nil, sql.ErrNoRows
	}
	if s, ok := getBuiltinByName(name); ok {
		s.Active = true
		return s, nil
	}
	return nil, sql.ErrNoRows
}

// GetByID returns a skill for the settings UI (builtins readable even if disabled).
func GetByID(id, user string) (*Skill, error) {
	if isBuiltinID(id) {
		s, ok := getBuiltinByID(id)
		if !ok {
			return nil, sql.ErrNoRows
		}
		// If user overrode this name, surface the user skill instead.
		if u, err := repo.GetByName(s.Name, user); err == nil {
			u.Builtin = false
			u.Active = true
			return u, nil
		}
		s.Active = builtinsEnabled(user)
		return s, nil
	}

	s, err := repo.GetByID(id, user)
	if err != nil {
		return nil, err
	}
	s.Builtin = false
	s.Active = true
	return s, nil
}

func mapToSortedSlice(byName map[string]*Skill) []*Skill {
	out := make([]*Skill, 0, len(byName))
	for _, s := range byName {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		// User skills first, then builtins; alpha within each group.
		if out[i].Builtin != out[j].Builtin {
			return !out[i].Builtin
		}
		return out[i].Name < out[j].Name
	})
	return out
}
