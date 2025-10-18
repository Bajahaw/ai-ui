import {backendToFrontendMessage, Conversation, FrontendMessage, Message,} from "@/lib/api";

let tempIdCounter = -1;

export interface ClientConversation {
  id: string;

  title: string;

  messages: FrontendMessage[];

  backendConversation?: Conversation;

  pendingMessageIds: Set<string>;

  activeBranches: Map<number, number>; // messageId -> activeChildId for messages with multiple children

  createdAt?: string;
  updatedAt?: string;
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

    const nowIso = new Date().toISOString();

    const conversation: ClientConversation = {
      id: conversationId,
      title:
        firstMessage.length > 50
          ? firstMessage.substring(0, 47) + "..."
          : firstMessage,
      messages: [userMessage, assistantPlaceholder],
      pendingMessageIds: new Set([tempMessageId, assistantPlaceholderId]),
      activeBranches: new Map(),
      // Track timestamps on the client conversation so sorting works immediately
      createdAt: nowIso,
      updatedAt: nowIso,
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

    // Update timestamps so sorting reflects recent activity immediately
    const nowIso = new Date().toISOString();
    (conversation as any).updatedAt = nowIso;
    if (!(conversation as any).createdAt) {
      (conversation as any).createdAt = nowIso;
    }

    return tempMessageId;
  }

  confirmConversationCreated(
    conversationId: string,
    backendConversation: Conversation,
  ): void {
    const conversation = this.conversations.get(conversationId);
    if (!conversation) return;

    // Re-key the conversation if the backend ID is different
    if (backendConversation.id && conversation.id !== backendConversation.id) {
      this.conversations.delete(conversationId);
      conversation.id = backendConversation.id;
      this.conversations.set(backendConversation.id, conversation);
    }

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
    
    // Use an empty object fallback to avoid 'undefined' message maps
    this.updateWithBackendMessages(
      backendConversation.id,
      backendConversation.messages || {},
    );

    // Only rebuild messages from backend if backend has messages
    // Otherwise preserve the streaming messages we already have
    if (backendConversation.messages && Object.keys(backendConversation.messages).length > 0) {
      conversation.messages = this.buildMessagesFromBackend(
        conversation.backendConversation,
      );
    }

    // Ensure client-side timestamps reflect the backend values so sorting is consistent
    (conversation as any).createdAt =
      backendConversation.createdAt ||
      (conversation as any).createdAt ||
      new Date().toISOString();
    (conversation as any).updatedAt =
      backendConversation.updatedAt ||
      (conversation as any).updatedAt ||
      new Date().toISOString();
  }

