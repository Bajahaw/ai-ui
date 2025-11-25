import {useCallback, useEffect, useState} from "react";

import {Model} from "@/lib/api/types";

import {useProviders} from "./useProviders";

/**

 * Simplified useModels hook

 *

 * - Single source of truth: derives model list directly from providers returned by useProviders

 * - No global caches or subscribers

 * - Automatically updates when providers change (add/update/remove)

 * - Exposes a lightweight refresh which attempts to load models for providers that currently have none

 */

interface UseModelsReturn {
  models: Model[]; // Only enabled models

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

    refreshProviders,

    loadModels,
  } = useProviders();

  const [models, setModels] = useState<Model[]>([]);

  const [isLoading, setIsLoading] = useState<boolean>(providersLoading);

  const [error, setError] = useState<string | null>(providersError || null);

  // Helper to aggregate models from the providers array.

  const aggregateModelsFromProviders = useCallback(
    (providersList: typeof providers) => {
      const all = providersList.flatMap((p) => p.models || []);

      // Deduplicate by id in case multiple providers expose same id (keep first)

      const seen = new Set<string>();

      const unique: Model[] = [];

      for (const m of all) {
        if (!seen.has(m.id)) {
          seen.add(m.id);

          unique.push(m);
        }
      }

      // Sort alphabetically for predictable UI order

      unique.sort((a, b) => a.name.localeCompare(b.name));

      // Return only enabled models
      return unique.filter((m) => m.is_enabled);
    },

    [],
  );

  // Recompute models whenever providers change

  useEffect(() => {
    setModels(aggregateModelsFromProviders(providers));

    // sync loading / error state from providers

    setIsLoading(providersLoading);

    setError(providersError || null);
  }, [
    providers,

    providersLoading,

    providersError,

    aggregateModelsFromProviders,
  ]);

  // Refresh models: try to explicitly load models for providers that don't have models yet,

  // then re-aggregate. This is a gentle attempt; errors for specific providers won't block others.

  const refreshModels = useCallback(async (): Promise<void> => {
    setIsLoading(true);

    setError(null);

    try {
      // If providers are stale, consider refreshing the providers first.

      // Some provider operations update provider list; calling refreshProviders keeps them in sync.

      try {
        await refreshProviders();
      } catch {
        // ignore provider refresh errors here; we'll still attempt per-provider loads
      }

      // Attempt to fetch models from providers that are missing them

      const loadPromises = providers.map(async (p) => {
        if (!p.models || p.models.length === 0) {
          try {
            await loadModels(p.id);
          } catch (e) {
            // swallow per-provider load errors (we still want other providers to try)
            // optionally we could collect per-provider errors if needed
            // console.warn("Failed to load models for provider", p.id, e);
          }
        }
      });

      await Promise.all(loadPromises);

      // After attempting loads, re-aggregate from the (possibly updated) providers array.

      // Note: useProviders should update its providers state after loadModels calls,

      // but we re-derive from the current providers snapshot to be robust.

      setModels(aggregateModelsFromProviders(providers));
    } catch (err: any) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setIsLoading(false);
    }
  }, [providers, refreshProviders, loadModels, aggregateModelsFromProviders]);

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

    [models],
  );

  const clearError = useCallback(() => setError(null), []);

  const clearCache = useCallback(() => {
    // No persistent cache in this simplified hook. Clearing will just clear the current list.

    setModels([]);

    setError(null);
  }, []);

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
