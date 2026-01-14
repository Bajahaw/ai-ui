package files

import (
	"ai-client/cmd/utils"
	"database/sql"
)

type Repository interface {
	GetAll(user string) ([]File, error)
	GetByIDs(fileIDs []string, user string) ([]File, error)
	Save(file File) error
	DeleteByID(id string, user string) error
}

type RepositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &RepositoryImpl{db: db}
}

func (r *RepositoryImpl) GetAll(user string) ([]File, error) {
	fileSql := `
	SELECT id, name, type, size, path, url, content, created_at
	FROM Files
	WHERE user = ?
	`

	rows, err := r.db.Query(fileSql, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		if err := rows.Scan(
			&file.ID,
			&file.Name,
			&file.Type,
			&file.Size,
			&file.Path,
			&file.URL,
			&file.Content,
			&file.CreatedAt,
		); err != nil {
			continue // mimic original behavior of skipping errors?
		}
		files = append(files, file)
	}
	return files, nil
}

func (r *RepositoryImpl) GetByIDs(fileIDs []string, user string) ([]File, error) {
	if len(fileIDs) == 0 {
		return []File{}, nil
	}

	fileSql := `
	SELECT id, name, type, size, path, url, content, created_at
	FROM Files
	WHERE id IN (` + utils.SqlPlaceholders(len(fileIDs)) + `) AND user = ?
	`

	args := make([]any, len(fileIDs)+1)
	for i, id := range fileIDs {
		args[i] = id
	}
	args[len(fileIDs)] = user

	rows, err := r.db.Query(fileSql, args...)
	if err != nil {
		return []File{}, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var file File
		if err := rows.Scan(
			&file.ID,
			&file.Name,
			&file.Type,
			&file.Size,
			&file.Path,
			&file.URL,
			&file.Content,
			&file.CreatedAt,
		); err != nil {
			continue
		}
		files = append(files, file)
	}

	return files, nil
}

func (r *RepositoryImpl) Save(file File) error {
	attSql := `INSERT INTO Files (id, name, type, size, path, url, content, user, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(attSql,
		file.ID,
		file.Name,
		file.Type,
		file.Size,
		file.Path,
		file.URL,
		file.Content,
		file.User,
		file.CreatedAt,
	)
	return err
}

func (r *RepositoryImpl) DeleteByID(id string, user string) error {
	deleteSql := `DELETE FROM Files WHERE id = ? AND user = ?`
	_, err := r.db.Exec(deleteSql, id, user)
	return err
}
