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

// Update tool enable flags (convenience function) - removed
// Update tool approval requirements - removed
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
