import {MCPServerRequest, MCPServerResponse} from "@/lib/api/types";

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
