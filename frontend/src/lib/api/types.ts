// Backend API Types matching Go structures

export interface Message {
  id: number;

  convId: string;
  role: string;

  content: string;

  parentId?: number;

  children: number[];

  attachment?: string;
}

export interface Conversation {
  id: string;

  userId: string;
  title?: string;

  createdAt: string;
  updatedAt: string;

  // Client-only compatibility fields
  messages?: Record<number, Message>;
  root?: number[];
  activeMessageId?: number;
  activeBranches?: Record<number, number>; // messageId -> activeChildId mapping
}

export interface ChatRequest {
  conversationId: string | null;
  parentId: number;
  model: string;
  content: string;
  webSearch?: boolean;
  attachment?: string;
}

export interface ChatResponse {
  messages: Record<number, Message>;
}

export interface RetryRequest {
  conversationId: string;
  parentId: number;
  model: string;
}

export interface RetryResponse {
  messages: Record<number, Message>;
}

export interface UpdateRequest {
  conversationId: string;
  messageId: number;
  content: string;
}

export interface UpdateResponse {
  messages: Record<number, Message>;
}

// Streaming types
export interface StreamMetadata {
  conversationId: string;
  userMessageId: number;
}

export interface StreamChunk {
  content?: string;
  reasoning?: string;
}

export interface StreamComplete {
  userMessageId: number;
  assistantMessageId: number;
}

export interface CreateConversationRequest {
  conversation: Conversation;
}

export type CreateConversationResponse = Conversation;

// Frontend types for compatibility
export interface FrontendMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  status?: "success" | "error" | "pending";
  error?: string;
  timestamp: number;
  attachment?: string;
}

export interface FrontendConversation {
  id: string;
  title: string;
  messages: FrontendMessage[];
  backendConversation: Conversation;
}

// Utility function to generate optimistic client-only conversation ID (placeholder)

// Note: Backend now uses UUIDs. When creating a new conversation implicitly,
// read the real UUID from the returned messages' convId.
export const generateOptimisticConversationId = (): string => {
  const now = new Date();

  const date = now.toISOString().split("T")[0].replace(/-/g, "");

  const time = now.toTimeString().split(" ")[0].replace(/:/g, "");

  return `conv-${date}-${time}`;
};

// Convert backend message to frontend message
export const backendToFrontendMessage = (
  backendMsg: Message,
  status: "success" | "error" | "pending" = "success",
  error?: string,
): FrontendMessage => {
  // Safety checks for null/undefined message
  if (!backendMsg || typeof backendMsg !== "object") {
    console.error("Invalid backend message provided:", backendMsg);
    return {
      id: "error",
      role: "assistant",
      content: "Error: Invalid message data",
      status: "error",
      error: "Invalid message data",
      timestamp: Date.now(),
    };
  }

  // Validate required fields
  if (!backendMsg.role) {
    console.error("Backend message missing valid role:", backendMsg);
  }

  return {
    id: (backendMsg.id || "unknown").toString(),
    role:
      backendMsg.role === "user" || backendMsg.role === "assistant"
        ? backendMsg.role
        : "assistant",
    content: backendMsg.content || "",
    status,
    error,
    timestamp: Date.now(), // Backend doesn't provide timestamp, use current time
    attachment: backendMsg.attachment,
  };
};

// Provider API Types
export interface ProviderRequest {
  base_url: string;
  api_key: string;
}

export interface ProviderResponse {
  id: string;
  base_url: string;
}

export interface Model {
  id: string; // provider id + name (for quick finding) e.g: provider-123/meta/llama-3b

  name: string; // original name from provider

  provider: string; // provider id

  is_enabled: boolean; // whether the model is enabled (shown/usable)
}

export interface ModelsResponse {
  models: Model[];
}

// Settings API Types
export interface Settings {
  settings: Record<string, string>;
}

// Frontend types for providers
export interface FrontendProvider {
  id: string;
  name: string;
  baseUrl: string;
  apiKey?: string; // Optional for display purposes
  models?: Model[];
}

// File upload types
export interface FileUploadResponse {
  fileUrl: string;
}
