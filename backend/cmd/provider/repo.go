package provider

import "errors"

type Provider struct {
	ID      string `json:"id"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

type Repository interface {
	getAllProviders() []*Provider
	getProvider(id string) (*Provider, error)
	addProvider(provider *Provider) error
	updateProvider(provider *Provider) error
	deleteProvider(id string)
}

type InMemoryProviderRepo struct {
	providers map[string]*Provider
}

func NewInMemoryProviderRepo() *InMemoryProviderRepo {
	return &InMemoryProviderRepo{
		providers: make(map[string]*Provider),
	}
}

func (repo *InMemoryProviderRepo) getAllProviders() []*Provider {
	var allProviders []*Provider
	for _, provider := range repo.providers {
		allProviders = append(allProviders, provider)
	}
	return allProviders
}

func (repo *InMemoryProviderRepo) getProvider(id string) (*Provider, error) {
	if provider, exists := repo.providers[id]; exists {
		return provider, nil
	}
	return nil, errors.New("provider not found")
}

func (repo *InMemoryProviderRepo) addProvider(provider *Provider) error {
	if _, exists := repo.providers[provider.ID]; exists {
		return errors.New("provider already exists")
	}
	repo.providers[provider.ID] = provider
	return nil
}

func (repo *InMemoryProviderRepo) updateProvider(provider *Provider) error {
	if _, exists := repo.providers[provider.ID]; !exists {
		return errors.New("provider not found")
	}
	repo.providers[provider.ID] = provider
	return nil
}

func (repo *InMemoryProviderRepo) deleteProvider(id string) {
	delete(repo.providers, id)
}
