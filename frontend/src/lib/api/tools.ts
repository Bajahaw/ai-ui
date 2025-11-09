import { getApiUrl } from "@/lib/config";
import { Tool, ToolListResponse } from "./types";

// Get all tools
export const getAllTools = async (): Promise<ToolListResponse> => {
  const response = await fetch(getApiUrl("/api/tools/all"), {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch tools: ${response.statusText}`);
  }

  return response.json();
};

// Save all tools with updated enable/disable states
export const saveAllTools = async (tools: Tool[]): Promise<void> => {
  const response = await fetch(getApiUrl("/api/tools/saveAll"), {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    credentials: "include",
    body: JSON.stringify({ tools }),
  });

  if (!response.ok) {
    throw new Error(`Failed to save tools: ${response.statusText}`);
  }
};

// Update tool enable flags (convenience function)
export const updateToolEnableFlags = async (
  flagUpdates: { id: string; is_enabled: boolean }[],
): Promise<void> => {
  if (flagUpdates.length === 0) return;

  // Get current list
  const current = await getAllTools();
  const updateMap = new Map<string, boolean>(
    flagUpdates.map((u) => [u.id, u.is_enabled]),
  );

  // Merge updates
  const merged: Tool[] = current.tools.map((t) =>
    updateMap.has(t.id)
      ? { ...t, is_enabled: updateMap.get(t.id)! }
      : t,
  );

  await saveAllTools(merged);
};

// Update tool approval requirements
export const updateToolApprovalFlags = async (
  approvalUpdates: { id: string; require_approval: boolean }[],
): Promise<void> => {
  if (approvalUpdates.length === 0) return;

  // Get current list
  const current = await getAllTools();
  const updateMap = new Map<string, boolean>(
    approvalUpdates.map((u) => [u.id, u.require_approval]),
  );

  // Merge updates
  const merged: Tool[] = current.tools.map((t) =>
    updateMap.has(t.id)
      ? { ...t, require_approval: updateMap.get(t.id)! }
      : t,
  );

  await saveAllTools(merged);
};

// Optimistic utility for local state updates
export function locallyApplyToolFlags(
  tools: Tool[],
  updates: { id: string; is_enabled?: boolean; require_approval?: boolean }[],
): Tool[] {
  if (updates.length === 0) return tools;
  const map = new Map<string, { is_enabled?: boolean; require_approval?: boolean }>(
    updates.map((u) => [u.id, u]),
  );
  return tools.map((t) => {
    const update = map.get(t.id);
    if (!update) return t;
    return {
      ...t,
      ...(update.is_enabled !== undefined && { is_enabled: update.is_enabled }),
      ...(update.require_approval !== undefined && { require_approval: update.require_approval }),
    };
  });
}
