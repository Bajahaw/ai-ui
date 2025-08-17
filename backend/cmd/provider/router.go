package provider

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/utils"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"net/http"
	"time"
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

var repo = newInMemoryProviderRepo()

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
	if err != nil {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	client := openai.NewClient(
		option.WithAPIKey(provider.APIKey),
		option.WithBaseURL(provider.BaseURL),
	)

	list, err := client.Models.List(r.Context())
	if err != nil {
		http.Error(w, "Error fetching models: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var models []Model
	for _, model := range list.Data {
		models = append(models, Model{
			Name: model.ID,
			ID:   provider.ID + "/" + model.ID,
		})
	}

	response := ModelsResponse{Models: models}
	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func getProvidersList(w http.ResponseWriter, _ *http.Request) {
	providers := repo.getAllProviders()
	utils.RespondWithJSON(w, &providers, http.StatusOK)
}

func getProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	provider, err := repo.getProvider(id)
	if err != nil {
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
	if err := utils.ExtractJSONBody(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	provider := &Provider{
		ID:      "provider-" + time.Now().Format("20060102-150405"),
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
	}

	repo.saveProvider(provider)

	response := Response{
		ID:      provider.ID,
		BaseURL: provider.BaseURL,
	}

	utils.RespondWithJSON(w, &response, http.StatusCreated)
}

func deleteProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repo.deleteProvider(id)
	w.WriteHeader(http.StatusNoContent)
}
