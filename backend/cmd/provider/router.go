package provider

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/utils"
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
	Name string `json:"name"`
	ID   string `json:"id"`
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
	mux.HandleFunc("GET /{id}/models", getAllModels)
	mux.HandleFunc("DELETE /delete/{id}", deleteProvider)

	return http.StripPrefix("/api/providers", auth.Authenticated(mux))
}

func getAllModels(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	provider, err := repo.getProvider(id)
	whenError(w, "Provider not found", err, http.StatusNotFound)

	client := openai.NewClient(
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	)

	list, err := client.Models.List(r.Context())
	whenError(w, "Error fetching models from provider", err, http.StatusInternalServerError)

	var models []Model
	for _, model := range list.Data {
		models = append(models, Model{
			Name: provider.ID + "/" + utils.ExtractModelName(model.ID),
			ID:   provider.ID + "/" + model.ID,
		})
	}

	response := ModelsResponse{Models: models}
	utils.RespondWithJSON(w, &response, http.StatusOK)
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

	utils.RespondWithJSON(w, &providers, http.StatusOK)
}

func getProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	provider, err := repo.getProvider(id)
	whenError(w, "Provider not found", err, http.StatusNotFound)

	response := Response{
		ID:      provider.ID,
		BaseURL: provider.BaseURL,
	}

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func saveProvider(w http.ResponseWriter, r *http.Request) {
	var req Request
	err := utils.ExtractJSONBody(r, &req)
	whenError(w, "Invalid request body", err, http.StatusBadRequest)

	provider := &Provider{
		ID:      utils.ExtractProviderName(req.BaseURL) + "-" + uuid.New().String()[:4],
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
	}

	err = repo.saveProvider(provider)
	whenError(w, "Error saving provider", err, http.StatusInternalServerError)

	response := Response{
		ID:      provider.ID,
		BaseURL: provider.BaseURL,
	}

	utils.RespondWithJSON(w, &response, http.StatusCreated)
}

func deleteProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := repo.deleteProvider(id)
	whenError(w, "Error deleting provider", err, http.StatusInternalServerError)

	w.WriteHeader(http.StatusNoContent)
}

func whenError(w http.ResponseWriter, msg string, err error, code int) {
	if err != nil {
		log.Error(msg, "err", err)
		http.Error(w, msg+": "+err.Error(), code)
	}
}
