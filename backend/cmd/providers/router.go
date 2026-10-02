package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Bajahaw/ai-ui/cmd/auth"
	"github.com/Bajahaw/ai-ui/cmd/chatgptoauth"
	"github.com/Bajahaw/ai-ui/cmd/utils"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Request creates a provider, or updates one when ID is set.
// On update, a blank APIKey or header value keeps the stored one.
type Request struct {
	ID      string            `json:"id,omitempty"`
	BaseURL string            `json:"base_url"`
	APIKey  string            `json:"api_key"`
	Headers map[string]string `json:"headers"`
}

// Response never includes credentials: header values are blanked.
type Response struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	BaseURL string            `json:"base_url"`
	Label   string            `json:"label,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

var errFetchModels = errors.New("failed to fetch models from provider")

func toResponse(p *Provider) Response {
	if p.Type == chatgptoauth.ProviderType {
		return Response{ID: p.ID, Type: p.Type, BaseURL: p.BaseURL, Label: p.Headers["label"]}
	}
	return Response{ID: p.ID, Type: p.Type, BaseURL: p.BaseURL, Headers: utils.RedactHeaders(p.Headers)}
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
	user := utils.ExtractContextUser(r)
	var models ModelRequest
	err := utils.ExtractJSONBody(r, &models)
	if err != nil {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = providers.SaveModels(models.Models, user)
	if err != nil {
		log.Error("Error saving models for provider", "err", err)
		if errors.Is(err, ErrUnauthorizedProviderReference) {
			http.Error(w, "Unauthorized provider reference", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Error saving models for provider", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func fetchAllModels(provider *Provider) ([]*Model, error) {
	if provider.Type == chatgptoauth.ProviderType {
		tokens, err := resolveChatGPTTokens(provider)
		if err != nil {
			return nil, err
		}
		client := chatgptoauth.NewClient()
		list, err := client.ListModels(context.Background(), tokens)
		if err != nil {
			log.Error("Error fetching ChatGPT models", "provider", provider.ID, "err", err)
			return nil, err
		}
		models := make([]*Model, 0, len(list))
		for _, model := range list {
			models = append(models, &Model{
				ID:         provider.ID + "/" + model.Slug,
				Name:       model.Slug,
				ProviderID: provider.ID,
				IsEnabled:  true,
			})
		}
		return models, nil
	}

	models := make([]*Model, 0)
	opts := []option.RequestOption{
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
		option.WithQuery("output_modalities", "all"),
	}
	for key, value := range provider.Headers {
		opts = append(opts, option.WithHeader(key, value))
	}
	client := openai.NewClient(opts...)

	list, err := client.Models.List(context.Background(), opts...)
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
		response = append(response, toResponse(p))
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

	response := toResponse(provider)
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
		ID:      req.ID,
		Type:    ProviderTypeOpenAI,
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
		User:    utils.ExtractContextUser(r),
		Headers: req.Headers,
	}

	status := http.StatusCreated
	if req.ID == "" {
		provider.ID = utils.ExtractProviderName(req.BaseURL) + "-" + uuid.New().String()[:4]
		err = providers.Save(provider)
	} else {
		existing, getErr := providers.GetByID(req.ID, provider.User)
		if getErr != nil || existing.Type != ProviderTypeOpenAI {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		if provider.APIKey == "" {
			provider.APIKey = existing.APIKey
		}
		provider.Headers = utils.MergeHeaders(req.Headers, existing.Headers)
		err = providers.Upsert(provider)
		status = http.StatusOK
	}
	if err != nil {
		log.Error("Error saving provider", "err", err)
		http.Error(w, "Error saving provider", http.StatusInternalServerError)
		return
	}

	if err = syncModels(provider); err != nil {
		log.Error("Error syncing models for provider", "provider", provider.ID, "err", err)
	}

	response := toResponse(provider)
	utils.RespondWithJSON(w, &response, status)
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

	if err = syncModels(provider); err != nil {
		log.Error("Error refreshing models", "provider", provider.ID, "err", err)
		switch {
		case errors.Is(err, errFetchModels):
			http.Error(w, "Failed to fetch models from provider", http.StatusBadGateway)
		case errors.Is(err, ErrUnauthorizedProviderReference):
			http.Error(w, "Unauthorized provider reference", http.StatusUnauthorized)
		default:
			http.Error(w, "Error saving models", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// syncModels fetches the provider's model list, preserving is_enabled for
// known models (new ones default to enabled) and removing stale ones.
func syncModels(provider *Provider) error {
	// Fetch fresh model list from provider API
	freshModels, err := fetchAllModels(provider)
	if err != nil {
		return fmt.Errorf("%w: %v", errFetchModels, err)
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
	if err = providers.SaveModels(freshModels, provider.User); err != nil {
		return err
	}
	// Remove stale models that no longer exist at the provider
	return providers.DeleteModelsNotIn(provider.ID, newModelIDs)
}