  updateWithChatResponse(
    conversationId: string,
    backendMessages: Record<number, Message>,
    isLazyLoad: boolean = false,
  ): void {
    const conversation = this.conversations.get(conversationId);

    if (!conversation) return;

    // Determine real conversation ID from messages (backend provides convId).
    // Coerce to string when available, otherwise leave undefined so the
    // existing optimistic id remains.
    const foundConvId = Object.values(backendMessages)
      .map((m) => m?.convId)
      .find((id) => typeof id === "string" && id.trim().length > 0);
    const realConvId: string | undefined = foundConvId
      ? String(foundConvId)
      : undefined;

    if (realConvId && conversation.id !== realConvId) {
      // Re-key the conversation map to use the real UUID
      this.conversations.delete(conversationId);
      conversation.id = realConvId;

      if (!conversation.backendConversation) {
        conversation.backendConversation = {
          id: realConvId,
          userId: "",
          title: conversation.title,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
          messages: {},
        } as unknown as Conversation;
      } else {
        conversation.backendConversation.id = realConvId;
      }

      // Ensure client-side createdAt/updatedAt reflect backend conversation timestamps
      (conversation as any).createdAt =
        conversation.backendConversation.createdAt ||
        (conversation as any).createdAt ||
        new Date().toISOString();
      (conversation as any).updatedAt =
        conversation.backendConversation.updatedAt ||
        (conversation as any).updatedAt ||
        new Date().toISOString();

      this.conversations.set(realConvId, conversation);
    }

    // Ensure backendConversation exists
    if (!conversation.backendConversation) {
      conversation.backendConversation = {
        id: realConvId || conversation.id,
        userId: "",
        title: conversation.title,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        messages: {},
      } as unknown as Conversation;
    }

    // Ensure messages map exists (defensive)
    // Ensure messages map exists
    if (!conversation.backendConversation.messages) {
      conversation.backendConversation.messages = {};
    }

    // existingIds removed; we now detect new/edited messages by comparing stored message objects

    // Merge messages into frontend state first
    this.updateWithBackendMessages(conversation.id, backendMessages);

    // Merge into backendConversation.messages while detecting newly added IDs
    // Also treat edited messages (content changed) as an update that should bump updatedAt.
    let newMessageAdded = false;
    for (const [idStr, msg] of Object.entries(backendMessages)) {
      const idNum = Number(idStr);
      const existingMsg = conversation.backendConversation.messages[idNum];

      // If the message ID is completely new, mark as added
      if (!existingMsg) {
        newMessageAdded = true;
      } else {
        // If the message content changed compared to stored copy, treat as an update
        // (server typically updates timestamp on edited messages)
        if (existingMsg.content !== msg.content) {
          newMessageAdded = true;
        }
      }

      // Ensure the backend messages map is updated with the latest message
      conversation.backendConversation.messages[idNum] = msg;
    }

    const messageIds = Object.keys(backendMessages).map(Number);

    if (messageIds.length > 0) {
      // Set active message to the latest assistant message (highest id) if available
      const assistantIds = messageIds.filter(
        (id) => backendMessages[id].role === "assistant",
      );
      const assistantMessageId = assistantIds.length > 0 ? Math.max(...assistantIds) : undefined;
      if (assistantMessageId !== undefined) {
        conversation.backendConversation.activeMessageId = assistantMessageId;

        // Update active branches when new assistant message is added (client-side)

        const assistantMessage = backendMessages[assistantMessageId];

        if (assistantMessage && assistantMessage.parentId) {
          // Update parent's children array if this is a new branch

          const parentMessage =
            conversation.backendConversation.messages?.[
              assistantMessage.parentId
            ];

          if (parentMessage) {
            if (!Array.isArray(parentMessage.children)) {
              parentMessage.children = [];
            }
            if (!parentMessage.children.includes(assistantMessageId)) {
              parentMessage.children.push(assistantMessageId);
            }
          }

          // Set this as the active branch for the parent

          conversation.activeBranches.set(
            assistantMessage.parentId,

            assistantMessageId,
          );
        }
      }
    }

    // Touch updatedAt when new messages are added or when existing message contents changed.
    // Don't update timestamps during lazy loading of existing messages
    if (newMessageAdded && !isLazyLoad) {
      const nowIso = new Date().toISOString();
      conversation.backendConversation.updatedAt = nowIso;
      (conversation as any).updatedAt = nowIso;
    }

    // Rebuild the visible message list to reflect the active branch only
    conversation.messages = this.buildMessagesFromBackend(
      conversation.backendConversation,
    );
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

    // Ensure messages map exists
    if (!conversation.backendConversation.messages) {
      conversation.backendConversation.messages = {};
    }

    // Process all messages from retry response (parent + new assistant)
    for (const [messageId, message] of Object.entries(retryMessages)) {
      const numericId = parseInt(messageId);

      // Update the message in backend conversation
      conversation.backendConversation.messages[numericId] = message;

      // If this is the new assistant message, set it as active

      if (message.role === "assistant" && message.parentId) {
        // Set as active message and active branch (client-side)

        conversation.backendConversation.activeMessageId = numericId;

        conversation.activeBranches.set(message.parentId, numericId);
      }
    }

    // Touch updatedAt
    const nowIso = new Date().toISOString();
    conversation.backendConversation.updatedAt = nowIso;
    (conversation as any).updatedAt = nowIso;

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
      let messageUpdated = false;

      // Try to find and update existing message with same ID first
      const existingMessage = conversation.messages.find(
        (m) => m.id === backendMsg.id.toString(),
      );

      if (existingMessage) {
        // Update existing message

        existingMessage.content = backendMsg.content;
        existingMessage.status = "success";
        existingMessage.attachment = backendMsg.attachment;
        messageUpdated = true;
      } else {
        // Find matching pending message by content and role
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
          pendingMessage.attachment = backendMsg.attachment;
          conversation.pendingMessageIds.delete(oldId);
          messageUpdated = true;
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
            placeholderMessage.attachment = backendMsg.attachment;
            conversation.pendingMessageIds.delete(oldId);
            messageUpdated = true;
          }
        }
      }

      // If no existing message was updated, add new message
      if (!messageUpdated) {
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

    // Clean up any remaining pending messages that should now be successful
    // This handles edge cases where matching failed but messages should be marked as successful
    const remainingPendingMessages = conversation.messages.filter(
      (m) => conversation.pendingMessageIds.has(m.id) && m.status === "pending",
    );

    for (const message of remainingPendingMessages) {
      // Check if this message matches any backend message that we just processed
      const matchingBackendMsg = Object.values(backendMessages).find(
        (backendMsg) =>
          backendMsg.role === message.role &&
          (backendMsg.content === message.content ||
            (message.role === "assistant" &&
              message.content === "" &&
              backendMsg.content !== "")),
      );

      if (matchingBackendMsg) {
        message.status = "success";
        if (message.role === "assistant" && message.content === "") {
          message.content = matchingBackendMsg.content;
          message.id = matchingBackendMsg.id.toString();
        }
        conversation.pendingMessageIds.delete(message.id);
      } else {
        // Fallback: if no exact match found, try to update similar messages
        // This prevents messages from staying in pending state forever
        const backendMsgArray = Object.values(backendMessages);
        const similarBackendMsg = backendMsgArray.find(
          (backendMsg) => backendMsg.role === message.role,
        );

        if (
          similarBackendMsg &&
          message.role === "assistant" &&
          message.content === ""
        ) {
          // For empty assistant placeholders, update with any assistant response

          message.status = "success";
          message.content = similarBackendMsg.content;
          message.id = similarBackendMsg.id.toString();
          conversation.pendingMessageIds.delete(message.id);
        } else if (similarBackendMsg && message.role === "user") {
          // For user messages, if we have a user message in the backend response, mark as success

          message.status = "success";
          conversation.pendingMessageIds.delete(message.id);
        } else {
          console.warn(
            `[ConversationManager] Warning: pending message ${message.id} (${message.role}) could not be matched with backend response`,
            {
              messageContent: message.content.substring(0, 50) + "...",
              backendMessageIds: Object.keys(backendMessages),
              backendRoles: backendMsgArray.map((m) => m.role),
            },
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

  hasLoadedMessages(conversationId: string): boolean {
    const conversation = this.conversations.get(conversationId);
    return !!(
      conversation?.backendConversation?.messages &&
      Object.keys(conversation.backendConversation.messages).length > 0
    );
  }

  getAllConversations(): ClientConversation[] {
    const list = Array.from(this.conversations.values());

    return list.sort((a, b) => {
      const aTime = new Date(
        a.backendConversation?.updatedAt ||
          (a as any).updatedAt ||
          a.backendConversation?.createdAt ||
          (a as any).createdAt ||
          0,
      ).getTime();
      const bTime = new Date(
        b.backendConversation?.updatedAt ||
          (b as any).updatedAt ||
          b.backendConversation?.createdAt ||
          (b as any).createdAt ||
          0,
      ).getTime();
      return bTime - aTime;
    });
  }

  loadBackendConversations(backendConversations: Conversation[]): void {
    for (const backendConv of backendConversations) {
      const existingConv = this.conversations.get(backendConv.id);

      if (existingConv) {
        // Merge minimal backend conversation fields (messages may be fetched separately)
        existingConv.backendConversation = {
          ...(existingConv.backendConversation || {}),
          ...backendConv,
        } as Conversation;
        existingConv.title = backendConv.title || existingConv.title;
        (existingConv as any).createdAt = backendConv.createdAt;
        (existingConv as any).updatedAt = backendConv.updatedAt;
      } else {
        const clientConv: ClientConversation = {
          id: backendConv.id,

          title: backendConv.title || "New Chat",

          messages: [], // messages are loaded via chat responses or messages endpoint
          backendConversation: backendConv,

          pendingMessageIds: new Set(),

          activeBranches: new Map(),

          createdAt: backendConv.createdAt,
          updatedAt: backendConv.updatedAt,
        };
        this.conversations.set(backendConv.id, clientConv);
      }
    }
  }

  private buildMessagesFromBackend(
    backendConv: Conversation,
  ): FrontendMessage[] {
    if (!backendConv.messages) return [];

    const conversation = this.conversations.get(backendConv.id);

    const messages = (backendConv.messages || {}) as Record<number, Message>;
    // Build children relationships on the fly since backend may omit them
    // Extract the object values into a typed array first to avoid implicit 'any'
    const messageList: Message[] = Object.values(messages) as Message[];
    for (const msg of messageList) {
      if (!msg) continue;
      const pid = msg.parentId;
      if (typeof pid === "number" && messages[pid]) {
        const parent = messages[pid] as Message;
        if (!Array.isArray(parent.children)) parent.children = [];
        if (!parent.children.includes(msg.id)) {
          parent.children.push(msg.id);
        }
      }
    }

    // Helper: find roots (messages without a valid parent)
    // A message is considered a root when it has no numeric parentId (undefined/null)
    // or when parentId is explicitly 0. Use explicit type checks instead of truthiness.
    const isRoot = (m: Message): boolean => {
      return typeof m.parentId !== "number" || m.parentId === 0;
    };

    const all: Message[] = Object.values(messages) as Message[];
    if (all.length === 0) return [];

    // Prefer to render only the active path from the root down to the leaf.
    const path: Message[] = [];
    // Determine the starting root of the active path.
    // If an active message is set, walk up to the root of its chain.
    // Otherwise, pick the most recent root.
    let start: Message | undefined;

    if (backendConv.activeMessageId && messages[backendConv.activeMessageId]) {
      let currentId: number | undefined = backendConv.activeMessageId;
      let topMost: Message | undefined;
      while (currentId !== undefined) {
        const msg: Message | undefined = messages[currentId];
        if (!msg) break;
        topMost = msg;
        currentId = msg.parentId;
      }
      start = topMost;
    } else {
      // Start from one of the roots (pick the latest by id)
      const roots = all.filter(isRoot).sort((a, b) => b.id - a.id);
      start = roots[0] || all.sort((a, b) => a.id - b.id)[0];
    }
    if (!start) return [];

    // Walk down following client-side activeBranches when available,
    // otherwise pick the last child (most recent) if multiple
    let current: Message | undefined = start;
    while (current) {
      path.push(current);
      if (!current.children || current.children.length === 0) break;

      let nextChildId: number = current.children[current.children.length - 1];
      if (conversation && conversation.activeBranches.has(current.id)) {
        const activeChildId = conversation.activeBranches.get(current.id)!;
        if (current.children.includes(activeChildId)) {
          nextChildId = activeChildId;
        }
      }

      current = messages[nextChildId];
      if (!current) break;
    }

    return path.map((m) => backendToFrontendMessage(m, "success"));
  }

  hasPendingMessages(conversationId: string): boolean {
    const conversation = this.conversations.get(conversationId);
    return conversation ? conversation.pendingMessageIds.size > 0 : false;
  }

  getActiveMessageId(conversationId: string): number | undefined {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) return undefined;

    // Only return the explicitly tracked activeMessageId.
    // Do NOT guess or fallback here; callers must ensure this is set correctly
    // (e.g., after message send/stream completion or messages load).
    return conversation.backendConversation.activeMessageId;
  }

  // Branch navigation methods
  hasMultipleBranches(conversationId: string, messageId: number): boolean {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) {
      return false;
    }

    const message = conversation.backendConversation.messages?.[messageId];
    return (message?.children?.length ?? 0) > 1;
  }

  getActiveBranchIndex(conversationId: string, messageId: number): number {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) return 0;

    const message = conversation.backendConversation.messages?.[messageId];
    if (!message?.children || message.children.length <= 1) return 0;

    const activeChildId = conversation.activeBranches.get(messageId);
    if (activeChildId === undefined) return 0;

    return message.children.indexOf(activeChildId);
  }

  getBranchCount(conversationId: string, messageId: number): number {
    const conversation = this.conversations.get(conversationId);
    if (!conversation?.backendConversation) return 1;

    const message = conversation.backendConversation.messages?.[messageId];
    return message?.children?.length || 1;
  }

  switchToBranch(
    conversationId: string,

    messageId: number,

    branchIndex: number,
  ): void {
    const conversation = this.conversations.get(conversationId);

    if (!conversation?.backendConversation) return;

    const message = conversation.backendConversation.messages?.[messageId];

    if (!message?.children || branchIndex >= message.children.length) return;

    const newActiveChildId = message.children[branchIndex];

    conversation.activeBranches.set(messageId, newActiveChildId);

    // Active branch is tracked client-side via activeBranches Map

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
        conversation.backendConversation.messages?.[messageId];

      if (assistantMessage && assistantMessage.role === "assistant") {
        return assistantMessage.parentId;
      }
    }

    // Fallback: Find the assistant message by content matching
    const frontendMessage = conversation.messages.find(
      (m) => m.id === assistantMessageId && m.role === "assistant",
    );

    if (!frontendMessage) return undefined;

    // Find matching backend message by content (safely handle undefined messages map)
    const backendMessages = Object.values(
      conversation.backendConversation.messages || {},
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
        conversation.backendConversation.messages?.[
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
      const message: Message | undefined = backendConv.messages?.[currentId];
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

  // Update message content (for streaming)
  updateMessageContent(
    conversationId: string,
    messageId: string,
    newContent: string,
  ): void {
    const conversation = this.conversations.get(conversationId);
    if (!conversation) return;

    const message = conversation.messages.find((m) => m.id === messageId);
    if (message) {
      message.content = newContent;
      message.status = "success";
    }
  }

  // Confirm messages with real IDs from backend (after streaming completes)
  confirmMessages(
    conversationId: string,
    idMapping: Record<string, number>,
  ): void {
    const conversation = this.conversations.get(conversationId);
    if (!conversation) {
      console.warn(`Conversation ${conversationId} not found for confirmMessages`);
      return;
    }

    // Update frontend message IDs
    for (const [tempId, realId] of Object.entries(idMapping)) {
      const message = conversation.messages.find((m) => m.id === tempId);
      if (message) {
        message.id = realId.toString();
        conversation.pendingMessageIds.delete(tempId);
      }
    }

    // Also update backend conversation structure if it exists
    if (conversation.backendConversation?.messages) {
      for (const [tempId, realId] of Object.entries(idMapping)) {
        // If backend messages were added with temp IDs, update them
        const tempIdNum = Number(tempId);
        if (!isNaN(tempIdNum) && conversation.backendConversation.messages[tempIdNum]) {
          const msg = conversation.backendConversation.messages[tempIdNum];
          delete conversation.backendConversation.messages[tempIdNum];
          conversation.backendConversation.messages[realId] = msg;
          
          // Update the message's own id field
          if (msg) {
            msg.id = realId;
          }
        }
      }
    }
  }

  // Re-key conversation from temporary ID to real backend ID
  rekeyConversation(oldId: string, newId: string): void {
    const conversation = this.conversations.get(oldId);
    if (!conversation) return;

    this.conversations.delete(oldId);
    conversation.id = newId;
    this.conversations.set(newId, conversation);
  }
}
