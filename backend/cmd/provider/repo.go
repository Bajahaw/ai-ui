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
