package secrets

import (
	"database/sql"
	"errors"

	logger "github.com/charmbracelet/log"
	"github.com/google/uuid"
)

var (
	log  *logger.Logger
	db   *sql.DB
	repo Repository
)

func SetupSecrets(l *logger.Logger, database *sql.DB) {
	log = l
	db = database
	repo = NewRepository(db)
}

// GetValueMap returns name -> value for expansion. Values never leave the server.
func GetValueMap(user string) map[string]string {
	out := make(map[string]string)
	if repo == nil || user == "" {
		return out
	}
	// Load values via a dedicated query path
	rows, err := db.Query(`SELECT name, value FROM UserSecrets WHERE user = ?`, user)
	if err != nil {
		if log != nil {
			log.Error("Error loading secret values", "err", err)
		}
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			continue
		}
		out[name] = value
	}
	return out
}

// ListMeta returns id+name only (no values).
func ListMeta(user string) []SecretResponse {
	if repo == nil {
		return []SecretResponse{}
	}
	list, err := repo.List(user)
	if err != nil {
		if log != nil {
			log.Error("Error listing secrets", "err", err)
		}
		return []SecretResponse{}
	}
	out := make([]SecretResponse, 0, len(list))
	for _, s := range list {
		out = append(out, SecretResponse{ID: s.ID, Name: s.Name})
	}
	return out
}

// Create stores a new secret.
func Create(user, name, value string) (*SecretResponse, error) {
	n, err := NormalizeName(name)
	if err != nil {
		return nil, err
	}
	if err := validateValue(value); err != nil {
		return nil, err
	}
	count, err := repo.Count(user)
	if err != nil {
		return nil, err
	}
	if count >= maxSecretsPerUser {
		return nil, ErrLimit
	}
	s := &Secret{
		ID:    uuid.NewString(),
		Name:  n,
		Value: value,
		User:  user,
	}
	if err := repo.Save(s); err != nil {
		return nil, err
	}
	return &SecretResponse{ID: s.ID, Name: s.Name}, nil
}

// Update changes name and optionally value. Empty value keeps existing.
func Update(id, user, name, value string) (*SecretResponse, error) {
	n, err := NormalizeName(name)
	if err != nil {
		return nil, err
	}
	updateValue := value != ""
	if updateValue {
		if err := validateValue(value); err != nil {
			return nil, err
		}
	}
	if err := repo.Update(id, user, n, value, updateValue); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &SecretResponse{ID: id, Name: n}, nil
}

// Delete removes a secret owned by the user.
func Delete(id, user string) error {
	if err := repo.DeleteByID(id, user); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
