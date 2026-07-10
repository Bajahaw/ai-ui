package skills

import (
	"database/sql"
)

type Repository interface {
	GetAll(user string) []*Skill
	GetByID(id, user string) (*Skill, error)
	GetByName(name, user string) (*Skill, error)
	Save(skill *Skill) error
	Update(id, user string, skill *Skill) error
	DeleteByID(id, user string) error
}

type RepositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &RepositoryImpl{db: db}
}

func (r *RepositoryImpl) GetAll(user string) []*Skill {
	skills := make([]*Skill, 0)
	rows, err := r.db.Query(`SELECT id, name, description, content FROM Skills WHERE user = ? ORDER BY name`, user)
	if err != nil {
		log.Error("Error querying skills", "err", err)
		return skills
	}
	defer rows.Close()
	for rows.Next() {
		var s Skill
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Content); err != nil {
			log.Error("Error scanning skill", "err", err)
			continue
		}
		s.User = user
		skills = append(skills, &s)
	}
	return skills
}

func (r *RepositoryImpl) GetByID(id, user string) (*Skill, error) {
	var s Skill
	err := r.db.QueryRow(`SELECT id, name, description, content FROM Skills WHERE id = ? AND user = ?`, id, user).
		Scan(&s.ID, &s.Name, &s.Description, &s.Content)
	if err != nil {
		return nil, err
	}
	s.User = user
	return &s, nil
}

func (r *RepositoryImpl) GetByName(name, user string) (*Skill, error) {
	var s Skill
	err := r.db.QueryRow(`SELECT id, name, description, content FROM Skills WHERE name = ? AND user = ?`, name, user).
		Scan(&s.ID, &s.Name, &s.Description, &s.Content)
	if err != nil {
		return nil, err
	}
	s.User = user
	return &s, nil
}

// Save inserts a new skill or, on duplicate (user, name), updates description
// and content. Re-uploading a skill with the same name replaces its body.
func (r *RepositoryImpl) Save(skill *Skill) error {
	_, err := r.db.Exec(`INSERT INTO Skills (id, name, description, content, user) VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(user, name) DO UPDATE SET description=excluded.description, content=excluded.content`,
		skill.ID, skill.Name, skill.Description, skill.Content, skill.User)
	return err
}

// Update modifies an existing skill identified by id+user, allowing the name
// to change. The (user, name) uniqueness constraint still applies.
func (r *RepositoryImpl) Update(id, user string, skill *Skill) error {
	res, err := r.db.Exec(`UPDATE Skills SET name=?, description=?, content=? WHERE id=? AND user=?`,
		skill.Name, skill.Description, skill.Content, id, user)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *RepositoryImpl) DeleteByID(id, user string) error {
	_, err := r.db.Exec(`DELETE FROM Skills WHERE id = ? AND user = ?`, id, user)
	return err
}
