import { getApiUrl } from "@/lib/config";
import { MCPServerRequest, MCPServerResponse } from "./types";
import { getHeaders } from "./headers";

// Get all MCP servers
export const getMCPServers = async (): Promise<MCPServerResponse[]> => {
	const response = await fetch(getApiUrl("/api/tools/mcp/all"), {
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
	const response = await fetch(getApiUrl("/api/tools/mcp/save"), {
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

// Delete MCP server
export const deleteMCPServer = async (id: string): Promise<void> => {
	const response = await fetch(getApiUrl(`/api/tools/mcp/delete/${id}`), {
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
