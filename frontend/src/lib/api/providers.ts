import { getApiUrl } from "@/lib/config";
import { FrontendProvider, ProviderRequest, ProviderResponse } from "./types";
import { getHeaders } from "./headers";

// Get all providers
export const getProviders = async (): Promise<ProviderResponse[]> => {
	const response = await fetch(getApiUrl("/api/providers/"), {
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
	const response = await fetch(getApiUrl("/api/providers/save"), {
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

// Delete provider
export const deleteProvider = async (id: string): Promise<void> => {
	const response = await fetch(getApiUrl(`/api/providers/delete/${id}`), {
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
): FrontendProvider => {
	return {
		id: backendProvider.id,
		name: getProviderDisplayName(backendProvider),
		baseUrl: backendProvider.base_url,
	};
};
