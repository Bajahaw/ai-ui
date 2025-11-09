import { useCallback, useEffect, useState } from "react";
import {
  getMCPServers,
  saveMCPServer,
  deleteMCPServer,
} from "@/lib/api/mcpServers";
import { MCPServerRequest, MCPServerResponse } from "@/lib/api/types";

export interface UseMCPServersReturn {
  mcpServers: MCPServerResponse[];
  isLoading: boolean;
  error: string | null;

  refreshMCPServers: () => Promise<void>;
  addMCPServer: (serverData: MCPServerRequest) => Promise<MCPServerResponse>;
  updateMCPServer: (
    serverData: MCPServerRequest,
  ) => Promise<MCPServerResponse>;
  removeMCPServer: (id: string) => Promise<void>;
  clearError: () => void;
}

export const useMCPServers = (): UseMCPServersReturn => {
  const [mcpServers, setMCPServers] = useState<MCPServerResponse[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const clearError = useCallback(() => setError(null), []);

  const refreshMCPServers = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      const servers = await getMCPServers();
      setMCPServers(servers);
    } catch (err: any) {
      const msg =
        err instanceof Error ? err.message : "Failed to load MCP servers";
      setError(msg);
      console.error("Error loading MCP servers:", err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const addMCPServer = useCallback(
    async (serverData: MCPServerRequest): Promise<MCPServerResponse> => {
      setError(null);
      const newServer = await saveMCPServer(serverData);
      await refreshMCPServers();
      return newServer;
    },
    [refreshMCPServers],
  );

  const updateMCPServer = useCallback(
    async (serverData: MCPServerRequest): Promise<MCPServerResponse> => {
      setError(null);
      const updatedServer = await saveMCPServer(serverData);
      await refreshMCPServers();
      return updatedServer;
    },
    [refreshMCPServers],
  );

  const removeMCPServer = useCallback(
    async (id: string): Promise<void> => {
      setError(null);
      await deleteMCPServer(id);
      await refreshMCPServers();
    },
    [refreshMCPServers],
  );

  // Initial load
  useEffect(() => {
    refreshMCPServers();
  }, [refreshMCPServers]);

  return {
    mcpServers,
    isLoading,
    error,
    refreshMCPServers,
    addMCPServer,
    updateMCPServer,
    removeMCPServer,
    clearError,
  };
};
