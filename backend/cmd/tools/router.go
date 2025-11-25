package tools

import (
	"ai-client/cmd/auth"
	"ai-client/cmd/utils"
	"net/http"
)

func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /all", listAllTools)
	mux.HandleFunc("POST /saveAll", saveListOfTools)
	// mux.HandleFunc("GET /{id}", GetTool)
	// mux.HandleFunc("POST /save", SaveTool)
	// mux.HandleFunc("DELETE /delete/{id}", DeleteTool)

	mux.HandleFunc("GET /mcp/all", listMCPServers)
	mux.HandleFunc("GET /mcp/{id}", getMCPServer)
	mux.HandleFunc("POST /mcp/save", saveMCPServer)
	mux.HandleFunc("DELETE /mcp/delete/{id}", deleteMCPServer)

	return http.StripPrefix("/api/tools", auth.Authenticated(mux))
}

type ToolListResponse struct {
	Tools []Tool `json:"tools"`
}

func listAllTools(w http.ResponseWriter, _ *http.Request) {
	tools := toolRepo.GetAllTools()
	response := ToolListResponse{
		Tools: tools,
	}
	utils.RespondWithJSON(w, response, http.StatusOK)
}

func saveListOfTools(w http.ResponseWriter, r *http.Request) {
	var req ToolListResponse
	if err := utils.ExtractJSONBody(r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := toolRepo.SaveListOfTools(req.Tools); err != nil {
		http.Error(w, "Error saving tools", http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, nil, http.StatusOK)
}
