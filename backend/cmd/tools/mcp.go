package tools

import (
	"ai-client/cmd/utils"
	"net/http"
)

type MCPServer struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"api_key"`
	Tools    []Tool `json:"tools,omitempty"`
}

type MCPServerResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	// APIKey string
	Tools []Tool `json:"tools"`
}

type MCPServerListResponse struct {
	Servers []MCPServerResponse `json:"servers"`
}

type MCPServerRequest struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"api_key"`
}

func listMCPServers(w http.ResponseWriter, r *http.Request) {
	servers := mcpRepo.GetAllMCPServers()
	response := make([]MCPServerResponse, len(servers))
	for i, server := range servers {
		response[i] = MCPServerResponse{
			ID:       server.ID,
			Name:     server.Name,
			Endpoint: server.Endpoint,
			Tools:    server.Tools,
		}
	}
	utils.RespondWithJSON(w, response, http.StatusOK)
}

func getMCPServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	server, err := mcpRepo.GetMCPServer(id)
	if err != nil {
		log.Error("Error getting MCP server", "err", err)
		http.Error(w, "MCP server not found", http.StatusNotFound)
		return
	}

	response := MCPServerResponse{
		ID:       server.ID,
		Name:     server.Name,
		Endpoint: server.Endpoint,
		Tools:    server.Tools,
	}
	utils.RespondWithJSON(w, response, http.StatusOK)
}

func saveMCPServer(w http.ResponseWriter, r *http.Request) {
	var req MCPServerRequest
	err := utils.ExtractJSONBody(r, &req)
	if err != nil {
		log.Error("Error unmarshalling request body", "err", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	server := MCPServer{
		ID:       req.ID,
		Name:     req.Name,
		Endpoint: req.Endpoint,
		APIKey:   req.APIKey,
	}

	// Validate connection to MCP server, fetch tools, etc. (omitted for brevity)

	err = mcpRepo.SaveMCPServer(server)
	if err != nil {
		log.Error("Error saving MCP server", "err", err)
		http.Error(w, "Error saving MCP server", http.StatusInternalServerError)
		return
	}

	response := MCPServerResponse{
		ID:       server.ID,
		Name:     server.Name,
		Endpoint: server.Endpoint,
		Tools:    server.Tools,
	}

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func deleteMCPServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := mcpRepo.DeleteMCPServer(id)
	if err != nil {
		log.Error("Error deleting MCP server", "err", err)
		http.Error(w, "Error deleting MCP server", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, "MCP server deleted successfully", http.StatusOK)
}
