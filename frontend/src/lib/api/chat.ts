import {
  ChatRequest,
  ChatResponse,
  RetryRequest,
  RetryResponse,
  UpdateRequest,
  UpdateResponse,
  StreamMetadata,
  StreamChunk,
  StreamComplete,
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

  async sendMessageStream(
    conversationId: string | null,
    parentId: number | null,
    model: string,
    content: string,
    webSearch: boolean = false,
    attachment?: string,
    onChunk?: (chunk: string) => void,
    onMetadata?: (metadata: StreamMetadata) => void,
    onComplete?: (data: StreamComplete) => void,
    onError?: (error: string) => void,
  ): Promise<void> {
    if (!model) {
      throw new Error("Valid model is required");
    }

    if (!content) {
      throw new Error("Valid message content is required");
    }

    const request: ChatRequest = {
      conversationId: conversationId,
      parentId: parentId || 0,
      model,
      content,
      webSearch,
      attachment,
    };

    try {
      const response = await fetch(getApiUrl("/api/chat/stream"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Stream request failed: ${response.statusText} - ${errorText}`);
      }

      const reader = response.body?.getReader();
      if (!reader) {
        throw new Error("No response body available for streaming");
      }

      const decoder = new TextDecoder();
      let buffer = "";
      let currentEvent = "";

      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() || "";

          for (const line of lines) {
            if (line.startsWith("event: ")) {
              currentEvent = line.slice(7).trim();
            } else if (line.startsWith("data: ")) {
              const data = line.slice(6);

              if (currentEvent === "metadata" && onMetadata) {
                try {
                  const metadata: StreamMetadata = JSON.parse(data);
                  onMetadata(metadata);
                } catch (e) {
                  console.error("Failed to parse metadata:", e);
                }
                currentEvent = "";
              } else if (currentEvent === "complete" && onComplete) {
                try {
                  const completeData: StreamComplete = JSON.parse(data);
                  onComplete(completeData);
                } catch (e) {
                  console.error("Failed to parse complete data:", e);
                }
                currentEvent = "";
              } else if (currentEvent === "error") {
                if (onError) {
                  try {
                    const errorData = JSON.parse(data);
                    onError(errorData.error || "Unknown error");
                  } catch (e) {
                    onError(data);
                  }
                }
                currentEvent = "";
              } else if (currentEvent === "" && onChunk) {
                // Regular content chunk (no event prefix)
                try {
                  const chunk: StreamChunk = JSON.parse(data);
                  onChunk(chunk.content);
                } catch (e) {
                  console.error("Failed to parse chunk:", e);
                }
              }
            } else if (line === "") {
              // Empty line - reset event
              currentEvent = "";
            }
          }
        }
      } finally {
        reader.releaseLock();
      }
    } catch (err) {
      console.error("Stream error:", err);
      if (onError) {
        onError(err instanceof Error ? err.message : String(err));
      }
      throw err;
    }
  }
}

// Default instance
export const chatAPI = new ChatAPI();
