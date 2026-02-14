import { backendToFrontendMessage, Conversation, FrontendMessage, Message, ToolCall } from "@/lib/api";

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
		attachments?: import("@/lib/api/types").Attachment[],
	): ClientConversation {
		const conversationId = this.generateConversationId();
		const tempMessageId = this.generateTempId();

		const userMessage: FrontendMessage = {
			id: tempMessageId,
			role: "user",
			content: firstMessage,
			status: "pending",
			timestamp: Date.now(),
			attachments,
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

		const title =
			firstMessage.length > 60
				? firstMessage.substring(0, 60) + "..."
				: firstMessage;

		const conversation: ClientConversation = {
			id: conversationId,
			title,
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
		attachments?: import("@/lib/api/types").Attachment[],
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
			attachments,
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

		// Do not mutate backend timestamps client-side; rely on backend updates when available

		return tempMessageId;
	}

	updateWithChatResponse(
		conversationId: string,
		backendMessages: Record<number, Message>,
	): void {
		const conversation = this.conversations.get(conversationId);

		if (!conversation) return;

		// Determine real conversation ID from messages (backend provides convId)
		const realConvId = Object.values(backendMessages)
			.map((m) => m?.convId)
			.find((id) => id.trim().length > 0);

		if (realConvId && conversation.id !== realConvId) {
			// Re-key the conversation map to use the real UUID
			this.conversations.delete(conversationId);
			conversation.id = realConvId;

			if (conversation.backendConversation) {
				conversation.backendConversation.id = realConvId;
			}

			this.conversations.set(realConvId, conversation);
		}

		// Ensure backendConversation exists (minimal shape); don't set or mutate timestamps client-side
		if (!conversation.backendConversation) {
			conversation.backendConversation = {
				id: realConvId || conversation.id,
				userId: "",
				title: conversation.title,
				messages: {}, // Ensure messages object is initialized
			} as unknown as Conversation;
		}

		// Merge messages into frontend state first
		this.updateWithBackendMessages(conversation.id, backendMessages);

		// Merge into backendConversation.messages
		for (const [idStr, msg] of Object.entries(backendMessages)) {
			const idNum = Number(idStr);
			conversation.backendConversation.messages[idNum] = msg;

			// Ensure children array is sorted by ID so newest branches are always last
			if (msg.children && msg.children.length > 1) {
				msg.children.sort((a, b) => a - b);
			}
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

				// Set this as the active branch for the parent (backend already provides children relationships)
				const assistantMessage = backendMessages[assistantMessageId];

				if (assistantMessage && assistantMessage.parentId) {
					conversation.activeBranches.set(
						assistantMessage.parentId,
						assistantMessageId,
					);
				}
			}
		}

		// Preserve only pending/error messages that don't have real backend IDs yet
		// Messages with real IDs (numeric) are already in backend and will be included via buildMessagesFromBackend
		const pendingAndErrorMessages = conversation.messages.filter(
			(m) =>
				(m.status === "pending" || !!m.error) &&
				isNaN(parseInt(m.id)) // Only preserve temp IDs (temp_xxx format)
		);

		// Rebuild the visible message list to reflect the active branch only
		conversation.messages = this.buildMessagesFromBackend(
			conversation.backendConversation,
		);

		// Add back pending and error messages that haven't been saved to backend yet
		for (const msg of pendingAndErrorMessages) {
			const exists = conversation.messages.some(m => m.id === msg.id);
			if (!exists) {
				conversation.messages.push(msg);
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
			let messageUpdated = false;

			// Try to find and update existing message with same ID first
			const existingMessage = conversation.messages.find(
				(m) => m.id === backendMsg.id.toString(),
			);

			if (existingMessage) {
				// Update existing message
				existingMessage.content = backendMsg.content;
				existingMessage.status = "completed";
			existingMessage.attachments = backendMsg.attachments;
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
					pendingMessage.status = "completed";
			pendingMessage.attachments = backendMsg.attachments;
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
						placeholderMessage.status = "completed";
						placeholderMessage.error = backendMsg.error;
					placeholderMessage.attachments = backendMsg.attachments;
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
						backendToFrontendMessage(backendMsg),
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
				message.status = "completed";
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
					message.status = "completed";
					message.content = similarBackendMsg.content;
					message.id = similarBackendMsg.id.toString();
					conversation.pendingMessageIds.delete(message.id);
				} else if (similarBackendMsg && message.role === "user") {
					// For user messages, if we have a user message in the backend response, mark as success
					message.status = "completed";
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
		assistantMessageId: string,
		error: string,
	): void {
		const conversation = this.conversations.get(conversationId);
		if (!conversation) return;

		// Find the assistant message by ID
		const assistantMessage = conversation.messages.find(
			(m) => m.id === assistantMessageId && m.role === "assistant",
		);

		if (assistantMessage) {
			// Mark as error - this message will be preserved even when rebuilding from backend
			assistantMessage.status = "completed";
			assistantMessage.error = error;
			conversation.pendingMessageIds.delete(assistantMessageId);
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
			// Backend always provides updatedAt, sort by that only
			const aTime = a.backendConversation?.updatedAt ? new Date(a.backendConversation.updatedAt).getTime() : 0;
			const bTime = b.backendConversation?.updatedAt ? new Date(b.backendConversation.updatedAt).getTime() : 0;

			return bTime - aTime;
		});
	}

	loadBackendConversations(backendConversations: Conversation[]): void {
		for (const backendConv of backendConversations) {
			const existingConv = this.conversations.get(backendConv.id);

			// Ensure messages object is always initialized
			const normalizedBackendConv = {
				...backendConv,
				messages: backendConv.messages || {},
			};

			// Sort children arrays by ID so newest branches are always last
			for (const msg of Object.values(normalizedBackendConv.messages)) {
				if (msg.children && msg.children.length > 1) {
					msg.children.sort((a, b) => a - b);
				}
			}

			if (existingConv) {
				// Merge minimal backend conversation fields (messages may be fetched separately)
				existingConv.backendConversation = {
					...(existingConv.backendConversation || {}),
					...normalizedBackendConv,
				} as Conversation;
				existingConv.title = normalizedBackendConv.title || existingConv.title;
			} else {
				const clientConv: ClientConversation = {
					id: normalizedBackendConv.id,

					title: normalizedBackendConv.title || "New Chat",

					messages: [], // messages are loaded via chat responses or messages endpoint
					backendConversation: normalizedBackendConv,

					pendingMessageIds: new Set(),

					activeBranches: new Map(),
				};
				this.conversations.set(normalizedBackendConv.id, clientConv);
			}
		}
	}

	private buildMessagesFromBackend(
		backendConv: Conversation,
	): FrontendMessage[] {
		const conversation = this.conversations.get(backendConv.id);

		const messages = backendConv.messages;

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

		return path.map((m) => backendToFrontendMessage(m));
	}

	hasPendingMessages(conversationId: string): boolean {
		const conversation = this.conversations.get(conversationId);
		return conversation ? conversation.pendingMessageIds.size > 0 : false;
	}

	getActiveMessageId(conversationId: string): number | undefined {
		const conversation = this.conversations.get(conversationId);
		if (!conversation?.backendConversation) return undefined;
		return conversation.backendConversation.activeMessageId;
	}

	/**
	 * Sets the active branch for a parent message and rebuilds the messages array
	 */
	setActiveBranch(conversationId: string, parentId: number, childId: number): void {
		const conversation = this.conversations.get(conversationId);
		if (!conversation?.backendConversation) return;

		conversation.activeBranches.set(parentId, childId);
		conversation.backendConversation.activeMessageId = childId;

		// Rebuild messages to reflect the new active branch
		conversation.messages = this.buildMessagesFromBackend(
			conversation.backendConversation,
		);
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
		// Preserve only pending messages that don't have real backend IDs yet
		const pendingAndErrorMessages = conversation.messages.filter(
			(m) => m.status === "pending"
		);

		conversation.messages = this.buildMessagesFromBackend(
			conversation.backendConversation,
		);

		// Add back pending messages that haven't been saved to backend yet
		for (const msg of pendingAndErrorMessages) {
			const exists = conversation.messages.some(m => m.id === msg.id);
			if (!exists) {
				conversation.messages.push(msg);
			}
		}
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
			const message: Message | undefined = backendConv.messages[currentId];
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
			// While streaming, keep the message marked as pending so UI
			// continues to show the spinner and hides action buttons.
			// Only mark as completed when the stream fully finishes.
			message.status = "pending";
		}
	}

	// Add or update tool call to message (for streaming)
	addToolCall(
		conversationId: string,
		messageId: string,
		toolCall: ToolCall,
	): void {
		const conversation = this.conversations.get(conversationId);
		if (!conversation) return;

		const message = conversation.messages.find((m) => m.id === messageId);
		if (message) {
			if (!message.toolCalls) {
				message.toolCalls = [];
			}

			// Check if a tool call with the same ID already exists
			const existingIndex = message.toolCalls.findIndex((tc) => tc.id === toolCall.id);

			if (existingIndex !== -1) {
				// Update existing tool call (merge properties to preserve any existing data)
				message.toolCalls[existingIndex] = {
					...message.toolCalls[existingIndex],
					...toolCall,
				};
			} else {
				// Add new tool call
				message.toolCalls.push(toolCall);
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

	handleExternalCreate(conversation: Conversation): void {
		if (this.conversations.has(conversation.id)) {
			this.handleExternalUpdate(conversation);
			return;
		}

		// Create client conversation wrapper
		const clientConv: ClientConversation = {
			id: conversation.id,
			title: conversation.title || "New Conversation",
			messages: [], // Initially empty, user will fetch messages when opening
			backendConversation: conversation,
			pendingMessageIds: new Set(),
			activeBranches: new Map(),
		};
		// Populate initial messages from backend if any are provided (usually unlikely for just metadata)
		if (conversation.messages && Object.keys(conversation.messages).length > 0) {
			clientConv.messages = this.buildMessagesFromBackend(conversation);
		}

		this.conversations.set(conversation.id, clientConv);
	}

	handleExternalUpdate(conversation: Conversation): void {
		const existing = this.conversations.get(conversation.id);
		if (!existing) {
			this.handleExternalCreate(conversation);
			return;
		}

		// Update metadata
		existing.title = conversation.title || existing.title;
		
		// Update backend struct
		if (!existing.backendConversation) {
			existing.backendConversation = conversation;
		} else {
			existing.backendConversation.title = conversation.title;
			existing.backendConversation.updatedAt = conversation.updatedAt;
			// Merge messages if provided? Usually update event might just be title change
			// If we want to sync messages, we'd need more logic, but for now assuming metadata sync primarily
		}
	}

	handleExternalDelete(conversationId: string): void {
		this.conversations.delete(conversationId);
	}
}
