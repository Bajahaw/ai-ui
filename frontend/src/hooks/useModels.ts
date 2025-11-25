import { useModelsContext } from "./useModelsContext";

/**
 * useModels hook - simplified wrapper around ModelsContext
 * Returns only enabled models for chat interface and model selection
 */
export const useModels = () => {
  const {
    enabledModels: models,
    isLoading,
    error,
    refreshModels,
    getModelDisplayName,
    clearError,
  } = useModelsContext();

  return {
    models,
    isLoading,
    error,
    refreshModels,
    getModelDisplayName,
    clearError,
  };
};
