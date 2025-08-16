import { ChatRequest, ChatResponse, generateConversationId } from "./types.ts";
import { ApiErrorHandler, isChatResponse } from "./errorHandler.ts";
import { getApiUrl } from "../config.ts";

export class ChatAPI {
  constructor() {}

  async sendMessage(
    conversationId: string | null,
    activeMessageId: number | null,
    model: string,
    content: string,
    webSearch: boolean = false,
  ): Promise<ChatResponse & { conversationId: string }> {
    if (!model) {
      throw new Error("Valid model is required");
    }

    if (!content) {
      throw new Error("Valid message content is required");
    }

    return ApiErrorHandler.handleApiCall(async () => {
      const request: ChatRequest = {
        conversationId: conversationId || generateConversationId(),
        activeMessageId: activeMessageId || 1,
        model,
        content,
        webSearch,
      };

      const response = await fetch(getApiUrl("/api/chat"), {
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
        conversationId: request.conversationId,
      };
    }, "sendMessage");
  }
}

// Default instance
export const chatAPI = new ChatAPI();
