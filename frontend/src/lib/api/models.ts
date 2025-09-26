/**
 * Models API helper
 *
 * Uses the new global model management endpoints:
 *   GET  /api/models/all       -> returns all models (enabled + disabled)
 *   POST /api/models/save-all  -> persists model enable/disable changes
 *
 * Conventions:
 * - A "Model" (from ./types) includes: id, name, provider, is_enabled
 * - For bulk updates where only enable flags change, you can call
 *   updateModelEnableFlags([...]) with a minimal payload.
 *
 * If the backend expects the complete list for save-all, use saveAllModels.
 */

import { Model, ModelsResponse } from "./types";
import { getApiUrl } from "../config";

interface SaveAllModelsRequest {
  models: Array<{
    id: string;
    name: string;
    provider: string;
    is_enabled: boolean;
  }>;
}

interface EnableFlagsUpdate {
  id: string;
  is_enabled: boolean;
}

/**
 * Fetch all models (enabled + disabled).
 */
export async function getAllModels(): Promise<ModelsResponse> {
  const response = await fetch(getApiUrl("/api/models/all"), {
    method: "GET",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch models: ${response.status} ${response.statusText}`);
  }

  return response.json();
}

/**
 * Save (overwrite) the entire models list with new enable/disable states.
 * Use this if you already have the full list (e.g., from getAllModels) and
 * want to persist a set of changes in one shot.
 */
export async function saveAllModels(models: Model[]): Promise<void> {
  const body: SaveAllModelsRequest = {
    models: models.map((m) => ({
      id: m.id,
      name: m.name,
      provider: m.provider,
      is_enabled: m.is_enabled,
    })),
  };

  const response = await fetch(getApiUrl("/api/models/save-all"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    throw new Error(`Failed to save models: ${response.status} ${response.statusText}`);
  }
}

/**
 * Convenience helper to update only enable flags.
 *
 * Strategy:
 * 1. Fetch current list
 * 2. Merge the enable flags for provided ids
 * 3. Call saveAllModels with the full updated list
 *
 * This keeps the API simple while still allowing partial updates on the client side.
 */
export async function updateModelEnableFlags(flagUpdates: EnableFlagsUpdate[]): Promise<void> {
  if (flagUpdates.length === 0) return;

  // Get the current full list
  const current = await getAllModels();
  const updateMap = new Map<string, boolean>(
    flagUpdates.map((u) => [u.id, u.is_enabled]),
  );

  const merged: Model[] = current.models.map((m) =>
    updateMap.has(m.id) ? { ...m, is_enabled: updateMap.get(m.id)! } : m,
  );

  await saveAllModels(merged);
}

/**
 * Optimistic utility:
 * Applies enable/disable to a local array (immutable) so UI can update while request is in-flight.
 */
export function locallyApplyEnableFlags(
  models: Model[],
  updates: EnableFlagsUpdate[],
): Model[] {
  if (updates.length === 0) return models;
  const map = new Map<string, boolean>(updates.map((u) => [u.id, u.is_enabled]));
  return models.map((m) =>
    map.has(m.id) ? { ...m, is_enabled: map.get(m.id)! } : m,
  );
}
