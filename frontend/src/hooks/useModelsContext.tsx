import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  ReactNode,
} from "react";
import { getAllModels, saveAllModels } from "@/lib/api/models";
import { Model } from "@/lib/api/types";

interface ModelsContextValue {
  models: Model[];
  enabledModels: Model[];
  isLoading: boolean;
  error: string | null;
  
  refreshModels: () => Promise<void>;
  saveModels: (models: Model[]) => Promise<void>;
  updateModelsLocal: (models: Model[]) => void;
  
  // Filters
  getModelsByProvider: (providerId: string) => Model[];
  getEnabledModelsByProvider: (providerId: string) => Model[];
  getModelDisplayName: (modelId: string) => string;
  
  clearError: () => void;
}

const ModelsContext = createContext<ModelsContextValue | undefined>(undefined);

export const ModelsProvider = ({ children }: { children: ReactNode }) => {
  const [models, setModels] = useState<Model[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const clearError = useCallback(() => setError(null), []);

  const refreshModels = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await getAllModels();
      setModels(response.models);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to load models";
      setError(msg);
      console.error("Error loading models:", err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const updateModelsLocal = useCallback((newModels: Model[]) => {
    setModels(newModels);
  }, []);

  const saveModelsFn = useCallback(async (modelsToSave: Model[]) => {
    setError(null);
    try {
      await saveAllModels(modelsToSave);
      setModels(modelsToSave);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to save models";
      setError(msg);
      throw err;
    }
  }, []);

  // Derived: only enabled models
  const enabledModels = useMemo(
    () => models.filter((m) => m.is_enabled).sort((a, b) => a.name.localeCompare(b.name)),
    [models]
  );

  // Filter helpers
  const getModelsByProvider = useCallback(
    (providerId: string) => models.filter((m) => m.provider === providerId),
    [models]
  );

  const getEnabledModelsByProvider = useCallback(
    (providerId: string) => enabledModels.filter((m) => m.provider === providerId),
    [enabledModels]
  );

  const getModelDisplayName = useCallback(
    (modelId: string): string => {
      const m = models.find((x) => x.id === modelId);
      if (m) return m.name;
      if (modelId.includes("/")) {
        const parts = modelId.split("/");
        return parts[parts.length - 1] || modelId;
      }
      return modelId;
    },
    [models]
  );

  // Initial load
  useEffect(() => {
    refreshModels();
  }, [refreshModels]);

  const value = useMemo<ModelsContextValue>(
    () => ({
      models,
      enabledModels,
      isLoading,
      error,
      refreshModels,
      saveModels: saveModelsFn,
      updateModelsLocal,
      getModelsByProvider,
      getEnabledModelsByProvider,
      getModelDisplayName,
      clearError,
    }),
    [
      models,
      enabledModels,
      isLoading,
      error,
      refreshModels,
      saveModelsFn,
      updateModelsLocal,
      getModelsByProvider,
      getEnabledModelsByProvider,
      getModelDisplayName,
      clearError,
    ]
  );

  return (
    <ModelsContext.Provider value={value}>{children}</ModelsContext.Provider>
  );
};

export const useModelsContext = (): ModelsContextValue => {
  const ctx = useContext(ModelsContext);
  if (!ctx) {
    throw new Error("useModelsContext must be used within a ModelsProvider");
  }
  return ctx;
};
