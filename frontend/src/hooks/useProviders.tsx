import { createContext, useCallback, useContext, useEffect, useMemo, useState, ReactNode } from "react";
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

export interface UseProvidersReturn {
  providers: FrontendProvider[];
  isLoading: boolean;
  error: string | null;

  // Always fetches fresh data from backend; the 'forceRefresh' param is accepted for backward-compat but ignored.
  refreshProviders: (forceRefresh?: boolean) => Promise<void>;

  addProvider: (providerData: ProviderRequest) => Promise<ProviderResponse>;
  updateProvider: (providerData: ProviderRequest) => Promise<ProviderResponse>;
  removeProvider: (id: string) => Promise<void>;

  // Loads models for a specific provider and updates local state.
  loadModels: (providerId: string) => Promise<Model[]>;

  clearError: () => void;

  // Backward-compat no-op: caching has been removed.
  clearCache: () => void;
}

const ProvidersContext = createContext<UseProvidersReturn | undefined>(undefined);

interface ProvidersProviderProps {
  children: ReactNode;
}

/**
 * ProvidersProvider
 *
 * - Single source of truth for providers and their models.
 * - No caching layer: always fetches the latest from the backend on refresh.
 * - Operations (add/update/remove) refresh the list to keep all consumers in sync.
 * - Model list in the chat interface automatically updates because state is shared via context.
 */
export const ProvidersProvider = ({ children }: ProvidersProviderProps) => {
  const [providers, setProviders] = useState<FrontendProvider[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const clearError = useCallback(() => setError(null), []);

  // No-op for backward compatibility with existing calls
  const clearCache = useCallback(() => {
    // Intentionally left blank: caching removed
  }, []);

  const refreshProviders = useCallback(async (_forceRefresh?: boolean) => {
    setIsLoading(true);
    setError(null);

    try {
      const backendProviders = await getProviders();

      // For each provider, try to fetch its models.
      const frontendProviders = await Promise.all(
        backendProviders.map(async (provider) => {
          try {
            const modelsResponse = await getProviderModels(provider.id);
            return backendToFrontendProvider(provider, modelsResponse);
          } catch (modelErr) {
            // If models fail to load, still include provider without models.
            console.warn(`Failed to load models for provider ${provider.id}:`, modelErr);
            return backendToFrontendProvider(provider);
          }
        }),
      );

      setProviders(frontendProviders);
    } catch (err: any) {
      const msg = err instanceof Error ? err.message : "Failed to load providers";
      setError(msg);
      console.error("Error loading providers:", err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const addProvider = useCallback(
    async (providerData: ProviderRequest): Promise<ProviderResponse> => {
      setError(null);
      const newProvider = await saveProvider(providerData);
      // Always fetch fresh list after change
      await refreshProviders();
      return newProvider;
    },
    [refreshProviders],
  );

  const updateProvider = useCallback(
    async (providerData: ProviderRequest): Promise<ProviderResponse> => {
      setError(null);
      const updatedProvider = await saveProvider(providerData);
      // Always fetch fresh list after change
      await refreshProviders();
      return updatedProvider;
    },
    [refreshProviders],
  );

  const removeProvider = useCallback(
    async (id: string): Promise<void> => {
      setError(null);
      await deleteProvider(id);
      // Always fetch fresh list after change
      await refreshProviders();
    },
    [refreshProviders],
  );

  const loadModels = useCallback(
    async (providerId: string): Promise<Model[]> => {
      setError(null);
      const modelsResponse = await getProviderModels(providerId);

      // Update models for the specific provider in local state
      setProviders((prev) =>
        prev.map((p) =>
          p.id === providerId ? { ...p, models: modelsResponse.models } : p,
        ),
      );

      return modelsResponse.models;
    },
    [],
  );

  // Initial load
  useEffect(() => {
    refreshProviders();
  }, [refreshProviders]);

  const value = useMemo<UseProvidersReturn>(
    () => ({
      providers,
      isLoading,
      error,
      refreshProviders,
      addProvider,
      updateProvider,
      removeProvider,
      loadModels,
      clearError,
      clearCache, // still exposed for backward-compat (no-op)
    }),
    [
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
    ],
  );

  return <ProvidersContext.Provider value={value}>{children}</ProvidersContext.Provider>;
};

/**
 * useProviders hook
 * Must be used within a ProvidersProvider.
 */
export const useProviders = (): UseProvidersReturn => {
  const ctx = useContext(ProvidersContext);
  if (!ctx) {
    throw new Error("useProviders must be used within a ProvidersProvider");
  }
  return ctx;
};
