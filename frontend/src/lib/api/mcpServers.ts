
import {
  MCPServerRequest,
  MCPServerResponse,
  MCPUpdateCheckResponse,
} from "./types";
import { getHeaders } from "./headers";

// Get all MCP servers
export const getMCPServers = async (): Promise<MCPServerResponse[]> => {
  const response = await fetch("/api/tools/mcp/all", {
    method: "GET",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch MCP servers: ${response.statusText}`);
  }

  return response.json();
};

// Get a specific MCP server - removed
// Save/update MCP server
export const saveMCPServer = async (
  serverData: MCPServerRequest,
): Promise<MCPServerResponse> => {
  const response = await fetch("/api/tools/mcp/save", {
    method: "POST",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(serverData),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to save MCP server: ${response.statusText}`);
  }

  return response.json();
};

// Restore default MCP server
export const restoreDefaultMCPServer = async (): Promise<void> => {
  const response = await fetch("/api/tools/mcp/restore-default", {
    method: "POST",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to restore default MCP server: ${response.statusText}`);
  }
};

// Delete MCP server
export const deleteMCPServer = async (id: string): Promise<void> => {
  const response = await fetch(`/api/tools/mcp/delete/${id}`, {
    method: "DELETE",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to delete MCP server: ${response.statusText}`);
  }
};

// Callback URL to register with the OAuth app (same for every server).
export const getMCPOAuthRedirectURL = async (): Promise<string> => {
  const response = await fetch("/api/tools/mcp/oauth/redirect-url", {
    method: "GET",
    headers: getHeaders(),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to get OAuth redirect URL: ${response.statusText}`);
  }

  const data: { redirect_url: string } = await response.json();
  return data.redirect_url;
};

// Begin the OAuth2 flow for a saved server; returns the URL to open.
export const startMCPOAuth = async (id: string): Promise<string> => {
  const response = await fetch(`/api/tools/mcp/oauth/start/${id}`, {
    method: "POST",
    headers: getHeaders(),
    credentials: "include",
  });

  if (!response.ok) {
    const detail = (await response.text()).trim();
    throw new Error(detail || `Failed to start OAuth: ${response.statusText}`);
  }

  const data: { auth_url: string } = await response.json();
  return data.auth_url;
};

// Checks servers that are due (throttled server-side) and applies changes for
// those with auto-update on; the rest are flagged update_pending.
export const checkMCPUpdates = async (): Promise<MCPUpdateCheckResponse> => {
  const response = await fetch("/api/tools/mcp/check-updates", {
    method: "POST",
    headers: getHeaders(),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to check MCP updates: ${response.statusText}`);
  }

  return response.json();
};

// Refresh tools for a specific MCP server (re-fetches from MCP server)
export const refreshMCPTools = async (id: string): Promise<void> => {
  const response = await fetch(
    `/api/tools/mcp/refresh-tools/${id}`,
    {
      method: "POST",
      headers: getHeaders({
        "Content-Type": "application/json",
      }),
      credentials: "include",
    },
  );

  if (!response.ok) {
    throw new Error(
      `Failed to refresh MCP tools: ${response.statusText}`,
    );
  }
};
