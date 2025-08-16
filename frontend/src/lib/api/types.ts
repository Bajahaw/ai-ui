// Backend API Types matching Go structures

export interface Message {
  id: number;
  role: string;
  content: string;
  parentId?: number;
  children: number[];
}

export interface Conversation {
  id: string;
  title?: string;
  messages: Record<number, Message>;
  root: number[];
  activeMessageId: number;
}

export interface ChatRequest {
  conversationId: string;
  activeMessageId: number;
  model: string;
  content: string;
  webSearch?: boolean;
}

export interface ChatResponse {
  messages: Record<number, Message>;
}

export interface CreateConversationRequest {
  conversation: Conversation;
}

export interface CreateConversationResponse {
  id: string;
}

// Frontend types for compatibility
export interface FrontendMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  status?: "success" | "error" | "pending";
  error?: string;
  timestamp: number;
}

export interface FrontendConversation {
  id: string;
  title: string;
  messages: FrontendMessage[];
  backendConversation: Conversation;
}

// Utility function to generate conversation ID
export const generateConversationId = (): string => {
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
  };
};

// Convert frontend message to backend message
export const frontendToBackendMessage = (
  frontendMsg: FrontendMessage,
  numericId: number,
  parentId?: number,
): Message => {
  // Safety checks for null/undefined message
  if (!frontendMsg || typeof frontendMsg !== "object") {
    console.error("Invalid frontend message provided:", frontendMsg);
    throw new Error("Invalid frontend message data");
  }

  // Validate required fields
  if (!frontendMsg.role) {
    console.error("Frontend message missing valid role:", frontendMsg);
    throw new Error("Frontend message missing valid role");
  }

  if (numericId < 0) {
    console.error("Invalid numeric ID provided:", numericId);
    throw new Error("Invalid numeric ID for backend message");
  }

  return {
    id: numericId,
    role: frontendMsg.role,
    content: frontendMsg.content,
    parentId: parentId !== undefined ? parentId : undefined,
    children: [],
  };
};
