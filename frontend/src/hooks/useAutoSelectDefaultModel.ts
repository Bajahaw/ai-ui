import { useEffect, useRef, useState } from "react";
import { useModels } from "./useModels";
import { useSettings } from "./useSettings";
import { useProviders } from "./useProviders";

/**
 * Hook that automatically selects the first available model as default
 * ONLY when adding the very first provider (going from 0 to 1+ providers)
 *
 * Behavior:
 * - ✅ Fresh app + first provider added → Auto-selects first model
 * - ❌ Additional providers added → No action (preserves user preferences)
 * - ❌ Page refresh with existing providers → No action (initialization only)
 * - ✅ Default model becomes unavailable → Updates to available model (fallback)
 *
 * This ensures user preferences are never overridden when adding more providers,
 * while still providing a good default experience for new users.
 *
 * @returns {object} Object containing the current auto-selected model ID
 */
export const useAutoSelectDefaultModel = () => {
  const { models, isLoading: modelsLoading } = useModels();
  const { providers, isLoading: providersLoading } = useProviders();
  const {
    getSingleSetting,
    updateSingleSetting,
    isLoading: settingsLoading,
  } = useSettings();
  const hasAutoSelected = useRef(false);
  const previousProviderCount = useRef<number | null>(null);
  const [autoSelectedModel, setAutoSelectedModel] = useState<string>("");

  useEffect(() => {
    // Only proceed if everything is loaded
    if (modelsLoading || settingsLoading || providersLoading) {
      return;
    }

    const currentProviderCount = providers.length;
    const currentDefaultModel = getSingleSetting("defaultModel");

    // Check if this is the transition from 0 providers to having providers
    // Handle initialization: if previousProviderCount is null, this is the first render
    const isFirstProviderAdded =
      previousProviderCount.current === 0 &&
      currentProviderCount > 0 &&
      models.length > 0;

    // Initialize previousProviderCount on first render
    if (previousProviderCount.current === null) {
      previousProviderCount.current = currentProviderCount;
      // On initialization, don't trigger auto-selection if providers already exist
      // This handles page refresh scenarios where providers are already loaded
      return;
    }

    // Update the previous count for next comparison
    previousProviderCount.current = currentProviderCount;

    // ONLY auto-select when adding the very first provider
    if (
      isFirstProviderAdded &&
      !currentDefaultModel &&
      !hasAutoSelected.current
    ) {
      const firstModel = models[0];
      if (firstModel) {
        console.log(
          `Auto-selecting first model "${firstModel.name}" (${firstModel.id}) as default when adding first provider`,
        );
        hasAutoSelected.current = true;
        setAutoSelectedModel(firstModel.id);
        updateSingleSetting("defaultModel", firstModel.id).catch((error) => {
          console.warn("Failed to auto-set default model:", error);
          hasAutoSelected.current = false; // Reset on failure
          setAutoSelectedModel("");
        });
      }
    } else if (
      currentProviderCount > 0 &&
      !isFirstProviderAdded &&
      !currentDefaultModel
    ) {
      // Skip auto-selection when adding additional providers
      console.log(
        `Skipping auto-selection: additional provider added (${currentProviderCount} providers), user preferences preserved`,
      );
    } else if (currentDefaultModel && models.length > 0) {
      // Handle existing default model validation (not auto-selection)
      const isCurrentModelAvailable = models.some(
        (model) => model.id === currentDefaultModel,
      );

      if (isCurrentModelAvailable) {
        // Model is available, set it as the current model
        setAutoSelectedModel(currentDefaultModel);
      } else {
        // If the current default model is no longer available, select the first available one
        // This handles the case where a provider with the default model is removed
        const firstModel = models[0];
        if (firstModel) {
          setAutoSelectedModel(firstModel.id);
          updateSingleSetting("defaultModel", firstModel.id).catch((error) => {
            console.warn("Failed to update default model:", error);
          });
        }
      }
    }
  }, [
    models,
    providers,
    modelsLoading,
    settingsLoading,
    providersLoading,
    getSingleSetting,
    updateSingleSetting,
  ]);

  return { autoSelectedModel };
};
