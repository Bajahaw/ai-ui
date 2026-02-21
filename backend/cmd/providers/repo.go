package providers

import (
	"database/sql"
	"strings"
)

type Provider struct {
	ID      string `json:"id"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	User    string `json:"-"`
}

type Repository interface {
	GetAll(user string) []*Provider
	GetByID(id string, user string) (*Provider, error)
	Save(provider *Provider) error
	DeleteByID(id string, user string) error
	SaveModels(models []*Model) error
	GetAllModels(user string) []*Model
	GetModelsByProvider(providerID string) []*Model
	DeleteModelsNotIn(providerID string, modelIDs []string) error
}

type Repo struct {
	db *sql.DB
	//cache map[string]*Provider
}

func newProviderRepo(db *sql.DB) Repository {
	return &Repo{
		db: db,
		//cache: make(map[string]*Provider),
	}
}

func (repo *Repo) GetAll(user string) []*Provider {
	var allProviders = make([]*Provider, 0)
	query := `SELECT id, url, api_key FROM Providers WHERE user = ?`
	rows, err := repo.db.Query(query, user)
	if err != nil {
		log.Error("Error querying providers", "err", err)
		return allProviders
	}
	defer rows.Close()
	for rows.Next() {
		var p Provider
		if err = rows.Scan(&p.ID, &p.BaseURL, &p.APIKey); err != nil {
			log.Error("Error scanning provider", "err", err)
			continue
		}
		allProviders = append(allProviders, &Provider{
			ID:      p.ID,
			BaseURL: p.BaseURL,
			APIKey:  p.APIKey,
			User:    user,
		})
	}
	if err = rows.Err(); err != nil {
		log.Error("Error iterating over provider rows", "err", err)
	}

	return allProviders
}

func (repo *Repo) GetByID(id string, user string) (*Provider, error) {
	var p Provider
	query := `SELECT id, url, api_key FROM Providers WHERE id = ? AND user = ?`
	err := repo.db.QueryRow(query, id, user).Scan(&p.ID, &p.BaseURL, &p.APIKey)
	if err != nil {
		return nil, err
	}

	return &Provider{
		ID:      p.ID,
		BaseURL: p.BaseURL,
		APIKey:  p.APIKey,
		User:    user,
	}, nil
}

func (repo *Repo) Save(provider *Provider) error {
	query := `INSERT INTO Providers (id, url, api_key, user) VALUES (?, ?, ?, ?)`
	_, err := repo.db.Exec(query, provider.ID, provider.BaseURL, provider.APIKey, provider.User)
	return err
}

func (repo *Repo) DeleteByID(id string, user string) error {
	query := `DELETE FROM Providers WHERE id = ? AND user = ?`
	_, err := repo.db.Exec(query, id, user)
	return err
}

func (repo *Repo) SaveModels(models []*Model) error {
	if len(models) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("INSERT INTO Models (id, provider_id, name, is_enabled) VALUES ")

	args := make([]any, 0, len(models)*4)
	for i, m := range models {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(?, ?, ?, ?)")
		args = append(args, m.ID, m.ProviderID, m.Name, m.IsEnabled)
	}

	// on conflict, only update the enabled status
	sb.WriteString(" ON CONFLICT(id) DO UPDATE SET is_enabled=excluded.is_enabled")

	_, err := repo.db.Exec(sb.String(), args...)

	return err
}

func (repo *Repo) GetAllModels(user string) []*Model {
	var models = make([]*Model, 0)
	query := `
		SELECT m.id, m.provider_id, m.name, m.is_enabled 
		FROM Models m
		JOIN Providers p ON m.provider_id = p.id
		WHERE p.user = ?
	`
	rows, err := repo.db.Query(query, user)
	if err != nil {
		log.Error("Error querying models", "err", err)
		return models
	}
	defer rows.Close()
	for rows.Next() {
		var m Model
		if err = rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.IsEnabled); err != nil {
			log.Error("Error scanning model", "err", err)
			continue
		}
		models = append(models, &Model{
			ID:         m.ID,
			Name:       m.Name,
			ProviderID: m.ProviderID,
			IsEnabled:  m.IsEnabled,
		})
	}
	if err = rows.Err(); err != nil {
		log.Error("Error iterating over model rows", "err", err)
	}

	return models
}

func (repo *Repo) GetModelsByProvider(providerID string) []*Model {
	var models = make([]*Model, 0)
	query := `SELECT id, provider_id, name, is_enabled FROM Models WHERE provider_id = ?`
	rows, err := repo.db.Query(query, providerID)
	if err != nil {
		log.Error("Error querying models by provider", "err", err)
		return models
	}
	defer rows.Close()
	for rows.Next() {
		var m Model
		if err = rows.Scan(&m.ID, &m.ProviderID, &m.Name, &m.IsEnabled); err != nil {
			log.Error("Error scanning model", "err", err)
			continue
		}
		models = append(models, &Model{
			ID:         m.ID,
			Name:       m.Name,
			ProviderID: m.ProviderID,
			IsEnabled:  m.IsEnabled,
		})
	}
	if err = rows.Err(); err != nil {
		log.Error("Error iterating over model rows by provider", "err", err)
	}
	return models
}

// DeleteModelsNotIn deletes models for a provider that are NOT in the provided list of model IDs.
func (repo *Repo) DeleteModelsNotIn(providerID string, modelIDs []string) error {
	if len(modelIDs) == 0 {
		_, err := repo.db.Exec("DELETE FROM Models WHERE provider_id = ?", providerID)
		return err
	}

	var sb strings.Builder
	sb.WriteString("DELETE FROM Models WHERE provider_id = ? AND id NOT IN (")
	args := make([]any, 0, len(modelIDs)+1)
	args = append(args, providerID)
	for i, id := range modelIDs {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("?")
		args = append(args, id)
	}
	sb.WriteString(")")

	_, err := repo.db.Exec(sb.String(), args...)
	return err
}
