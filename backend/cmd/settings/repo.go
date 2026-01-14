package settings

import "database/sql"

type Repository interface {
	GetAll(user string) (map[string]string, error)
	Save(settings map[string]string, user string) error
	SaveDefaults(defaults map[string]string, user string) error
	Get(key string, user string) (string, error)
}

type RepositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &RepositoryImpl{db: db}
}

func (r *RepositoryImpl) GetAll(user string) (map[string]string, error) {
	sql := "SELECT key, value FROM Settings WHERE user = ?"
	rows, err := r.db.Query(sql, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value
	}
	return settings, nil
}

func (r *RepositoryImpl) Save(settings map[string]string, user string) error {
	for key, value := range settings {
		if key == "" {
			continue // Should maybe log this?
		}

		// on conflict, update the value
		sql := "INSERT INTO Settings (key, value, user) VALUES (?, ?, ?) ON CONFLICT(key, user) DO UPDATE SET value=excluded.value"
		_, err := r.db.Exec(sql, key, value, user)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *RepositoryImpl) SaveDefaults(defaults map[string]string, user string) error {
	for key, value := range defaults {
		if key == "" {
			continue
		}

		// on conflict, do not update the value
		sql := "INSERT INTO Settings (key, value, user) VALUES (?, ?, ?) ON CONFLICT(key, user) DO NOTHING"
		_, err := r.db.Exec(sql, key, value, user)
		if err != nil {
			// Log error but continue for other defaults? The original implementation did that.
			// Since repository typically returns error, we might need to handle this.
			// However, for bulk insert of defaults, we probably want to try all.
			// Let's modify the interface to allow this behavior or just return the first error.
			// The original implementation logged and continued.
			// I'll return the first error for now, but in practice the caller (Setup/Hook) should handle it.
			// Actually, let's keep the "continue on error" behavior but we can't easily log here without the logger.
			// So I'll just return the first error which is a change in behavior, specifically stricter.
			return err
		}
	}
	return nil
}

func (r *RepositoryImpl) Get(key string, user string) (string, error) {
	sql := "SELECT value FROM Settings WHERE key = ? AND user = ?"
	row := r.db.QueryRow(sql, key, user)

	var value string
	err := row.Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}
