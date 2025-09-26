package provider

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/utils"
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

type Request struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

type Response struct {
	ID      string `json:"id"`
	BaseURL string `json:"base_url"`
}

type Model struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProviderID string `json:"provider"`
	IsEnabled  bool   `json:"is_enabled"`
}

type ModelRequest struct {
	Models []Model `json:"models"`
}

type ModelsResponse struct {
	Models []Model `json:"models"`
}

var repo = newProviderRepo()

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", getProvidersList)
	mux.HandleFunc("GET /{id}", getProvider)
	mux.HandleFunc("POST /save", saveProvider)
	mux.HandleFunc("GET /{id}/models", getProviderModels)
	mux.HandleFunc("DELETE /delete/{id}", deleteProvider)

	return http.StripPrefix("/api/providers", auth.Authenticated(mux))
}

func ModelsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /all", getAllModels)
	mux.HandleFunc("POST /save-all", saveModels)

	return http.StripPrefix("/api/models", auth.Authenticated(mux))
}

func getAllModels(w http.ResponseWriter, r *http.Request) {
	models := repo.getAllModels()
	response := ModelsResponse{
		Models: models,
	}
	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func saveModels(w http.ResponseWriter, r *http.Request) {
	var models ModelRequest
	err := utils.ExtractJSONBody(r, &models)
	if err != nil {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = repo.saveModels(models.Models)
	if err != nil {
		log.Error("Error saving models for provider", "err", err)
		http.Error(w, "Error saving models for provider", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func getProviderModels(w http.ResponseWriter, r *http.Request) {
	var providerID = r.PathValue("id")
	provider, err := repo.getProvider(providerID)
	if err != nil {
		log.Error("Provider not found", "err", err)
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	models, err := repo.getProviderModels(provider)
	if err != nil {
		log.Error("Error fetching models for provider", "err", err)
		http.Error(w, "Error fetching models for provider", http.StatusInternalServerError)
		return
	}

	response := ModelsResponse{
		Models: models,
	}

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func fetchAllModels(provider *Provider) ([]Model, error) {
	client := openai.NewClient(
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	)

	list, err := client.Models.List(context.Background())
	if err != nil {
		log.Error("Error fetching models", "err", err)
		return nil, err
	}

	var models []Model
	for _, model := range list.Data {
		models = append(models, Model{
			ID:         provider.ID + "/" + model.ID,
			Name:       model.ID,
			ProviderID: provider.ID,
			IsEnabled:  true,
		})
	}

	return models, nil
}

func getProvidersList(w http.ResponseWriter, _ *http.Request) {
	providers := repo.getAllProviders()

	response := make([]Response, 0, len(providers))
	for _, p := range providers {
		response = append(response, Response{
			ID:      p.ID,
			BaseURL: p.BaseURL,
		})
	}

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func getProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	provider, err := repo.getProvider(id)
	if err != nil {
		log.Error("Provider not found", "err", err)
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	response := Response{
		ID:      provider.ID,
		BaseURL: provider.BaseURL,
	}

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func saveProvider(w http.ResponseWriter, r *http.Request) {
	var req Request
	err := utils.ExtractJSONBody(r, &req)
	if err != nil || req.BaseURL == "" {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	provider := &Provider{
		ID:      utils.ExtractProviderName(req.BaseURL) + "-" + uuid.New().String()[:4],
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
	}

	err = repo.saveProvider(provider)
	if err != nil {
		log.Error("Error saving provider", "err", err)
		http.Error(w, "Error saving provider", http.StatusInternalServerError)
		return
	}

	models, err := fetchAllModels(provider)
	if err != nil {
		log.Error("Error fetching models for provider", "err", err)
	}

	err = repo.saveModels(models)
	if err != nil {
		log.Error("Error saving models for provider", "err", err)
	}

	response := Response{
		ID:      provider.ID,
		BaseURL: provider.BaseURL,
	}

	utils.RespondWithJSON(w, &response, http.StatusCreated)
}

func deleteProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := repo.deleteProvider(id)
	if err != nil {
		log.Error("Error deleting provider", "err", err)
		http.Error(w, "Error deleting provider", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
