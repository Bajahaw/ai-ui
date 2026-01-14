package auth

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	token    string
}

type UserRepository interface {
	GetByUsername(username string) (*User, error)
	CreateUser(username, token string) error
}

func registerNewUser(username, token string) error {
	_, err := db.Exec(
		`INSERT INTO users (username, token) VALUES (?, ?)`,
		username, token)
	return err
}
