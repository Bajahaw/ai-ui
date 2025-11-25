import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  ReactNode,
} from "react";
import {
  getProviders,
  saveProvider,
  deleteProvider,
  backendToFrontendProvider,
} from "@/lib/api/providers";
import { ProviderRequest, ProviderResponse, FrontendProvider } from "@/lib/api/types";

export interface UseProvidersReturn {
  providers: FrontendProvider[];
  isLoading: boolean;
  error: string | null;

  refreshProviders: (forceRefresh?: boolean) => Promise<void>;
  addProvider: (providerData: ProviderRequest) => Promise<ProviderResponse>;
  removeProvider: (id: string) => Promise<void>;

  clearError: () => void;
}

const ProvidersContext = createContext<UseProvidersReturn | undefined>(
  undefined,
);

interface ProvidersProviderProps {
  children: ReactNode;
}

/**
 * ProvidersProvider - manages provider list only
 * Models are managed separately via ModelsContext
 */
export const ProvidersProvider = ({ children }: ProvidersProviderProps) => {
  const [providers, setProviders] = useState<FrontendProvider[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const clearError = useCallback(() => setError(null), []);

  const refreshProviders = useCallback(async (_forceRefresh?: boolean) => {
    setIsLoading(true);
    setError(null);

    try {
      const backendProviders = await getProviders();
      const frontendProviders = backendProviders.map((p) => backendToFrontendProvider(p));
      setProviders(frontendProviders);
    } catch (err) {
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
      await refreshProviders();
      return newProvider;
    },
    [refreshProviders],
  );

  const removeProvider = useCallback(
    async (id: string): Promise<void> => {
      setError(null);
      await deleteProvider(id);
      await refreshProviders();
    },
    [refreshProviders],
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
      removeProvider,
      clearError,
    }),
    [providers, isLoading, error, refreshProviders, addProvider, removeProvider, clearError],
  );

  return (
    <ProvidersContext.Provider value={value}>
      {children}
    </ProvidersContext.Provider>
  );
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
