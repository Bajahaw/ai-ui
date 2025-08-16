package provider

import (
	"ai-client/cmd/utils"
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

var repo = NewInMemoryProviderRepo()

func Handler() http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", getAllProviders)
	mux.HandleFunc("GET /{id}", getProvider)
	mux.HandleFunc("POST /", addProvider)
	mux.HandleFunc("PATCH /{id}", updateProvider)
	mux.HandleFunc("DELETE /{id}", deleteProvider)

	return mux
}

func deleteProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repo.deleteProvider(id)
	w.WriteHeader(http.StatusNoContent)
}

func getAllProviders(w http.ResponseWriter, _ *http.Request) {
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

func addProvider(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := utils.ExtractJSONBody(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	response := &Provider{
		ID:      "provider-" + time.Now().Format("20060102-150405"),
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
	}

	utils.RespondWithJSON(w, &response, http.StatusCreated)
}

func updateProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req Request
	if err := utils.ExtractJSONBody(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	provider := &Provider{
		ID:      id,
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
	}

	err := repo.updateProvider(provider)
	if err != nil {
		http.Error(w, "Error updating provider: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := Response{
		ID:      provider.ID,
		BaseURL: provider.BaseURL,
	}

	utils.RespondWithJSON(w, &response, http.StatusOK)
}
