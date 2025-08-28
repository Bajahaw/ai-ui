import { useState, useEffect, useCallback } from "react";
import { Model } from "@/lib/api/types";
import { useProviders } from "./useProviders";

// Simple in-memory storage for models
let cachedModels: Model[] | null = null;
let hasLoadedModels = false;

interface UseModelsReturn {
  models: Model[];
  isLoading: boolean;
  error: string | null;
  refreshModels: () => Promise<void>;
  getModelDisplayName: (modelId: string) => string;
  clearError: () => void;
  clearCache: () => void;
}

export const useModels = (): UseModelsReturn => {
  const {
    providers,
    isLoading: providersLoading,
    error: providersError,
  } = useProviders();
  const [models, setModels] = useState<Model[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  const clearCache = useCallback(() => {
    cachedModels = null;
    hasLoadedModels = false;
  }, []);

  const refreshModels = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);

      // Use cached models if available
      if (cachedModels && hasLoadedModels) {
        setModels(cachedModels);
        setIsLoading(false);
        return;
      }

      // Collect all models from all providers
      const allModels: Model[] = [];
      for (const provider of providers) {
        if (provider.models && provider.models.length > 0) {
          allModels.push(...provider.models);
        }
      }

      // Sort models alphabetically by name
      const sortedModels = allModels.sort((a, b) =>
        a.name.localeCompare(b.name),
      );

      setModels(sortedModels);
      cachedModels = sortedModels;
      hasLoadedModels = true;
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to load models";
      setError(errorMessage);
      console.error("Error loading models:", err);
    } finally {
      setIsLoading(false);
    }
  }, [providers]);

  const getModelDisplayName = useCallback(
    (modelId: string): string => {
      const model = models.find((m) => m.id === modelId);
      if (model) {
        return model.name;
      }

      // Fallback: try to extract a readable name from the ID
      if (modelId.includes("/")) {
        const parts = modelId.split("/");
        return parts[parts.length - 1] || modelId;
      }

      return modelId;
    },
    [models],
  );

  // Update models when providers change or when providers are loaded for the first time
  useEffect(() => {
    if (!providersLoading && providers.length > 0) {
      // Clear cache when providers change to force refresh
      if (hasLoadedModels) {
        cachedModels = null;
        hasLoadedModels = false;
      }
      refreshModels();
    } else if (
      !providersLoading &&
      providers.length === 0 &&
      !hasLoadedModels
    ) {
      // No providers available, set empty models
      setModels([]);
      setIsLoading(false);
    }
  }, [providers, providersLoading, refreshModels]);

  // Handle provider errors
  useEffect(() => {
    if (providersError) {
      setError(providersError);
      setIsLoading(false);
    }
  }, [providersError]);

  return {
    models,
    isLoading: isLoading || providersLoading,
    error,
    refreshModels,
    getModelDisplayName,
    clearError,
    clearCache,
  };
};
