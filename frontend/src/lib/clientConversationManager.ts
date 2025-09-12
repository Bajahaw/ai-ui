import {
  backendToFrontendMessage,
  Conversation,
  FrontendMessage,
  Message,
} from "@/lib/api";

let tempIdCounter = -1;

export interface ClientConversation {
  id: string;
  title: string;
  messages: FrontendMessage[];
  backendConversation?: Conversation;
  pendingMessageIds: Set<string>;
  activeBranches: Map<number, number>; // messageId -> activeChildId for messages with multiple children
}

export class ClientConversationManager {
  private conversations: Map<string, ClientConversation> = new Map();

  generateTempId(): string {
    return `temp_${Date.now()}_${Math.abs(tempIdCounter--)}`;
  }

  private generateConversationId(): string {
    const now = new Date();
    const date = now.toISOString().split("T")[0].replace(/-/g, "");
    const time = now.toTimeString().split(" ")[0].replace(/:/g, "");
    return `conv-${date}-${time}`;
  }

  createConversation(
    firstMessage: string,
    attachment?: string,
  ): ClientConversation {
    const conversationId = this.generateConversationId();
    const tempMessageId = this.generateTempId();

    const userMessage: FrontendMessage = {
      id: tempMessageId,
      role: "user",
      content: firstMessage,
      status: "pending",
      timestamp: Date.now(),
      attachment,
    };

    // Add placeholder assistant message immediately
    const assistantPlaceholderId = this.generateTempId();
    const assistantPlaceholder: FrontendMessage = {
      id: assistantPlaceholderId,
      role: "assistant",
      content: "",
      status: "pending",
      timestamp: Date.now(),
    };

    const conversation: ClientConversation = {
      id: conversationId,
      title:
        firstMessage.length > 50
          ? firstMessage.substring(0, 47) + "..."
          : firstMessage,
      messages: [userMessage, assistantPlaceholder],
      pendingMessageIds: new Set([tempMessageId, assistantPlaceholderId]),
      activeBranches: new Map(),
    };

    this.conversations.set(conversationId, conversation);
    return conversation;
  }

  addMessageOptimistically(
    conversationId: string,
    content: string,
    attachment?: string,
  ): string {
    const conversation = this.conversations.get(conversationId);
    if (!conversation)
      throw new Error(`Conversation ${conversationId} not found`);

    const tempMessageId = this.generateTempId();
    const userMessage: FrontendMessage = {
      id: tempMessageId,
      role: "user",
      content,
      status: "pending",
      timestamp: Date.now(),
      attachment,
    };

    // Add placeholder assistant message immediately
    const assistantPlaceholderId = this.generateTempId();
    const assistantPlaceholder: FrontendMessage = {
      id: assistantPlaceholderId,
      role: "assistant",
      content: "",
      status: "pending",
      timestamp: Date.now(),
    };

    conversation.messages.push(userMessage, assistantPlaceholder);
    conversation.pendingMessageIds.add(tempMessageId);
    conversation.pendingMessageIds.add(assistantPlaceholderId);
    return tempMessageId;
  }

  confirmConversationCreated(
    conversationId: string,
    backendConversation: Conversation,
  ): void {
    const conversation = this.conversations.get(conversationId);
    if (!conversation) return;

    conversation.backendConversation = backendConversation;
    // Initialize activeBranches from backend data
    if (backendConversation.activeBranches) {
      conversation.activeBranches = new Map(
        Object.entries(backendConversation.activeBranches).map(([k, v]) => [
          parseInt(k),
          v,
        ]),
      );
    } else {
      conversation.activeBranches = new Map();
    }
    this.updateWithBackendMessages(
      conversationId,
      backendConversation.messages,
    );
  }

