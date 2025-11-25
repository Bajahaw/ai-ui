package tools

import (
	"ai-client/cmd/utils"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func listMCPServers(w http.ResponseWriter, _ *http.Request) {
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
		ID:       uuid.NewString(),
		Name:     req.Name,
		Endpoint: req.Endpoint,
		APIKey:   req.APIKey,
	}

	server.Tools, err = GetMCPTools(server)
	if err != nil {
		log.Error("Error getting MCP tools", "err", err)
		http.Error(w, "Error connecting to MCP server", http.StatusBadRequest)
		return
	}

	// Save MCP server does save tools as well
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

func GetMCPTools(server MCPServer) ([]Tool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)

	headers := map[string]string{
		"Authorization": "Bearer " + server.APIKey,
	}

	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   server.Endpoint,
		HTTPClient: httpClientWithCustomHeaders(headers),
	}, nil)

	if err != nil {
		log.Error("Error connecting to MCP server", "err", err)
		return []Tool{}, err
	}
	defer session.Close()

	var tools []Tool
	if session.InitializeResult().Capabilities.Tools != nil {
		mcpTools := session.Tools(ctx, nil)
		for tool, err := range mcpTools {
			if err != nil {
				log.Error("Error fetching tool from MCP server", "err", err)
				continue
			}
			tools = append(tools, Tool{
				ID:          uuid.New().String(),
				MCPServerID: server.ID,
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: func() string {
					schemaBytes, _ := json.Marshal(tool.InputSchema)
					return string(schemaBytes)
				}(),
			})
		}
	}

	return tools, nil
}

type acceptHeaderRoundTripper struct {
	extraHeaders map[string]string
	delegate     http.RoundTripper
}

func (rt *acceptHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	// req.Header.Set("Authorization", "Bearer fc-***")
	// req.Header.Set("Cache-Control", "no-cache")
	// req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("Accept", "application/json, text/event-stream")

	for k, v := range rt.extraHeaders {
		req.Header.Set(k, v)
	}

	log.Debug("request url", "url", req.URL)
	log.Debug("request headers", "headers", req.Header)
	log.Debug("request method", "method", req.Method)
	// log.Debug("request params", "params", req.URL.Query())

	// Read and restore the request body (required to avoid consuming the body stream)
	// if req.Body != nil {
	// 	bodyBytes, err := io.ReadAll(req.Body)
	// 	if err != nil {
	// 		log.Debug("error reading request body", "error", err)
	// 		return nil, err
	// 	}
	// 	log.Debug("request body", "body", string(bodyBytes))
	// 	// Restore the body so it can be read again
	// 	req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	// }

	resp, err := rt.delegate.RoundTrip(req)
	if err != nil {
		// log.Errorf("[DEBUG] HTTP Request Error: %v", err)
		return nil, err
	}

	log.Debug("response headers", "headers", resp.Header)
	log.Debug("response status", "status", resp.Status)

	// Only log response body for non-streaming responses
	// SSE responses (text/event-stream) should not be read here as they're long-lived streams
	// contentType := resp.Header.Get("Content-Type")
	// if resp.Body != nil {
	// 	// if resp.Body != nil && contentType != "text/event-stream" {
	// 	bodyBytes, err := io.ReadAll(resp.Body)
	// 	if err != nil {
	// 		log.Error("error reading response body", "error", err)
	// 		return nil, err
	// 	}
	// 	log.Debug("response body", "body", string(bodyBytes))
	// 	// Restore the body so it can be read again
	// 	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	// } else if contentType == "text/event-stream" {
	// 	log.Debug("response body", "body", "(SSE stream - not logged)")
	// }

	return resp, err
}

func httpClientWithCustomHeaders(headers map[string]string) *http.Client {
	return &http.Client{
		Transport: &acceptHeaderRoundTripper{
			extraHeaders: headers,
			delegate:     http.DefaultTransport,
		},
	}
}

// MCPSessionManager manager to cache sessions
type MCPSessionManager struct {
	sessions sync.Map
}

func (mgr *MCPSessionManager) add(serverID string, session *mcp.ClientSession) {
	mgr.sessions.Store(serverID, session)

	go func() {
		// time.Sleep(5 * time.Minute) // or better
		<-time.After(5 * time.Minute)
		mgr.sessions.Delete(serverID)
		session.Close()
		log.Debug("MCP session closed due to inactivity", "serverID", serverID)
	}()
}

func (mgr *MCPSessionManager) get(serverID string) (*mcp.ClientSession, bool) {
	value, ok := mgr.sessions.Load(serverID)
	if !ok {
		return nil, false
	}
	return value.(*mcp.ClientSession), true
}
