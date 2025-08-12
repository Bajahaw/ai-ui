import { useState, useCallback } from "react";
import { BranchingConversation, BranchMessage } from "@/lib/conversation";

// Legacy message interface for compatibility
export interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  status?: "success" | "error" | "pending";
  error?: string;
  timestamp: number;
}

// Updated conversation interface with branching
export interface Conversation {
  id: string;
  title: string;
  branchingConversation: BranchingConversation;
}

// Simple ID generation
const generateId = () =>
  `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

// Create mock conversations
const createMockConversations = (): Conversation[] => [
  {
    id: "1",
    title: "Sample Chat",
    branchingConversation: (() => {
      const conv = new BranchingConversation();
      const msg1Id = conv.addMessage("Hello, can you help me?", "user");
      const msg2Id = conv.addMessage(
        "Of course! I'd be happy to help you. What do you need assistance with?",
        "assistant",
        msg1Id,
      );

      // Create a user follow-up
      const msg3Id = conv.addMessage("Can you tell me a joke?", "user", msg2Id);

      // Create 3 different assistant responses (branches)
      conv.addMessage(
        "Why don't scientists trust atoms? Because they make up everything!",
        "assistant",
        msg3Id,
      );

      conv.addBranchMessage(
        "What do you call a bear with no teeth? A gummy bear!",
        "assistant",
        msg3Id,
      );

      conv.addBranchMessage(
        "Why did the scarecrow win an award? Because he was outstanding in his field!",
        "assistant",
        msg3Id,
      );

      return conv;
    })(),
  },
  {
    id: "2",
    title: "New Chat",
    branchingConversation: new BranchingConversation(),
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
      branchingConversation: new BranchingConversation(),
    };

    // Use functional updates to ensure proper state synchronization
    setConversations((prev) => {
      // Check if conversation already exists (prevent duplicates)
      const existing = prev.find((c) => c.id === conversationId);
      if (existing) {
        console.log("🚫 Conversation already exists:", conversationId);
        return prev;
      }

      const updated = [newConversation, ...prev];
      console.log(
        "📝 Created conversation:",
        conversationId,
        "Total:",
        updated.length,
      );
      return updated;
    });

    // Update activeConversationId after state is set
    setActiveConversationId(conversationId);
    return conversationId;
  }, []);

  // Add message to conversation
  const addMessage = useCallback(
    (
      conversationId: string,
      content: string,
      role: "user" | "assistant",
      status: "success" | "error" | "pending" = "success",
      error?: string,
    ) => {
      let messageId: string | null = null;

      setConversations((prev) => {
        // Check if conversation exists
        const targetConv = prev.find((c) => c.id === conversationId);
        if (!targetConv) {
          console.error("❌ Conversation not found:", conversationId);
          console.log(
            "📊 Available conversations:",
            prev.map((c) => c.id),
          );
          return prev;
        }

        const activePath = targetConv.branchingConversation.getActivePath();

        // Check for duplicate messages (prevent StrictMode duplicates)
        const lastMessage = activePath[activePath.length - 1];
        if (
          lastMessage &&
          lastMessage.role === role &&
          lastMessage.content === content &&
          Date.now() - lastMessage.timestamp < 1000
        ) {
          console.log("🚫 Duplicate message detected, skipping:", {
            role,
            content: content.slice(0, 50),
            timeDiff: Date.now() - lastMessage.timestamp,
          });
          messageId = lastMessage.id;
          return prev;
        }

        const parentId =
          activePath.length > 0
            ? activePath[activePath.length - 1].id
            : undefined;

        messageId = targetConv.branchingConversation.addMessage(
          content,
          role,
          parentId,
          status,
          error,
        );

        console.log("💾 BranchingConversation.addMessage result:", {
          messageId,
          conversationId,
          role,
          content: content.slice(0, 50),
          activePath: activePath.length,
        });

        // Return updated conversations array
        return prev.map((conv) =>
          conv.id === conversationId ? targetConv : conv,
        );
      });

      console.log("🔄 addMessage returning:", messageId);
      return messageId;
    },
    [],
  );

  // Add branching message (alternative response)
  const addBranchMessage = useCallback(
    (
      conversationId: string,
      content: string,
      role: "user" | "assistant",
      parentId: string,
      status: "success" | "error" | "pending" = "success",
      error?: string,
    ) => {
      let messageId: string | null = null;

      setConversations((prev) =>
        prev.map((conv) => {
          if (conv.id === conversationId) {
            messageId = conv.branchingConversation.addBranchMessage(
              content,
              role,
              parentId,
              status,
              error,
            );
          }
          return conv;
        }),
      );

      return messageId;
    },
    [],
  );

  // Update message
  const updateMessage = useCallback(
    (
      conversationId: string,
      messageId: string,
      updates: Partial<BranchMessage>,
    ) => {
      setConversations((prev) =>
        prev.map((conv) => {
          if (conv.id === conversationId) {
            conv.branchingConversation.updateMessage(messageId, updates);
          }
          return conv;
        }),
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

          const activePath = conv.branchingConversation.getActivePath();
          // Find last assistant message in active path
          for (let i = activePath.length - 1; i >= 0; i--) {
            if (activePath[i].role === "assistant") {
              conv.branchingConversation.updateMessage(activePath[i].id, {
                content,
                status,
                error,
                timestamp: Date.now(),
              });
              break;
            }
          }

          return conv;
        }),
      );
    },
    [],
  );

  // Set active message (for branch navigation)
  const setActiveMessage = useCallback(
    (conversationId: string, messageId: string) => {
      setConversations((prev) =>
        prev.map((conv) => {
          if (conv.id === conversationId) {
            conv.branchingConversation.setActive(messageId);
            console.log("🔀 Switched to branch:", {
              conversationId,
              messageId,
              newActivePath: conv.branchingConversation
                .getActivePath()
                .map((m) => ({
                  id: m.id,
                  role: m.role,
                  content: m.content.slice(0, 30),
                })),
            });
          }
          return conv;
        }),
      );
    },
    [],
  );

  // Navigate to next branch
  const goToNextBranch = useCallback(
    (conversationId: string, messageId: string) => {
      setConversations((prev) =>
        prev.map((conv) => {
          if (conv.id === conversationId) {
            conv.branchingConversation.goToNextBranch(messageId);
          }
          return conv;
        }),
      );
    },
    [],
  );

  // Navigate to previous branch
  const goToPreviousBranch = useCallback(
    (conversationId: string, messageId: string) => {
      setConversations((prev) =>
        prev.map((conv) => {
          if (conv.id === conversationId) {
            conv.branchingConversation.goToPreviousBranch(messageId);
          }
          return conv;
        }),
      );
    },
    [],
  );

  // Get current messages (active path) - compatibility with existing interface
  const getCurrentMessages = useCallback(
    (conversation: Conversation): Message[] => {
      const activePath = conversation.branchingConversation.getActivePath();
      return activePath.map(
        (msg: BranchMessage): Message => ({
          id: msg.id,
          role: msg.role,
          content: msg.content,
          status: msg.status,
          error: msg.error,
          timestamp: msg.timestamp,
        }),
      );
    },
    [],
  );

  // Get branching information for a message
  const getBranchInfo = useCallback(
    (conversationId: string, messageId: string) => {
      const conversation = conversations.find(
        (conv) => conv.id === conversationId,
      );
      if (!conversation) return null;

      const branchingConv = conversation.branchingConversation;
      return {
        hasBranches: branchingConv.hasBranches(messageId),
        currentIndex: branchingConv.getCurrentBranchIndex(messageId),
        totalBranches: branchingConv.getTotalBranches(messageId),
        branches: branchingConv.getBranchesAt(messageId),
      };
    },
    [conversations],
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
    addBranchMessage,
    updateMessage,
    replaceLastAssistantMessage,
    setActiveMessage,
    goToNextBranch,
    goToPreviousBranch,
    selectConversation,
    startNewChat,
    getCurrentMessages,
    getBranchInfo,
  };
};
