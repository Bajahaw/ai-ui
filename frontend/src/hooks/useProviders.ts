import { useState, useEffect, useCallback } from "react";
import {
  getProviders,
  saveProvider,
  deleteProvider,
  getProviderModels,
  backendToFrontendProvider,
} from "@/lib/api/providers";
import {
  ProviderRequest,
  ProviderResponse,
  FrontendProvider,
  Model,
} from "@/lib/api/types";

// Simple in-memory storage for providers
let cachedProviders: FrontendProvider[] | null = null;
let hasLoadedProviders = false;

interface UseProvidersReturn {
  providers: FrontendProvider[];
  isLoading: boolean;
  error: string | null;
  refreshProviders: (forceRefresh?: boolean) => Promise<void>;
  addProvider: (providerData: ProviderRequest) => Promise<ProviderResponse>;
  updateProvider: (providerData: ProviderRequest) => Promise<ProviderResponse>;
  removeProvider: (id: string) => Promise<void>;
  loadModels: (providerId: string) => Promise<Model[]>;
  clearError: () => void;
  clearCache: () => void;
}

export const useProviders = (): UseProvidersReturn => {
  const [providers, setProviders] = useState<FrontendProvider[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasInitialized, setHasInitialized] = useState(false);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  const clearCache = useCallback(() => {
    cachedProviders = null;
    hasLoadedProviders = false;
  }, []);

  const refreshProviders = useCallback(
    async (forceRefresh: boolean = false) => {
      try {
        setIsLoading(true);
        setError(null);

        // Use cached providers unless force refresh is requested
        if (!forceRefresh && cachedProviders && hasLoadedProviders) {
          setProviders(cachedProviders);
          setIsLoading(false);
          setHasInitialized(true);
          return;
        }

        const backendProviders = await getProviders();
        const frontendProviders = await Promise.all(
          backendProviders.map(async (provider) => {
            try {
              // Try to load models for each provider
              const modelsResponse = await getProviderModels(provider.id);
              return backendToFrontendProvider(provider, modelsResponse);
            } catch (modelError) {
              // If models fail to load, still include the provider without models
              console.warn(
                `Failed to load models for provider ${provider.id}:`,
                modelError,
              );
              return backendToFrontendProvider(provider);
            }
          }),
        );

        setProviders(frontendProviders);
        cachedProviders = frontendProviders;
        hasLoadedProviders = true;
      } catch (err) {
        const errorMessage =
          err instanceof Error ? err.message : "Failed to load providers";
        setError(errorMessage);
        console.error("Error loading providers:", err);
      } finally {
        setIsLoading(false);
        setHasInitialized(true);
      }
    },
    [],
  );

  const addProvider = useCallback(
    async (providerData: ProviderRequest): Promise<ProviderResponse> => {
      try {
        setError(null);
        const newProvider = await saveProvider(providerData);

        // Clear cache and refresh the providers list to include the new provider
        cachedProviders = null;
        hasLoadedProviders = false;
        await refreshProviders(true);

        return newProvider;
      } catch (err) {
        const errorMessage =
          err instanceof Error ? err.message : "Failed to add provider";
        setError(errorMessage);
        throw err;
      }
    },
    [refreshProviders],
  );

  const updateProvider = useCallback(
    async (providerData: ProviderRequest): Promise<ProviderResponse> => {
      try {
        setError(null);
        const updatedProvider = await saveProvider(providerData);

        // Clear cache and refresh the providers list to reflect changes
        cachedProviders = null;
        hasLoadedProviders = false;
        await refreshProviders(true);

        return updatedProvider;
      } catch (err) {
        const errorMessage =
          err instanceof Error ? err.message : "Failed to update provider";
        setError(errorMessage);
        throw err;
      }
    },
    [refreshProviders],
  );

  const removeProvider = useCallback(
    async (id: string): Promise<void> => {
      try {
        setError(null);
        await deleteProvider(id);

        // Remove the provider from local state immediately for better UX
        const updatedProviders = providers.filter(
          (provider) => provider.id !== id,
        );
        setProviders(updatedProviders);
        cachedProviders = updatedProviders;
      } catch (err) {
        const errorMessage =
          err instanceof Error ? err.message : "Failed to delete provider";
        setError(errorMessage);
        // Clear cache and refresh providers to restore the state if deletion failed
        cachedProviders = null;
        hasLoadedProviders = false;
        await refreshProviders(true);
        throw err;
      }
    },
    [providers, refreshProviders],
  );

  const loadModels = useCallback(
    async (providerId: string): Promise<Model[]> => {
      try {
        setError(null);
        const modelsResponse = await getProviderModels(providerId);

        // Update the provider in the local state with the new models
        const updatedProviders = providers.map((provider) =>
          provider.id === providerId
            ? { ...provider, models: modelsResponse.models }
            : provider,
        );

        setProviders(updatedProviders);
        cachedProviders = updatedProviders;

        return modelsResponse.models;
      } catch (err) {
        const errorMessage =
          err instanceof Error ? err.message : "Failed to load models";
        setError(errorMessage);
        throw err;
      }
    },
    [providers],
  );

  // Load providers on mount or when cache is empty
  useEffect(() => {
    if (!hasInitialized) {
      refreshProviders();
    }
  }, [refreshProviders, hasInitialized]);

  return {
    providers,
    isLoading,
    error,
    refreshProviders,
    addProvider,
    updateProvider,
    removeProvider,
    loadModels,
    clearError,
    clearCache,
  };
};