  updateWithChatResponse(
    conversationId: string,
    backendMessages: Record<number, Message>,
  ): void {
    const conversation = this.conversations.get(conversationId);
    if (!conversation) return;

    this.updateWithBackendMessages(conversationId, backendMessages);

    if (conversation.backendConversation) {
      conversation.backendConversation.messages = {
        ...conversation.backendConversation.messages,
        ...backendMessages,
      };

      const messageIds = Object.keys(backendMessages).map(Number);
      if (messageIds.length > 0) {
        // Set active message to the last assistant message (response)
        const assistantMessageId = messageIds.find(
          (id) => backendMessages[id].role === "assistant",
        );
        if (assistantMessageId) {
          conversation.backendConversation.activeMessageId = assistantMessageId;

          // Update active branches when new assistant message is added
          const assistantMessage = backendMessages[assistantMessageId];
          if (assistantMessage && assistantMessage.parentId) {
            // Update parent's children array if this is a new branch
            const parentMessage =
              conversation.backendConversation.messages[
                assistantMessage.parentId
              ];
            if (
              parentMessage &&
              !parentMessage.children.includes(assistantMessageId)
            ) {
              parentMessage.children.push(assistantMessageId);
            }

            // Set this as the active branch for the parent
            conversation.activeBranches.set(
              assistantMessage.parentId,
              assistantMessageId,
            );
            if (!conversation.backendConversation.activeBranches) {
              conversation.backendConversation.activeBranches = {};
            }
            conversation.backendConversation.activeBranches[
              assistantMessage.parentId
            ] = assistantMessageId;
          }
        } else {
          conversation.backendConversation.activeMessageId = Math.max(
            ...messageIds,
          );
        }
      }
    }
  }

  /**
   * Handle retry response which contains both the parent message (with updated children)
   * and the new assistant message. This ensures proper conversation tree state.
   */
  updateWithRetryResponse(
    conversationId: string,
    retryMessages: Record<number, Message>,
  ): void {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) return;

    // Process all messages from retry response (parent + new assistant)
    for (const [messageId, message] of Object.entries(retryMessages)) {
      const numericId = parseInt(messageId);

      // Update the message in backend conversation
      conversation.backendConversation.messages[numericId] = message;

      // If this is the new assistant message, set it as active
      if (message.role === "assistant" && message.parentId) {
        // Set as active message and active branch
        conversation.backendConversation.activeMessageId = numericId;
        conversation.activeBranches.set(message.parentId, numericId);

        if (!conversation.backendConversation.activeBranches) {
          conversation.backendConversation.activeBranches = {};
        }
        conversation.backendConversation.activeBranches[message.parentId] =
          numericId;
      }
    }

    // Update frontend messages to reflect the changes
    this.updateWithBackendMessages(conversationId, retryMessages);

