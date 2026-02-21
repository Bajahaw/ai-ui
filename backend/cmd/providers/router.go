package providers

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/utils"
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
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
	Models []*Model `json:"models"`
}

type ModelsResponse struct {
	Models []*Model `json:"models"`
}

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", getProvidersList)
	mux.HandleFunc("GET /{id}", getProvider)
	mux.HandleFunc("POST /save", saveProvider)
	mux.HandleFunc("DELETE /delete/{id}", deleteProvider)
	mux.HandleFunc("POST /refresh-models/{id}", refreshProviderModels)

	return http.StripPrefix("/api/providers", auth.Authenticated(mux))
}

func ModelsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /all", getAllModels)
	mux.HandleFunc("POST /save-all", saveModels)

	return http.StripPrefix("/api/models", auth.Authenticated(mux))
}

func getAllModels(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	models := providers.GetAllModels(user)
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

	err = providers.SaveModels(models.Models)
	if err != nil {
		log.Error("Error saving models for provider", "err", err)
		http.Error(w, "Error saving models for provider", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func fetchAllModels(provider *Provider) ([]*Model, error) {
	models := make([]*Model, 0)
	client := openai.NewClient(
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	)

	list, err := client.Models.List(context.Background())
	if err != nil {
		log.Error("Error fetching models", "provider", provider.ID, "err", err)
		return nil, err
	}

	for _, model := range list.Data {
		models = append(models, &Model{
			ID:         provider.ID + "/" + model.ID,
			Name:       model.ID,
			ProviderID: provider.ID,
			IsEnabled:  true,
		})
	}

	return models, nil
}

func getProvidersList(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	providers := providers.GetAll(user)

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
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")
	provider, err := providers.GetByID(id, user)
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
		User:    utils.ExtractContextUser(r),
	}

	err = providers.Save(provider)
	if err != nil {
		log.Error("Error saving provider", "err", err)
		http.Error(w, "Error saving provider", http.StatusInternalServerError)
		return
	}

	models, fetchErr := fetchAllModels(provider)
	if fetchErr != nil {
		log.Error("Error fetching models for new provider", "err", fetchErr)
	} else {
		if err = providers.SaveModels(models); err != nil {
			log.Error("Error saving models for provider", "err", err)
		}
	}

	response := Response{
		ID:      provider.ID,
		BaseURL: provider.BaseURL,
	}

	utils.RespondWithJSON(w, &response, http.StatusCreated)
}

func deleteProvider(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")
	err := providers.DeleteByID(id, user)
	if err != nil {
		log.Error("Error deleting provider", "err", err)
		http.Error(w, "Error deleting provider", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func refreshProviderModels(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")

	provider, err := providers.GetByID(id, user)
	if err != nil {
		log.Error("Provider not found", "err", err)
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	// Fetch fresh model list from provider API
	freshModels, fetchErr := fetchAllModels(provider)
	if fetchErr != nil {
		log.Error("Error fetching models from provider", "err", fetchErr)
		http.Error(w, "Failed to fetch models from provider", http.StatusBadGateway)
		return
	}

	// Build map of existing is_enabled states to preserve them
	existingModels := providers.GetModelsByProvider(provider.ID)
	enabledMap := make(map[string]bool, len(existingModels))
	for _, m := range existingModels {
		enabledMap[m.ID] = m.IsEnabled
	}

	// Preserve is_enabled for existing models; new models default to true
	newModelIDs := make([]string, 0, len(freshModels))
	for _, m := range freshModels {
		if enabled, exists := enabledMap[m.ID]; exists {
			m.IsEnabled = enabled
		}
		newModelIDs = append(newModelIDs, m.ID)
	}

	// Upsert with correct is_enabled values
	if err = providers.SaveModels(freshModels); err != nil {
		log.Error("Error saving refreshed models", "err", err)
		http.Error(w, "Error saving models", http.StatusInternalServerError)
		return
	}

	// Remove stale models that no longer exist at the provider
	if err = providers.DeleteModelsNotIn(provider.ID, newModelIDs); err != nil {
		log.Error("Error deleting stale models", "err", err)
		http.Error(w, "Error cleaning up stale models", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
