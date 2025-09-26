package provider

import (
	"ai-client/cmd/data"
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
	saveModels(id string, models []Model) error
	getProviderModels(provider *Provider) ([]Model, error)
	getAllModels() []Model
}

type Repo struct {
	//cache map[string]*Provider
}

func newProviderRepo() *Repo {
	return &Repo{
		//cache: make(map[string]*Provider),
	}
}

func (repo *Repo) getAllProviders() []*Provider {
	var allProviders = make([]*Provider, 0)
	sql := `SELECT id, url, api_key FROM Providers`
	rows, err := data.DB.Query(sql)
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
	sql := `SELECT id, url, api_key FROM Providers WHERE id = ?`
	err := data.DB.QueryRow(sql, id).Scan(&p.ID, &p.BaseURL, &p.APIKey)
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
	sql := `INSERT INTO Providers (id, url, api_key) VALUES (?, ?, ?)`
	_, err := data.DB.Exec(sql, provider.ID, provider.BaseURL, provider.APIKey)
	if err != nil {
		log.Error("Error saving provider", "err", err)
	}

	return err
}

func (repo *Repo) deleteProvider(id string) error {
	sql := `DELETE FROM Providers WHERE id = ?`
	_, err := data.DB.Exec(sql, id)
	if err != nil {
		log.Error("Error deleting provider", "err", err)
	}

	return err
}

func (repo *Repo) saveModels(models []Model) error {
	// on conflict, replace
	sql := `
	INSERT INTO Models (id, provider_id, name, is_enabled) VALUES (?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET provider_id=excluded.provider_id, name=excluded.name, is_enabled=excluded.is_enabled`
	for _, model := range models {
		_, err := data.DB.Exec(sql, model.ID, model.ProviderID, model.Name, model.IsEnabled)
		if err != nil {
			log.Error("Error saving model", "err", err)
			return err
		}
	}

	return nil
}

func (repo *Repo) getProviderModels(provider *Provider) ([]Model, error) {
	var models = make([]Model, 0)
	sql := `SELECT id, provider_id, name, is_enabled FROM Models WHERE provider_id = ?`
	rows, err := data.DB.Query(sql, provider.ID)
	if err != nil {
		log.Error("Error querying models", "err", err)
		return models, err
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
		// return models, err
	}
	return models, nil
}

func (repo *Repo) getAllModels() []Model {
	var models = make([]Model, 0)
	sql := `SELECT id, provider_id, name, is_enabled FROM Models`
	rows, err := data.DB.Query(sql)
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
