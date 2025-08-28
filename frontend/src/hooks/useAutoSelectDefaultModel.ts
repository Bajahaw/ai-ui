import { useEffect } from "react";
import { useModels } from "./useModels";
import { useSettings } from "./useSettings";

/**
 * Hook that automatically selects the first available model as default
 * when no default model is set and models become available
 */
export const useAutoSelectDefaultModel = () => {
  const { models, isLoading: modelsLoading } = useModels();
  const { getSingleSetting, updateSingleSetting } = useSettings();

  useEffect(() => {
    // Only proceed if models are loaded and available
    if (modelsLoading || models.length === 0) {
      return;
    }

    const currentDefaultModel = getSingleSetting("defaultModel");

    // If no default model is set, automatically select the first available model
    if (!currentDefaultModel) {
      const firstModel = models[0];
      if (firstModel) {
        updateSingleSetting("defaultModel", firstModel.id).catch((error) => {
          console.warn("Failed to auto-set default model:", error);
        });
      }
    } else {
      // If a default model is set, verify it's still available
      const isCurrentModelAvailable = models.some(
        (model) => model.id === currentDefaultModel
      );

      // If the current default model is no longer available, select the first available one
      if (!isCurrentModelAvailable) {
        const firstModel = models[0];
        if (firstModel) {
          updateSingleSetting("defaultModel", firstModel.id).catch((error) => {
            console.warn("Failed to update default model:", error);
          });
        }
      }
    }
  }, [models, modelsLoading, getSingleSetting, updateSingleSetting]);
};
