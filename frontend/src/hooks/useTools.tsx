import {Tool} from "@/lib/api/types";

export interface UseToolsReturn {
  tools: Tool[];
  isLoading: boolean;
  error: string | null;

  refreshTools: () => Promise<Tool[]>;
  updateTools: (tools: Tool[]) => Promise<void>;
  clearError: () => void;
}
