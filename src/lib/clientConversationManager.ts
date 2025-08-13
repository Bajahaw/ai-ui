import {
  Conversation,
  Message,
  FrontendMessage,
  backendToFrontendMessage,
} from "@/lib/api";

let tempIdCounter = -1;

export interface ClientConversation {
  id: string;
  title: string;
  messages: FrontendMessage[];
  backendConversation?: Conversation;
  pendingMessageIds: Set<string>;
}

export class ClientConversationManager {
  private conversations: Map<string, ClientConversation> = new Map();

  private generateTempId(): string {
    return `temp_${Date.now()}_${Math.abs(tempIdCounter--)}`;
  }

  private generateConversationId(): string {
    const now = new Date();
    const date = now.toISOString().split("T")[0].replace(/-/g, "");
    const time = now.toTimeString().split(" ")[0].replace(/:/g, "");
    return `conv-${date}-${time}`;
  }

  createConversation(firstMessage: string): ClientConversation {
    const conversationId = this.generateConversationId();
    const tempMessageId = this.generateTempId();

    const userMessage: FrontendMessage = {
      id: tempMessageId,
      role: "user",
      content: firstMessage,
      status: "pending",
      timestamp: Date.now(),
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
    };

    this.conversations.set(conversationId, conversation);
    return conversation;
  }

  addMessageOptimistically(conversationId: string, content: string): string {
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
        conversation.backendConversation.activeMessageId = Math.max(
          ...messageIds,
        );
      }
    }
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
        };
        this.conversations.set(backendConv.id, clientConv);
      }
    }
  }

  private buildMessagesFromBackend(
    backendConv: Conversation,
  ): FrontendMessage[] {
    if (!backendConv.messages || !backendConv.activeMessageId) return [];

    const path: Message[] = [];
    let currentId: number | undefined = backendConv.activeMessageId;

    while (currentId !== undefined) {
      const message: Message = backendConv.messages[currentId];
      if (!message) break;
      path.unshift(message);
      currentId = message.parentId;
    }

    return path.map((msg) => backendToFrontendMessage(msg, "success"));
  }

  hasPendingMessages(conversationId: string): boolean {
    const conversation = this.conversations.get(conversationId);
    return conversation ? conversation.pendingMessageIds.size > 0 : false;
  }

  getActiveMessageId(conversationId: string): number | undefined {
    const conversation = this.conversations.get(conversationId);
    return conversation?.backendConversation?.activeMessageId;
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
