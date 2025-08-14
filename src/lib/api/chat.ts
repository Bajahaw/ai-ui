import {
  ChatRequest,
  ChatResponse,
  generateConversationId,
  Conversation,
  frontendToBackendMessage,
  CreateConversationRequest,
} from "./types";
import {
  ApiErrorHandler,
  isChatResponse,
  isCreateConversationResponse,
} from "./errorHandler";

// API client for chat endpoint
export class ChatAPI {
  private baseUrl: string;

  constructor(baseUrl: string = "") {
    this.baseUrl = baseUrl;
  }

  // POST /api/chat - handles both new conversations and existing ones
  async sendMessage(
    conversationId: string | null,
    activeMessageId: number | null,
    model: string,
    content: string,
    webSearch: boolean = false,
    title?: string,
  ): Promise<ChatResponse & { conversationId: string }> {
    if (!model) {
      throw new Error("Valid model is required");
    }

    if (!content) {
      throw new Error("Valid message content is required");
    }

    return ApiErrorHandler.handleApiCall(async () => {
      let finalConversationId = conversationId;
      let finalActiveMessageId = activeMessageId;

      // If no conversation ID, create new conversation first
      if (!conversationId) {
        finalConversationId = generateConversationId();
        finalActiveMessageId = 1;

        const userMessage = frontendToBackendMessage(
          {
            id: "1",
            role: "user",
            content: content,
            status: "success",
            timestamp: Date.now(),
          },
          1,
        );

        const conversation: Conversation = {
          id: finalConversationId,
          title:
            title ||
            (content.length > 50 ? content.substring(0, 47) + "..." : content),
          messages: {
            1: userMessage,
          },
          root: [1],
          activeMessageId: 1,
        };

        const createRequest: CreateConversationRequest = {
          conversation,
        };

        const createResponse = await fetch(
          `${this.baseUrl}/api/conversations/add`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
            },
            credentials: "include",
            body: JSON.stringify(createRequest),
          },
        );

        if (!createResponse.ok) {
          await ApiErrorHandler.handleFetchError(
            createResponse,
            "Create conversation",
          );
        }

        const createResult = await createResponse.json();
        ApiErrorHandler.validateResponse(
          createResult,
          isCreateConversationResponse,
          "Create conversation",
        );
      }

      // Validate for existing conversations
      if (
        conversationId &&
        (typeof finalActiveMessageId !== "number" || finalActiveMessageId < 0)
      ) {
        throw new Error(
          "Valid active message ID is required for existing conversations",
        );
      }

      const request: ChatRequest = {
        conversationId: finalConversationId!,
        activeMessageId: finalActiveMessageId!,
        model,
        content,
        webSearch,
      };

      const response = await fetch(`${this.baseUrl}/api/chat`, {
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
        conversationId: finalConversationId!,
      };
    }, "sendMessage");
  }
}

// Default instance
export const chatAPI = new ChatAPI();
