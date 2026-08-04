package secrets

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Repository interface {
	List(user string) ([]*Secret, error)
	GetByID(id, user string) (*Secret, error)
	GetByName(name, user string) (*Secret, error)
	Count(user string) (int, error)
	Save(secret *Secret) error
	Update(id, user string, name, value string, updateValue bool) error
	DeleteByID(id, user string) error
}

type RepositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &RepositoryImpl{db: db}
}

func (r *RepositoryImpl) List(user string) ([]*Secret, error) {
	rows, err := r.db.Query(
		`SELECT id, name FROM UserSecrets WHERE user = ? ORDER BY name`,
		user,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*Secret, 0)
	for rows.Next() {
		var s Secret
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return nil, err
		}
		s.User = user
		out = append(out, &s)
	}
	return out, rows.Err()
}

func (r *RepositoryImpl) GetByID(id, user string) (*Secret, error) {
	var s Secret
	err := r.db.QueryRow(
		`SELECT id, name, value FROM UserSecrets WHERE id = ? AND user = ?`,
		id, user,
	).Scan(&s.ID, &s.Name, &s.Value)
	if err != nil {
		return nil, err
	}
	s.User = user
	return &s, nil
}

func (r *RepositoryImpl) GetByName(name, user string) (*Secret, error) {
	var s Secret
	err := r.db.QueryRow(
		`SELECT id, name, value FROM UserSecrets WHERE name = ? AND user = ?`,
		name, user,
	).Scan(&s.ID, &s.Name, &s.Value)
	if err != nil {
		return nil, err
	}
	s.User = user
	return &s, nil
}

func (r *RepositoryImpl) Count(user string) (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM UserSecrets WHERE user = ?`, user).Scan(&n)
	return n, err
}

func (r *RepositoryImpl) Save(secret *Secret) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(
		`INSERT INTO UserSecrets (id, name, value, user, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		secret.ID, secret.Name, secret.Value, secret.User, now, now,
	)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		return errors.New("a secret with this name already exists")
	}
	return err
}

func (r *RepositoryImpl) Update(id, user string, name, value string, updateValue bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var res sql.Result
	var err error
	if updateValue {
		res, err = r.db.Exec(
			`UPDATE UserSecrets SET name=?, value=?, updated_at=? WHERE id=? AND user=?`,
			name, value, now, id, user,
		)
	} else {
		res, err = r.db.Exec(
			`UPDATE UserSecrets SET name=?, updated_at=? WHERE id=? AND user=?`,
			name, now, id, user,
		)
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return errors.New("a secret with this name already exists")
		}
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *RepositoryImpl) DeleteByID(id, user string) error {
	res, err := r.db.Exec(`DELETE FROM UserSecrets WHERE id = ? AND user = ?`, id, user)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
