package provider

import "errors"

type Provider struct {
	ID      string `json:"id"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

type Repository interface {
	getAllProviders() []*Response
	getProvider(id string) (*Provider, error)
	saveProvider(provider *Provider)
	deleteProvider(id string)
}

type InMemoryProviderRepo struct {
	providers map[string]*Provider
}

func newInMemoryProviderRepo() *InMemoryProviderRepo {
	return &InMemoryProviderRepo{
		providers: make(map[string]*Provider),
	}
}

func (repo *InMemoryProviderRepo) getAllProviders() []*Response {
	var allProviders []*Response
	for _, provider := range repo.providers {
		allProviders = append(allProviders, &Response{
			ID:      provider.ID,
			BaseURL: provider.BaseURL,
		})
	}
	return allProviders
}

func (repo *InMemoryProviderRepo) getProvider(id string) (*Provider, error) {
	if provider, exists := repo.providers[id]; exists {
		return provider, nil
	}
	return nil, errors.New("provider not found")
}

func (repo *InMemoryProviderRepo) saveProvider(provider *Provider) {
	repo.providers[provider.ID] = provider
}

func (repo *InMemoryProviderRepo) deleteProvider(id string) {
	delete(repo.providers, id)
}
