import {getApiUrl} from "@/lib/config";
import {FrontendProvider, ModelsResponse, ProviderRequest, ProviderResponse,} from "./types";

// Get all providers
export const getProviders = async (): Promise<ProviderResponse[]> => {
  const response = await fetch(getApiUrl("/api/providers/"), {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch providers: ${response.statusText}`);
  }

  return response.json();
};

// Get a specific provider - removed
// Save/update provider
export const saveProvider = async (
  providerData: ProviderRequest,
): Promise<ProviderResponse> => {
  const response = await fetch(getApiUrl("/api/providers/save"), {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(providerData),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to save provider: ${response.statusText}`);
  }

  return response.json();
};

// Delete provider
export const deleteProvider = async (id: string): Promise<void> => {
  const response = await fetch(getApiUrl(`/api/providers/delete/${id}`), {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
    },
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to delete provider: ${response.statusText}`);
  }
};

// Get models from a provider
export const getProviderModels = async (
  id: string,
): Promise<ModelsResponse> => {
  const response = await fetch(getApiUrl(`/api/providers/${id}/models`), {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(
      `Failed to fetch models for provider: ${response.statusText}`,
    );
  }

  return response.json();
};

// Fetch all models (across all providers) - removed
// Bulk update model enabled flags - removed
// Utility function to create a display name for providers

export const getProviderDisplayName = (provider: ProviderResponse): string => {
  try {
    const url = new URL(provider.base_url);
    return url.hostname || provider.id;
  } catch {
    return provider.id;
  }
};

// Convert backend provider to frontend provider
export const backendToFrontendProvider = (
  backendProvider: ProviderResponse,
  models?: ModelsResponse,
): FrontendProvider => {
  return {
    id: backendProvider.id,

    name: getProviderDisplayName(backendProvider),

    baseUrl: backendProvider.base_url,

    models: (models?.models || []).map((m) => ({
      ...m,

      // Normalize: if backend omits is_enabled or it's true, treat as enabled
      is_enabled: m.is_enabled,
    })),
  };
};
