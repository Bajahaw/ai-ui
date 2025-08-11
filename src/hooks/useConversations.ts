import { useState, useCallback } from "react";

// Simple message interface
export interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  status?: "success" | "error" | "pending";
  error?: string;
  timestamp: number;
}

// Simple conversation interface
export interface Conversation {
  id: string;
  title: string;
  messages: Message[];
}

// Simple ID generation
const generateId = () =>
  `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

// Create mock conversations
const createMockConversations = (): Conversation[] => [
  {
    id: "1",
    title: "Sample Chat",
    messages: [
      {
        id: "msg-1",
        role: "user",
        content: "Hello, can you help me?",
        status: "success",
        timestamp: Date.now() - 10000,
      },
      {
        id: "msg-2",
        role: "assistant",
        content:
          "Of course! I'd be happy to help you. What do you need assistance with?",
        status: "success",
        timestamp: Date.now() - 9000,
      },
    ],
  },
  {
    id: "2",
    title: "New Chat",
    messages: [],
  },
];

export const useConversations = () => {
  const [conversations, setConversations] = useState<Conversation[]>(
    createMockConversations(),
  );
  const [activeConversationId, setActiveConversationId] = useState<
    string | null
  >(null);

  // Get current conversation
  const currentConversation = conversations.find(
    (conv) => conv.id === activeConversationId,
  );

  // Create new conversation
  const createConversation = useCallback((title: string) => {
    const conversationId = generateId();
    const newConversation: Conversation = {
      id: conversationId,
      title: title.slice(0, 50) + (title.length > 50 ? "..." : ""),
      messages: [],
    };

    setConversations((prev) => [newConversation, ...prev]);
    setActiveConversationId(conversationId);
    return conversationId;
  }, []);

  // Add message to conversation
  const addMessage = useCallback(
    (
      conversationId: string,
      content: string,
      role: "user" | "assistant",
      status: "success" | "error" = "success",
      error?: string,
    ) => {
      const message: Message = {
        id: generateId(),
        role,
        content,
        status,
        error,
        timestamp: Date.now(),
      };

      setConversations((prev) =>
        prev.map((conv) =>
          conv.id === conversationId
            ? { ...conv, messages: [...conv.messages, message] }
            : conv,
        ),
      );

      return message.id;
    },
    [],
  );

  // Update message
  const updateMessage = useCallback(
    (conversationId: string, messageId: string, updates: Partial<Message>) => {
      setConversations((prev) =>
        prev.map((conv) =>
          conv.id === conversationId
            ? {
                ...conv,
                messages: conv.messages.map((msg) =>
                  msg.id === messageId ? { ...msg, ...updates } : msg,
                ),
              }
            : conv,
        ),
      );
    },
    [],
  );

  // Replace last assistant message (for retry)
  const replaceLastAssistantMessage = useCallback(
    (
      conversationId: string,
      content: string,
      status: "success" | "error" = "success",
      error?: string,
    ) => {
      setConversations((prev) =>
        prev.map((conv) => {
          if (conv.id !== conversationId) return conv;

          const messages = [...conv.messages];
          // Find last assistant message
          for (let i = messages.length - 1; i >= 0; i--) {
            if (messages[i].role === "assistant") {
              messages[i] = {
                ...messages[i],
                content,
                status,
                error,
                timestamp: Date.now(),
              };
              break;
            }
          }

          return { ...conv, messages };
        }),
      );
    },
    [],
  );

  // Get current messages
  const getCurrentMessages = useCallback(
    (conversation: Conversation): Message[] => {
      return conversation.messages;
    },
    [],
  );

  // Select conversation
  const selectConversation = useCallback((conversationId: string) => {
    setActiveConversationId(conversationId);
  }, []);

  // Start new chat
  const startNewChat = useCallback(() => {
    setActiveConversationId(null);
  }, []);

  return {
    // State
    conversations,
    activeConversationId,
    currentConversation,

    // Actions
    createConversation,
    addMessage,
    updateMessage,
    replaceLastAssistantMessage,
    selectConversation,
    startNewChat,
    getCurrentMessages,
  };
};
