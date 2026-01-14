package auth

import (
	"database/sql"
	"errors"
	"strings"
)

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	passHash string `json:"-"`
}

type UserRepository interface {
	GetAll() []*User
	GetByUsername(username string) (*User, error)
	Save(user *User) error
}

type UserRepositoryImpl struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &UserRepositoryImpl{db: db}
}

func (r *UserRepositoryImpl) GetAll() []*User {
	query := `SELECT id, username FROM users`
	rows, err := r.db.Query(query)
	if err != nil {
		log.Error("Error retrieving users", "err", err)
		return []*User{}
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		if err := rows.Scan(
			&user.ID,
			&user.Username,
		); err != nil {

			log.Error("Error scanning user row", "err", err)
			return []*User{}
		}

		users = append(users, &user)
	}
	return users
}

func (r *UserRepositoryImpl) GetByUsername(username string) (*User, error) {
	query := `SELECT id, username, pass_hash FROM users WHERE username = ?`
	var user User
	err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.passHash,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepositoryImpl) Save(user *User) error {
	_, err := r.db.Exec(
		`INSERT INTO users (username, pass_hash) VALUES (?, ?)`,
		user.Username, user.passHash,
	)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return errors.New("Username already exists")
	}

	return err
}
