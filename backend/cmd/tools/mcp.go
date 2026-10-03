package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
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
	AuthType string            `json:"auth_type"`
	OAuth    *MCPOAuth         `json:"-"`
	Info     MCPServerInfo     `json:"server_info"`
	// AutoUpdate applies changes found by the app-load update check; when
	// off, the check only sets UpdatePending.
	AutoUpdate    bool  `json:"auto_update"`
	UpdatePending bool  `json:"update_pending"`
	LastCheckedAt int64 `json:"-"` // unix seconds
}

// MCPServerInfo is what the server reports about itself on initialize.
// It is server-controlled text, so it is length-capped on capture.
type MCPServerInfo struct {
	Name         string `json:"name"`
	Title        string `json:"title,omitempty"`
	Version      string `json:"version,omitempty"`
	Description  string `json:"description,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

const (
	mcpMaxInfoFieldRunes    = 1000
	mcpMaxInstructionsRunes = 16000
)

func serverInfoFromInit(res *mcp.InitializeResult) MCPServerInfo {
	if res == nil {
		return MCPServerInfo{}
	}
	info := MCPServerInfo{Instructions: truncateRunes(strings.TrimSpace(res.Instructions), mcpMaxInstructionsRunes)}
	if si := res.ServerInfo; si != nil {
		info.Name = truncateRunes(strings.TrimSpace(si.Name), mcpMaxInfoFieldRunes)
		info.Title = truncateRunes(strings.TrimSpace(si.Title), mcpMaxInfoFieldRunes)
		info.Version = truncateRunes(strings.TrimSpace(si.Version), mcpMaxInfoFieldRunes)
		info.Description = truncateRunes(strings.TrimSpace(si.Description), mcpMaxInfoFieldRunes)
	}
	return info
}

// MCPServerResponse never includes credentials: header values are blanked.
type MCPServerResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	// APIKey string
	Tools         []*Tool           `json:"tools"`
	Headers       map[string]string `json:"headers"`
	AuthType      string            `json:"auth_type"`
	OAuth         *MCPOAuthResponse `json:"oauth,omitempty"`
	ServerInfo    *MCPServerInfo    `json:"server_info,omitempty"`
	AutoUpdate    bool              `json:"auto_update"`
	UpdatePending bool              `json:"update_pending"`
}

func toMCPResponse(server *MCPServer) MCPServerResponse {
	resp := MCPServerResponse{
		ID:            server.ID,
		Name:          server.Name,
		Endpoint:      server.Endpoint,
		Tools:         server.Tools,
		Headers:       utils.RedactHeaders(server.Headers),
		AuthType:      server.AuthType,
		AutoUpdate:    server.AutoUpdate,
		UpdatePending: server.UpdatePending,
	}
	if server.AuthType == AuthTypeOAuth2 {
		resp.OAuth = toMCPOAuthResponse(server.OAuth)
	}
	if server.Info != (MCPServerInfo{}) {
		info := server.Info
		resp.ServerInfo = &info
	}
	return resp
}

type MCPServerListResponse struct {
	Servers []MCPServerResponse `json:"servers"`
}

// MCPServerRequest creates a server, or updates one when ID is set.
// On update, a blank APIKey or header value keeps the stored one.
type MCPServerRequest struct {
	ID       string            `json:"id,omitempty"`
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	APIKey   string            `json:"api_key"`
	Headers  map[string]string `json:"headers"`
	AuthType string            `json:"auth_type,omitempty"`
	OAuth    *MCPOAuthRequest  `json:"oauth,omitempty"`
	// Omitted keeps the stored setting (off for new servers).
	AutoUpdate *bool `json:"auto_update,omitempty"`
}

func listMCPServers(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	servers := mcps.GetAll(user)
	response := make([]MCPServerResponse, len(servers))
	for i, server := range servers {
		response[i] = toMCPResponse(server)
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

	utils.RespondWithJSON(w, toMCPResponse(server), http.StatusOK)
}

func restoreDefaultMCPServer(w http.ResponseWriter, r *http.Request) {
	user := utils.ExtractContextUser(r)
	SaveDefaultMCPServer(user)
	utils.RespondWithJSON(w, map[string]string{"status": "success"}, http.StatusOK)
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

	if req.AuthType != "" && req.AuthType != AuthTypeOAuth2 {
		http.Error(w, "Unsupported auth type", http.StatusBadRequest)
		return
	}

	server := MCPServer{
		ID:       req.ID,
		Name:     req.Name,
		Endpoint: req.Endpoint,
		APIKey:   req.APIKey,
		User:     user,
		Headers:  req.Headers,
		AuthType: req.AuthType,
	}

	isUpdate := req.ID != ""
	var existing *MCPServer
	if isUpdate {
		var getErr error
		existing, getErr = mcps.GetByID(req.ID, user)
		if getErr != nil {
			http.Error(w, "MCP server not found", http.StatusNotFound)
			return
		}
		if server.APIKey == "" {
			server.APIKey = existing.APIKey
		}
		server.Headers = utils.MergeHeaders(req.Headers, existing.Headers)
		server.AutoUpdate = existing.AutoUpdate
		server.UpdatePending = existing.UpdatePending
		server.LastCheckedAt = existing.LastCheckedAt
	} else {
		server.ID = uuid.NewString()
	}
	if req.AutoUpdate != nil {
		server.AutoUpdate = *req.AutoUpdate
	}

	if server.AuthType == AuthTypeOAuth2 {
		// The OAuth access token replaces the static Bearer key.
		server.APIKey = ""
		server.OAuth = mergeMCPOAuth(req.OAuth, server.Endpoint, existing)
	}

	// An OAuth server can't be reached until the user authorizes it, so it is
	// saved first and its tools are fetched by the OAuth callback.
	needsAuthorization := server.AuthType == AuthTypeOAuth2 && !server.OAuth.connected()
	if needsAuthorization {
		if existing != nil {
			server.Tools = existing.Tools
			server.Info = existing.Info
		}
	} else {
		server.Tools, server.Info, err = GetMCPTools(server)
		if err != nil {
			log.Error("Error getting MCP tools", "err", err)
			http.Error(w, "Error connecting to MCP server", http.StatusBadRequest)
			return
		}
		server.UpdatePending = false
		server.LastCheckedAt = time.Now().Unix()
	}

	if isUpdate {
		if err = mcps.Update(&server); err == nil && !needsAuthorization {
			err = storeCatalog(server.ID, user, server.Tools, server.Info)
		}
	} else {
		// Save MCP server does save tools as well
		err = mcps.Save(&server)
	}
	if err != nil {
		log.Error("Error saving MCP server", "err", err)
		http.Error(w, "Error saving MCP server", http.StatusInternalServerError)
		return
	}

	response := toMCPResponse(&server)
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
	mcpSessionManager.invalidate(id)

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
	if isBuiltInServer(server.ID) {
		freshTools, server.Info = builtInToolsFor(server.ID), builtInServerInfo()
	} else {
		var fetchErr error
		freshTools, server.Info, fetchErr = GetMCPTools(*server)
		if fetchErr != nil {
			log.Error("Error fetching tools from MCP server", "err", fetchErr)
			http.Error(w, "Failed to fetch tools from MCP server", http.StatusBadGateway)
			return
		}
	}

	if err = storeCatalog(server.ID, user, freshTools, server.Info); err != nil {
		log.Error("Error saving refreshed tools", "err", err)
		http.Error(w, "Error saving tools", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// storeCatalog saves a freshly fetched catalog and marks the server as
// checked with no pending update.
func storeCatalog(serverID, user string, freshTools []*Tool, info MCPServerInfo) error {
	if err := syncTools(serverID, freshTools); err != nil {
		return err
	}
	if err := mcps.UpdateServerInfo(serverID, user, info); err != nil {
		return err
	}
	return mcps.UpdateSyncState(serverID, user, false, time.Now().Unix())
}

// syncTools stores freshTools for an existing server: tools matched by name keep
// their ID, is_enabled and require_approval; tools no longer listed are removed.
func syncTools(serverID string, freshTools []*Tool) error {
	// Build map of existing tools keyed by name to preserve IDs and user-set flags
	existingTools := tools.GetAllByMCPServerID(serverID)
	existingMap := make(map[string]*Tool, len(existingTools))
	for _, t := range existingTools {
		existingMap[t.Name] = t
	}

	// Preserve IDs, is_enabled, and require_approval for existing tools; new tools keep generated IDs and defaults
	newToolIDs := make([]string, 0, len(freshTools))
	for _, t := range freshTools {
		if existing, exists := existingMap[t.Name]; exists {
			t.ID = existing.ID
			t.IsEnabled = existing.IsEnabled
			t.RequireApproval = existing.RequireApproval
		}
		newToolIDs = append(newToolIDs, t.ID)
	}

	// Upsert all fields (including schema/description changes) with correct state values
	if err := tools.UpsertAll(freshTools); err != nil {
		return err
	}
	// Remove stale tools that no longer exist on the MCP server
	return tools.DeleteNotIn(serverID, newToolIDs)
}

// GetMCPTools lists the server's tools along with what it reported about
// itself on initialize.
func GetMCPTools(server MCPServer) ([]*Tool, MCPServerInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mcpConnectTimeout)
	defer cancel()
	return fetchMCPCatalog(ctx, server)
}

// fetchMCPCatalog retries once on a fresh session when the cached one died.
func fetchMCPCatalog(ctx context.Context, server MCPServer) ([]*Tool, MCPServerInfo, error) {
	listed, info, err := listMCPCatalog(ctx, server)
	if err != nil && ctx.Err() == nil && sessionDead(err) {
		mcpSessionManager.invalidate(server.ID)
		listed, info, err = listMCPCatalog(ctx, server)
	}
	return listed, info, err
}

func listMCPCatalog(ctx context.Context, server MCPServer) ([]*Tool, MCPServerInfo, error) {
	session, err := mcpSessionManager.session(ctx, server)
	if err != nil {
		log.Error("Error connecting to MCP server", "err", err)
		return []*Tool{}, MCPServerInfo{}, err
	}

	res := session.InitializeResult()
	info := serverInfoFromInit(res)
	if res != nil && res.Capabilities.Tools == nil {
		return []*Tool{}, info, nil
	}

	var listed []*Tool
	for tool, err := range session.Tools(ctx, nil) {
		// Iteration stops at the first error, so a partial list must not be
		// treated as complete: syncTools would delete the missing tools.
		if err != nil {
			log.Error("Error fetching tools from MCP server", "err", err)
			return []*Tool{}, MCPServerInfo{}, err
		}
		// New tools start enabled; syncTools restores the stored flag for
		// tools that already exist, so a refresh never re-enables one.
		listed = append(listed, &Tool{
			ID:          uuid.New().String(),
			IsEnabled:   true,
			MCPServerID: server.ID,
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: func() string {
				schemaBytes, _ := json.Marshal(tool.InputSchema)
				return string(schemaBytes)
			}(),
		})
	}

	return listed, info, nil
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
