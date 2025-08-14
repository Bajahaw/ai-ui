import {
  Conversation,
  CreateConversationRequest,
  frontendToBackendMessage,
  generateConversationId,
} from "./types";
import {
  ApiErrorHandler,
  isConversation,
  isConversationArray,
  isCreateConversationResponse,
} from "./errorHandler";
import { getApiUrl } from "../config";

// API client for conversation endpoints
export class ConversationsAPI {
  constructor() {}

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
  async createConversation(
    title: string,
    firstMessage: string,
  ): Promise<string> {
    if (!title) {
      throw new Error("Valid conversation title is required");
    }

    if (!firstMessage) {
      throw new Error("Valid first message is required");
    }

    return ApiErrorHandler.handleApiCall(async () => {
      const conversationId = generateConversationId();

      // Create initial conversation structure with first user message
      const userMessage = frontendToBackendMessage(
        {
          id: "1",
          role: "user",
          content: firstMessage,
          status: "success",
          timestamp: Date.now(),
        },
        1, // First message gets ID 1
      );

      const conversation: Conversation = {
        id: conversationId,
        title,
        messages: {
          1: userMessage,
        },
        root: [1],
        activeMessageId: 1,
      };

      const request: CreateConversationRequest = {
        conversation,
      };

      const response = await fetch(getApiUrl("/api/conversations/add"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        await ApiErrorHandler.handleFetchError(response, "Create conversation");
      }

      const result = await response.json();

      // Validate response structure
      const validatedResult = ApiErrorHandler.validateResponse(
        result,
        isCreateConversationResponse,
        "Create conversation",
      );

      return validatedResult.id;
    }, "createConversation");
  }

  // GET /api/conversations/{id}
  async fetchConversation(id: string): Promise<Conversation> {
    if (!id) {
      throw new Error("Invalid conversation ID provided");
    }

    return ApiErrorHandler.handleApiCall(async () => {
      const response = await fetch(
        getApiUrl(`/api/conversations/${encodeURIComponent(id)}`),
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
          `Fetch conversation ${id}`,
        );
      }

      const data = await response.json();

      // Validate conversation data structure
      return ApiErrorHandler.validateResponse(
        data,
        isConversation,
        `Fetch conversation ${id}`,
      );
    }, `fetchConversation(${id})`);
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
