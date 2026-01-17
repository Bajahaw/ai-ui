import { Conversation, Message } from "./types.ts";

import { ApiErrorHandler, isConversation, isConversationArray, isMessagesMap, } from "./errorHandler.ts";

import { getApiUrl } from "../config.ts";

// API client for conversation endpoints
export class ConversationsAPI {
	constructor() { }

	// GET /api/conversations
	async fetchConversations(): Promise<Conversation[]> {
		return ApiErrorHandler.handleApiCall(async () => {
			const response = await fetch(getApiUrl("/api/conversations"), {
				method: "GET",
				headers: {
					"Content-Type": "application/json",
				},
				credentials: "include",
			});

			if (!response.ok) {
				await ApiErrorHandler.handleFetchError(response, "Fetch conversations");
			}

			const data = await response.json();

			// Validate response structure
			const validatedData = ApiErrorHandler.validateResponse(
				data,
				isConversationArray,
				"Fetch conversations",
			);

			return validatedData || [];
		}, "fetchConversations");
	}

	// POST /api/conversations/add

	async createConversation(title: string): Promise<Conversation> {
		if (!title) {
			throw new Error("Valid conversation title is required");
		}

		return ApiErrorHandler.handleApiCall(async () => {
			const now = new Date().toISOString();

			// Only send fields that the backend expects
			const conversationPayload = {
				id: "test123",
				userId: "admin",
				title: title.trim(),
				createdAt: now,
				updatedAt: now,
			};

			const response = await fetch(getApiUrl("/api/conversations/add"), {
				method: "POST",

				headers: {
					"Content-Type": "application/json",
				},

				credentials: "include",

				body: JSON.stringify({ conversation: conversationPayload }),
			});

			if (!response.ok) {
				await ApiErrorHandler.handleFetchError(response, "Create conversation");
			}

			const result = await response.json();

			// Validate response structure (backend returns the created conversation)
			const validated = ApiErrorHandler.validateResponse(
				result,
				isConversation,
				"Create conversation",
			);

			// Add client-only fields that backend doesn't include
			return {
				...validated,
				messages: {},
			};
		}, "createConversation");
	}

	// GET /api/conversations/{id} removed - not needed currently
	// GET /api/conversations/{id}/messages
	async fetchConversationMessages(
		id: string,
	): Promise<Record<number, Message>> {
		if (!id) {
			throw new Error("Invalid conversation ID provided");
		}

		return ApiErrorHandler.handleApiCall(async () => {
			const response = await fetch(
				getApiUrl(`/api/conversations/${encodeURIComponent(id)}/messages`),
				{
					method: "GET",
					headers: {
						"Content-Type": "application/json",
					},
					credentials: "include",
				},
			);

			if (!response.ok) {
				await ApiErrorHandler.handleFetchError(
					response,
					`Fetch conversation ${id} messages`,
				);
			}

			const data = await response.json();

			// Validate messages map structure
			return ApiErrorHandler.validateResponse(
				data,
				isMessagesMap,
				`Fetch conversation ${id} messages`,
			);
		}, `fetchConversationMessages(${id})`);
	}

	// DELETE /api/conversations/{id}

	async deleteConversation(id: string): Promise<void> {
		if (!id) {
			throw new Error("Invalid conversation ID provided");
		}

		return ApiErrorHandler.handleApiCall(async () => {
			const response = await fetch(
				getApiUrl(`/api/conversations/${encodeURIComponent(id)}`),
				{
					method: "DELETE",
					headers: {
						"Content-Type": "application/json",
					},
					credentials: "include",
				},
			);

			if (!response.ok) {
				await ApiErrorHandler.handleFetchError(
					response,
					`Delete conversation ${id}`,
				);
			}
		}, `deleteConversation(${id})`);
	}

	// POST /api/conversations/{id}/rename
	async renameConversation(id: string, title: string): Promise<void> {
		if (!id) {
			throw new Error("Invalid conversation ID provided");
		}

		if (!title || title.trim() === "") {
			throw new Error("Valid title is required");
		}

		return ApiErrorHandler.handleApiCall(async () => {
			const response = await fetch(
				getApiUrl(`/api/conversations/${encodeURIComponent(id)}/rename`),
				{
					method: "POST",
					headers: {
						"Content-Type": "application/json",
					},
					credentials: "include",
					body: JSON.stringify({ title: title.trim() }),
				},
			);

			if (!response.ok) {
				await ApiErrorHandler.handleFetchError(
					response,
					`Rename conversation ${id}`,
				);
			}
		}, `renameConversation(${id})`);
	}
}

// Default instance
export const conversationsAPI = new ConversationsAPI();