    // Rebuild the conversation view to ensure proper branch display
    conversation.messages = this.buildMessagesFromBackend(
      conversation.backendConversation,
    );
  }

  private updateWithBackendMessages(
    conversationId: string,
    backendMessages: Record<number, Message>,
  ): void {
    const conversation = this.conversations.get(conversationId);
    if (!conversation) return;

    for (const backendMsg of Object.values(backendMessages)) {
      // Find matching pending message
      const pendingMessage = conversation.messages.find(
        (m) =>
          conversation.pendingMessageIds.has(m.id) &&
          m.content === backendMsg.content &&
          m.role === backendMsg.role,
      );

      if (pendingMessage) {
        // Update pending message with real ID
        const oldId = pendingMessage.id;
        pendingMessage.id = backendMsg.id.toString();
        pendingMessage.status = "success";
        conversation.pendingMessageIds.delete(oldId);
      } else if (backendMsg.role === "assistant") {
        // Find pending assistant placeholder to replace
        const placeholderMessage = conversation.messages.find(
          (m) =>
            conversation.pendingMessageIds.has(m.id) &&
            m.role === "assistant" &&
            m.content === "" &&
            m.status === "pending",
        );

        if (placeholderMessage) {
          // Replace placeholder with actual response
          const oldId = placeholderMessage.id;
          placeholderMessage.id = backendMsg.id.toString();
          placeholderMessage.content = backendMsg.content;
          placeholderMessage.status = "success";
          conversation.pendingMessageIds.delete(oldId);
        } else {
          // Add new message if no placeholder found
          const exists = conversation.messages.some(
            (m) => m.id === backendMsg.id.toString(),
          );
          if (!exists) {
            conversation.messages.push(
              backendToFrontendMessage(backendMsg, "success"),
            );
          }
        }
      } else {
        // Add new message (non-assistant)
        const exists = conversation.messages.some(
          (m) => m.id === backendMsg.id.toString(),
        );
        if (!exists) {
          conversation.messages.push(
            backendToFrontendMessage(backendMsg, "success"),
          );
        }
      }
    }
  }

  markAssistantFailed(
    conversationId: string,
    userMessageId: string,
    error: string,
  ): void {
    const conversation = this.conversations.get(conversationId);
    if (!conversation) return;

    // Mark user message as successful (it was successfully rendered)
    const userMessage = conversation.messages.find(
      (m) => m.id === userMessageId,
    );
    if (userMessage && userMessage.role === "user") {
      userMessage.status = "success";
      conversation.pendingMessageIds.delete(userMessageId);
    }

    // Mark any pending assistant placeholder as failed
    const assistantPlaceholder = conversation.messages.find(
      (m) =>
        conversation.pendingMessageIds.has(m.id) &&
        m.role === "assistant" &&
        m.content === "" &&
        m.status === "pending",
    );

    if (assistantPlaceholder) {
      assistantPlaceholder.status = "error";
      assistantPlaceholder.error = error;
      assistantPlaceholder.content = "Failed to get response";
      conversation.pendingMessageIds.delete(assistantPlaceholder.id);
    }
  }

  getConversation(id: string): ClientConversation | undefined {
    return this.conversations.get(id);
  }

  getAllConversations(): ClientConversation[] {
    return Array.from(this.conversations.values());
  }

  loadBackendConversations(backendConversations: Conversation[]): void {
    for (const backendConv of backendConversations) {
      const existingConv = this.conversations.get(backendConv.id);

      if (existingConv) {
        existingConv.backendConversation = backendConv;
        if (existingConv.messages.length === 0) {
          existingConv.messages = this.buildMessagesFromBackend(backendConv);
        }
      } else {
        const firstMsg = backendConv.root?.[0]
          ? backendConv.messages[backendConv.root[0]]?.content
          : "New Chat";
        const clientConv: ClientConversation = {
          id: backendConv.id,
          title:
            backendConv.title ||
            (firstMsg?.length > 50
              ? firstMsg.substring(0, 47) + "..."
              : firstMsg || "New Chat"),
          messages: this.buildMessagesFromBackend(backendConv),
          backendConversation: backendConv,
          pendingMessageIds: new Set(),
          activeBranches: new Map(
            backendConv.activeBranches
              ? Object.entries(backendConv.activeBranches).map(([k, v]) => [
                  parseInt(k),
                  v,
                ])
              : [],
          ),
        };
        this.conversations.set(backendConv.id, clientConv);
      }
    }
  }

  private buildMessagesFromBackend(
    backendConv: Conversation,
  ): FrontendMessage[] {
    if (!backendConv.messages) return [];

    // Build the conversation path by following parent-child relationships
    // and respecting active branch selections
    const path: Message[] = [];
    const conversation = this.conversations.get(backendConv.id);

    // If we have an activeMessageId, trace back to build the path
    if (
      backendConv.activeMessageId &&
      backendConv.messages[backendConv.activeMessageId]
    ) {
      let currentId: number | undefined = backendConv.activeMessageId;

      while (currentId !== undefined) {
        const message: Message = backendConv.messages[currentId];
        if (!message) break;
        path.unshift(message);
        currentId = message.parentId;
      }
    } else if (backendConv.root && backendConv.root.length > 0) {
      // Build from root following active branches
      const visited = new Set<number>();
      const buildPath = (messageId: number) => {
        if (visited.has(messageId)) return;
        visited.add(messageId);

        const message = backendConv.messages[messageId];
        if (!message) return;

        path.push(message);

        // Follow active branch if message has multiple children
        if (message.children && message.children.length > 0) {
          let nextChildId = message.children[0]; // Default to first child

          // Check if there's an active branch selection for this message
          if (conversation?.activeBranches.has(messageId)) {
            const activeChildId = conversation.activeBranches.get(messageId);
            if (activeChildId && message.children.includes(activeChildId)) {
              nextChildId = activeChildId;
            }
          } else if (
            backendConv.activeBranches &&
            backendConv.activeBranches[messageId]
          ) {
            const activeChildId = backendConv.activeBranches[messageId];
            if (message.children.includes(activeChildId)) {
              nextChildId = activeChildId;
            }
          }

          buildPath(nextChildId);
        }
      };

      buildPath(backendConv.root[0]);
    }

    return path.map((msg) => backendToFrontendMessage(msg, "success"));
  }

  hasPendingMessages(conversationId: string): boolean {
    const conversation = this.conversations.get(conversationId);
    return conversation ? conversation.pendingMessageIds.size > 0 : false;
  }

  getActiveMessageId(conversationId: string): number | undefined {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) return undefined;

    // Return the stored activeMessageId if it exists
    if (conversation.backendConversation.activeMessageId) {
      return conversation.backendConversation.activeMessageId;
    }

    // Fallback: find the latest assistant message
    const assistantMessages = Object.values(
      conversation.backendConversation.messages,
    ).filter((msg) => msg.role === "assistant");

    if (assistantMessages.length > 0) {
      const latestAssistant = assistantMessages.reduce((latest, current) =>
        current.id > latest.id ? current : latest,
      );
      // Update the activeMessageId with the fallback
      conversation.backendConversation.activeMessageId = latestAssistant.id;
      return latestAssistant.id;
    }

    return undefined;
  }

  // Branch navigation methods
  hasMultipleBranches(conversationId: string, messageId: number): boolean {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) {
      return false;
    }

    const message = conversation.backendConversation.messages[messageId];
    return message?.children && message.children.length > 1;
  }

  getActiveBranchIndex(conversationId: string, messageId: number): number {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) return 0;

    const message = conversation.backendConversation.messages[messageId];
    if (!message?.children || message.children.length <= 1) return 0;

    const activeChildId = conversation.activeBranches.get(messageId);
    if (activeChildId === undefined) return 0;

    return message.children.indexOf(activeChildId);
  }

  getBranchCount(conversationId: string, messageId: number): number {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) return 1;

    const message = conversation.backendConversation.messages[messageId];
    return message?.children?.length || 1;
  }

  switchToBranch(
    conversationId: string,
    messageId: number,
    branchIndex: number,
  ): void {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) return;

    const message = conversation.backendConversation.messages[messageId];
    if (!message?.children || branchIndex >= message.children.length) return;

    const newActiveChildId = message.children[branchIndex];
    conversation.activeBranches.set(messageId, newActiveChildId);

    // Update backend conversation
    if (conversation.backendConversation.activeBranches) {
      conversation.backendConversation.activeBranches[messageId] =
        newActiveChildId;
    } else {
      conversation.backendConversation.activeBranches = {
        [messageId]: newActiveChildId,
      };
    }

    // Update activeMessageId to the new branch if it's the current active path
    if (
      conversation.backendConversation.activeMessageId === messageId ||
      this.isMessageInActivePath(conversation.backendConversation, messageId)
    ) {
      conversation.backendConversation.activeMessageId = newActiveChildId;
    }

    // Always rebuild messages to reflect the new active branch selection
    // This ensures the conversation view updates to show the selected branch
    conversation.messages = this.buildMessagesFromBackend(
      conversation.backendConversation,
    );
  }

  // Get the message that should be used as parent for retry (the user message before the assistant response)
  getRetryParentId(
    conversationId: string,
    assistantMessageId: string,
  ): number | undefined {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) return undefined;

    // First try to find the message by parsing the ID
    const messageId = parseInt(assistantMessageId);
    if (!isNaN(messageId)) {
      const assistantMessage =
        conversation.backendConversation.messages[messageId];

      if (assistantMessage && assistantMessage.role === "assistant") {
        return assistantMessage.parentId;
      }
    }

    // Fallback: Find the assistant message by content matching
    const frontendMessage = conversation.messages.find(
      (m) => m.id === assistantMessageId && m.role === "assistant",
    );

    if (!frontendMessage) return undefined;

    // Find matching backend message by content
    const backendMessages = Object.values(
      conversation.backendConversation.messages,
    );
    const matchingBackendMessage = backendMessages.find(
      (msg) =>
        msg.role === "assistant" && msg.content === frontendMessage.content,
    );

    if (matchingBackendMessage) {
      return matchingBackendMessage.parentId;
    }

    // Last resort: Use the current activeMessageId if it's an assistant message
    if (conversation.backendConversation.activeMessageId) {
      const activeMessage =
        conversation.backendConversation.messages[
          conversation.backendConversation.activeMessageId
        ];
      if (activeMessage && activeMessage.role === "assistant") {
        return activeMessage.parentId;
      }
    }

    return undefined;
  }

  // Helper method to check if a message is in the currently active conversation path
  private isMessageInActivePath(
    backendConv: Conversation,
    messageId: number,
  ): boolean {
    if (!backendConv.activeMessageId) return false;

    let currentId: number | undefined = backendConv.activeMessageId;
    while (currentId !== undefined) {
      if (currentId === messageId) return true;
      const message: Message = backendConv.messages[currentId];
      if (!message) break;
      currentId = message.parentId;
    }
    return false;
  }

  removeConversation(conversationId: string): void {
    this.conversations.delete(conversationId);
  }

  updateConversationTitle(conversationId: string, newTitle: string): void {
    const conversation = this.conversations.get(conversationId);
    if (conversation) {
      conversation.title = newTitle;
      if (conversation.backendConversation) {
        conversation.backendConversation.title = newTitle;
      }
    }
  }
}
