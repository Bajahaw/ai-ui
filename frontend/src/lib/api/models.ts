/**
 * Models API helper
 *
 * Uses global model management endpoints:
 *   GET  /api/models/all       -> returns all models (enabled + disabled)
 *   POST /api/models/save-all  -> persists model enable/disable changes
 */

import { Model, ModelsResponse } from "./types";
import { getApiUrl } from "../config";
import { getHeaders } from "./headers";

/**
 * Fetch all models (enabled + disabled).
 */
export async function getAllModels(): Promise<ModelsResponse> {
	const response = await fetch(getApiUrl("/api/models/all"), {
		method: "GET",
		headers: getHeaders({ "Content-Type": "application/json" }),
		credentials: "include",
	});

	if (!response.ok) {
		throw new Error(`Failed to fetch models: ${response.status} ${response.statusText}`);
	}

	return response.json();
}

/**
 * Save the entire models list with updated enable/disable states.
 */
export async function saveAllModels(models: Model[]): Promise<void> {
	const body = {
		models: models.map((m) => ({
			id: m.id,
			name: m.name,
			provider: m.provider,
			is_enabled: m.is_enabled,
		})),
	};

	const response = await fetch(getApiUrl("/api/models/save-all"), {
		method: "POST",
		headers: getHeaders({ "Content-Type": "application/json" }),
		credentials: "include",
		body: JSON.stringify(body),
	});

	if (!response.ok) {
		throw new Error(`Failed to save models: ${response.status} ${response.statusText}`);
	}
}

/**
 * Optimistic utility:
 * Applies enable/disable to a local array (immutable) so UI can update while request is in-flight.
 */
export function locallyApplyEnableFlags(
	models: Model[],
	updates: { id: string; is_enabled: boolean }[],
): Model[] {
	if (updates.length === 0) return models;
	const map = new Map<string, boolean>(updates.map((u) => [u.id, u.is_enabled]));
	return models.map((m) =>
		map.has(m.id) ? { ...m, is_enabled: map.get(m.id)! } : m,
	);
}
