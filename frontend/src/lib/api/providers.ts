
import { FrontendProvider, ProviderRequest, ProviderResponse } from "./types";
import { getHeaders } from "./headers";

// Get all providers
export const getProviders = async (): Promise<ProviderResponse[]> => {
  const response = await fetch("/api/providers/", {
    method: "GET",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
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
  const response = await fetch("/api/providers/save", {
    method: "POST",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(providerData),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to save provider: ${response.statusText}`);
  }

  return response.json();
};

// Refresh models for a specific provider (re-fetches from provider API)
export const refreshProviderModels = async (id: string): Promise<void> => {
  const response = await fetch(
    `/api/providers/refresh-models/${id}`,
    {
      method: "POST",
      headers: getHeaders({
        "Content-Type": "application/json",
      }),
      credentials: "include",
    },
  );

  if (!response.ok) {
    throw new Error(
      `Failed to refresh provider models: ${response.statusText}`,
    );
  }
};

// Delete provider
export const deleteProvider = async (id: string): Promise<void> => {
  const response = await fetch(`/api/providers/delete/${id}`, {
    method: "DELETE",
    headers: getHeaders({
      "Content-Type": "application/json",
    }),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to delete provider: ${response.statusText}`);
  }
};

// Utility function to create a display name for providers

export const getProviderDisplayName = (provider: ProviderResponse): string => {
  if (provider.type === "chatgpt-oauth" || provider.base_url?.startsWith("chatgpt://")) {
    return "ChatGPT";
  }
  try {
    const url = new URL(provider.base_url);
    return url.hostname || provider.id;
  } catch {
    return provider.id;
  }
};

/**
 * Short label shown next to model names in selectors.
 * Matches classic provider ids like "openai-a3f2" (name + ~4 chars).
 * ChatGPT becomes "chatgpt-a1b2" so multi-account stays distinguishable without
 * bloating the UI with the full internal provider id.
 */
export const formatProviderSelectLabel = (providerId: string): string => {
  if (!providerId) return "";

  // New short ids: cgpt-<6hex>
  if (providerId.startsWith("cgpt-")) {
    const hash = providerId.slice(5);
    return `chatgpt-${hash.slice(0, 4)}`;
  }

  // Legacy long ids: chatgpt-<account>-<username>
  if (providerId.startsWith("chatgpt-")) {
    const rest = providerId.slice("chatgpt-".length);
    const accountPart = rest.split("-")[0] || rest;
    return `chatgpt-${accountPart.slice(0, 4)}`;
  }

  // Already short (e.g. openai-a3f2)
  if (providerId.length <= 18) {
    return providerId;
  }

  // Long custom ids: keep first segment + 4-char suffix
  const parts = providerId.split("-");
  if (parts.length >= 2) {
    return `${parts[0]}-${parts[parts.length - 1].slice(0, 4)}`;
  }
  return providerId.slice(0, 12);
};

// Convert backend provider to frontend provider
export const backendToFrontendProvider = (
  backendProvider: ProviderResponse,
): FrontendProvider => {
  return {
    id: backendProvider.id,
    name: getProviderDisplayName(backendProvider),
    type: backendProvider.type,
    baseUrl: backendProvider.base_url,
    headers: backendProvider.headers,
  };
};
