import {
  Conversation,
  ConversationSearchHit,
  Message,
  WelcomeStats,
} from "./types.ts";

import {
  ApiErrorHandler,
  isConversation,
  isConversationArray,
  isMessagesMap,
} from "./errorHandler.ts";


import { getHeaders } from "./headers.ts";

// API client for conversation endpoints
export class ConversationsAPI {
  constructor() {}

  // GET /api/conversations
  async fetchConversations(): Promise<Conversation[]> {
    return ApiErrorHandler.handleApiCall(async () => {
      const response = await fetch("/api/conversations", {
        method: "GET",
        headers: getHeaders({
          "Content-Type": "application/json",
        }),
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

      const response = await fetch("/api/conversations/add", {
        method: "POST",

        headers: getHeaders({
          "Content-Type": "application/json",
        }),

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

  // GET /api/conversations/sync (SSE)
  // Returns an EventSource that streams ConversationEvents over a persistent connection.
  // The session ID is passed as a query param because EventSource cannot send custom headers.
  createSyncEventSource(sessionId: string): EventSource {
    const url = `/api/conversations/sync?sessionId=${encodeURIComponent(sessionId)}`;
    return new EventSource(url, { withCredentials: true });
  }

  // GET /api/conversations/{id}/messages
  async fetchConversationMessages(
    id: string,
  ): Promise<Record<number, Message>> {
    if (!id) {
      throw new Error("Invalid conversation ID provided");
    }

    return ApiErrorHandler.handleApiCall(async () => {
      const response = await fetch(
        `/api/conversations/${encodeURIComponent(id)}/messages`,
        {
          method: "GET",
          headers: getHeaders({
            "Content-Type": "application/json",
          }),
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
        `/api/conversations/${encodeURIComponent(id)}`,
        {
          method: "DELETE",
          headers: getHeaders({
            "Content-Type": "application/json",
          }),
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

  // GET /api/conversations/search?q=...
  // Supports AbortSignal so the sidebar can cancel in-flight queries while typing.
  async searchConversations(
    query: string,
    signal?: AbortSignal,
  ): Promise<ConversationSearchHit[]> {
    const q = query.trim();
    if (!q) {
      return [];
    }

    try {
      const response = await fetch(
        `/api/conversations/search?q=${encodeURIComponent(q)}`,
        {
          method: "GET",
          headers: getHeaders({
            "Content-Type": "application/json",
          }),
          credentials: "include",
          signal,
        },
      );

      if (!response.ok) {
        await ApiErrorHandler.handleFetchError(response, "Search conversations");
      }

      const data = await response.json();
      return Array.isArray(data) ? (data as ConversationSearchHit[]) : [];
    } catch (error) {
      // Aborts are expected during debounce — don't log them as API errors
      if (
        (error instanceof DOMException && error.name === "AbortError") ||
        (error instanceof Error && error.name === "AbortError") ||
        signal?.aborted
      ) {
        throw error;
      }
      console.error("API Error in searchConversations:", error);
      throw error instanceof Error
        ? error
        : new Error(`searchConversations: ${String(error)}`);
    }
  }

  // GET /api/conversations/stats
  async fetchStats(): Promise<WelcomeStats> {
    return ApiErrorHandler.handleApiCall(async () => {
      const response = await fetch("/api/conversations/stats", {
        method: "GET",
        headers: getHeaders({ "Content-Type": "application/json" }),
        credentials: "include",
      });
      if (!response.ok) {
        await ApiErrorHandler.handleFetchError(response, "Fetch stats");
      }
      const data = await response.json();
      return {
        totalTokens: data.totalTokens ?? 0,
        totalInputTokens: data.totalInputTokens ?? 0,
        totalConversations: data.totalConversations ?? 0,
        totalMessages: data.totalMessages ?? 0,
      };
    }, "fetchStats");
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
        `/api/conversations/${encodeURIComponent(id)}/rename`,
        {
          method: "POST",
          headers: getHeaders({
            "Content-Type": "application/json",
          }),
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
