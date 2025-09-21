import {
  ChatRequest,
  ChatResponse,
  RetryRequest,
  RetryResponse,
  UpdateRequest,
  UpdateResponse,
} from "./types.ts";

import { ApiErrorHandler, isChatResponse } from "./errorHandler.ts";
import { getApiUrl } from "../config.ts";

export class ChatAPI {
  constructor() {}

  async sendMessage(
    conversationId: string | null,
    parentId: number | null,
    model: string,
    content: string,
    webSearch: boolean = false,
    attachment?: string,
  ): Promise<ChatResponse & { conversationId: string }> {
    if (!model) {
      throw new Error("Valid model is required");
    }

    if (!content) {
      throw new Error("Valid message content is required");
    }

    return ApiErrorHandler.handleApiCall(async () => {
      const request: ChatRequest = {
        conversationId: conversationId,
        parentId: parentId || 0,
        model,
        content,
        webSearch,
        attachment,
      };

      const response = await fetch(getApiUrl("/api/chat/new"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        await ApiErrorHandler.handleFetchError(response, "Send message");
      }

      const data = await response.json();

      // Validate response structure
      const validatedData = ApiErrorHandler.validateResponse(
        data,
        isChatResponse,
        "Send message",
      );

      return {
        ...validatedData,
        // Ensure we always return a string for conversationId (use empty string when null)
        conversationId: request.conversationId || "",
      };
    }, "sendMessage");
  }

  async retryMessage(
    conversationId: string,
    parentId: number,
    model: string,
  ): Promise<RetryResponse> {
    if (!conversationId) {
      throw new Error("Valid conversation ID is required");
    }

    if (!model) {
      throw new Error("Valid model is required");
    }

    return ApiErrorHandler.handleApiCall(async () => {
      const request: RetryRequest = {
        conversationId,
        parentId,
        model,
      };

      const response = await fetch(getApiUrl("/api/chat/retry"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        await ApiErrorHandler.handleFetchError(response, "Retry message");
      }

      const data = await response.json();

      // Validate response structure
      return ApiErrorHandler.validateResponse(
        data,
        isChatResponse, // Reuse the same validator since structure is the same
        "Retry message",
      );
    }, "retryMessage");
  }

  async updateMessage(
    conversationId: string,
    messageId: number,
    content: string,
  ): Promise<UpdateResponse> {
    if (!conversationId) {
      throw new Error("Valid conversation ID is required");
    }

    if (!content) {
      throw new Error("Valid message content is required");
    }

    return ApiErrorHandler.handleApiCall(async () => {
      const request: UpdateRequest = {
        conversationId,
        messageId,
        content,
      };

      const response = await fetch(getApiUrl("/api/chat/update"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        await ApiErrorHandler.handleFetchError(response, "Update message");
      }

      const data = await response.json();

      // Validate response structure
      return ApiErrorHandler.validateResponse(
        data,
        isChatResponse, // Reuse the same validator since structure is the same
        "Update message",
      );
    }, "updateMessage");
  }
}

// Default instance
export const chatAPI = new ChatAPI();
