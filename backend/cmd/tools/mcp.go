package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Bajahaw/ai-ui/cmd/utils"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPServer struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	APIKey   string            `json:"api_key"`
	User     string            `json:"-"`
	Tools    []*Tool           `json:"tools,omitempty"`
	Headers  map[string]string `json:"headers"`
}

type MCPServerResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	// APIKey string
	Tools   []*Tool           `json:"tools"`
	Headers map[string]string `json:"headers"`
}

type MCPServerListResponse struct {
	Servers []MCPServerResponse `json:"servers"`
}

type MCPServerRequest struct {
	ID       string            `json:"id,omitempty"`
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	APIKey   string            `json:"api_key"`
	Headers  map[string]string `json:"headers"`
}

func listMCPServers(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	servers := mcps.GetAll(user)
	response := make([]MCPServerResponse, len(servers))
	for i, server := range servers {
		response[i] = MCPServerResponse{
			ID:       server.ID,
			Name:     server.Name,
			Endpoint: server.Endpoint,
			Tools:    server.Tools,
			Headers:  server.Headers,
		}
	}
	utils.RespondWithJSON(w, response, http.StatusOK)
}

func getMCPServer(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")
	server, err := mcps.GetByID(id, user)
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
		Headers:  server.Headers,
	}
	utils.RespondWithJSON(w, response, http.StatusOK)
}

func saveMCPServer(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
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
		User:     user,
		Headers:  req.Headers,
	}

	server.Tools, err = GetMCPTools(server)
	if err != nil {
		log.Error("Error getting MCP tools", "err", err)
		http.Error(w, "Error connecting to MCP server", http.StatusBadRequest)
		return
	}

	// Save MCP server does save tools as well
	err = mcps.Save(&server)
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
		Headers:  server.Headers,
	}

	utils.RespondWithJSON(w, &response, http.StatusOK)
}

func deleteMCPServer(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")
	err := mcps.DeleteByID(id, user)
	if err != nil {
		log.Error("Error deleting MCP server", "err", err)
		http.Error(w, "Error deleting MCP server", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, "MCP server deleted successfully", http.StatusOK)
}

func refreshMCPTools(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	id := r.PathValue("id")

	server, err := mcps.GetByID(id, user)
	if err != nil {
		log.Error("MCP server not found", "err", err)
		http.Error(w, "MCP server not found", http.StatusNotFound)
		return
	}

	// Built-in servers (id starts with "default") don't use MCP SDK
	var freshTools []*Tool
	if strings.HasPrefix(server.ID, "default") {
		freshTools = GetBuiltInTools()
		// Update MCPServerID to match the actual server ID
		for _, t := range freshTools {
			t.MCPServerID = server.ID
		}
	} else {
		var fetchErr error
		freshTools, fetchErr = GetMCPTools(*server)
		if fetchErr != nil {
			log.Error("Error fetching tools from MCP server", "err", fetchErr)
			http.Error(w, "Failed to fetch tools from MCP server", http.StatusBadGateway)
			return
		}
	}

	// Build map of existing tool states to preserve them
	existingTools := tools.GetAllByMCPServerID(server.ID)
	stateMap := make(map[string]struct {
		IsEnabled       bool
		RequireApproval bool
	}, len(existingTools))
	for _, t := range existingTools {
		stateMap[t.Name] = struct {
			IsEnabled       bool
			RequireApproval bool
		}{t.IsEnabled, t.RequireApproval}
	}

	// Preserve is_enabled and require_approval for existing tools; new tools default to true/false
	newToolIDs := make([]string, 0, len(freshTools))
	for _, t := range freshTools {
		if state, exists := stateMap[t.Name]; exists {
			t.IsEnabled = state.IsEnabled
			t.RequireApproval = state.RequireApproval
		}
		newToolIDs = append(newToolIDs, t.ID)
	}

	// Upsert with correct state values
	if err = tools.SaveAll(freshTools); err != nil {
		log.Error("Error saving refreshed tools", "err", err)
		http.Error(w, "Error saving tools", http.StatusInternalServerError)
		return
	}

	// Remove stale tools that no longer exist on the MCP server
	if err = tools.DeleteNotIn(server.ID, newToolIDs); err != nil {
		log.Error("Error deleting stale tools", "err", err)
		http.Error(w, "Error cleaning up stale tools", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func GetMCPTools(server MCPServer) ([]*Tool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "2025-11-25"}, nil)

	headers := map[string]string{
		"Authorization": "Bearer " + server.APIKey,
	}
	for k, v := range server.Headers {
		headers[k] = v
	}

	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   server.Endpoint,
		HTTPClient: httpClientWithCustomHeaders(headers),
	}, nil)

	if err != nil {
		log.Error("Error connecting to MCP server", "err", err)
		return []*Tool{}, err
	}
	defer session.Close()

	var tools []*Tool
	if session.InitializeResult().Capabilities.Tools != nil {
		mcpTools := session.Tools(ctx, nil)
		for tool, err := range mcpTools {
			if err != nil {
				log.Error("Error fetching tool from MCP server", "err", err)
				continue
			}
			tools = append(tools, &Tool{
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
