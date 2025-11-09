import { useCallback, useEffect, useState } from "react";
import { getAllTools, saveAllTools } from "@/lib/api/tools";
import { Tool } from "@/lib/api/types";

export interface UseToolsReturn {
  tools: Tool[];
  isLoading: boolean;
  error: string | null;

  refreshTools: () => Promise<Tool[]>;
  updateTools: (tools: Tool[]) => Promise<void>;
  clearError: () => void;
}

export const useTools = (): UseToolsReturn => {
  const [tools, setTools] = useState<Tool[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const clearError = useCallback(() => setError(null), []);

  const refreshTools = useCallback(async (): Promise<Tool[]> => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await getAllTools();
      const normalized = response.tools.map((t) => ({
        ...t,
        is_enabled: t.is_enabled ?? true,
        require_approval: t.require_approval ?? false,
      }));
      setTools(normalized);
      return normalized;
    } catch (err: any) {
      const msg = err instanceof Error ? err.message : "Failed to load tools";
      setError(msg);
      console.error("Error loading tools:", err);
      return [];
    } finally {
      setIsLoading(false);
    }
  }, []);

  const updateTools = useCallback(
    async (updatedTools: Tool[]): Promise<void> => {
      setError(null);
      await saveAllTools(updatedTools);
      await refreshTools();
    },
    [refreshTools],
  );

  // Initial load
  useEffect(() => {
    refreshTools();
  }, [refreshTools]);

  return {
    tools,
    isLoading,
    error,
    refreshTools,
    updateTools,
    clearError,
  };
};
