package provider

import (
	"database/sql"
	"strings"
)

type Provider struct {
	ID      string `json:"id"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

type Repository interface {
	getAllProviders() []*Provider
	getProvider(id string) (*Provider, error)
	saveProvider(provider *Provider) error
	deleteProvider(id string) error
	saveModels(models []Model) error
	getAllModels() []Model
}

type Repo struct {
	db *sql.DB
	//cache map[string]*Provider
}

func newProviderRepo(db *sql.DB) *Repo {
	return &Repo{
		db: db,
		//cache: make(map[string]*Provider),
	}
}

func (repo *Repo) getAllProviders() []*Provider {
	var allProviders = make([]*Provider, 0)
	query := `SELECT id, url, api_key FROM Providers`
	rows, err := repo.db.Query(query)
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
		})
	}
	if err = rows.Err(); err != nil {
		log.Error("Error iterating over provider rows", "err", err)
	}

	return allProviders
}

func (repo *Repo) getProvider(id string) (*Provider, error) {
	var p Provider
	query := `SELECT id, url, api_key FROM Providers WHERE id = ?`
	err := repo.db.QueryRow(query, id).Scan(&p.ID, &p.BaseURL, &p.APIKey)
	if err != nil {
		log.Error("Error querying provider", "err", err)
		return nil, err
	}

	return &Provider{
		ID:      p.ID,
		BaseURL: p.BaseURL,
		APIKey:  p.APIKey,
	}, nil
}

func (repo *Repo) saveProvider(provider *Provider) error {
	query := `INSERT INTO Providers (id, url, api_key) VALUES (?, ?, ?)`
	_, err := repo.db.Exec(query, provider.ID, provider.BaseURL, provider.APIKey)
	if err != nil {
		log.Error("Error saving provider", "err", err)
	}

	return err
}

func (repo *Repo) deleteProvider(id string) error {
	query := `DELETE FROM Providers WHERE id = ?`
	_, err := repo.db.Exec(query, id)
	return err
}

func (repo *Repo) saveModels(models []Model) error {
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

func (repo *Repo) getAllModels() []Model {
	var models = make([]Model, 0)
	query := `SELECT id, provider_id, name, is_enabled FROM Models`
	rows, err := repo.db.Query(query)
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
		models = append(models, Model{
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
