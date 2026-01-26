import { useCallback, useEffect, useRef, useState } from "react";
import { chatAPI, conversationsAPI, FrontendMessage, ToolCall, StreamStats } from "@/lib/api";
import { ApiErrorHandler } from "@/lib/api/errorHandler";
import { ClientConversation, ClientConversationManager, } from "@/lib/clientConversationManager";
import { useAuth } from "@/hooks/useAuth";

// ============================================================================
// Streaming Utilities - Extracted to reduce duplication
// ============================================================================

/**
 * Manages accumulated streaming state (content, reasoning, RAF scheduling)
 */
class StreamingState {
    content = "";
    reasoning = "";
    reasoningStartTime: number | null = null;
    rafId: number | null = null;

    addContent(chunk: string): void {
        this.content += chunk;
    }

    addReasoning(reasoning: string): void {
        if (!reasoning) return;

        // Track start time on first reasoning chunk
        if (this.reasoningStartTime === null) {
            this.reasoningStartTime = Date.now();
        }

        this.reasoning += reasoning;
    }

    getReasoningDuration(): number | undefined {
        if (this.reasoningStartTime === null || !this.reasoning) {
            return undefined;
        }
        return Math.round((Date.now() - this.reasoningStartTime) / 1000);
    }

    scheduleSync(syncCallback: () => void): void {
        // Cancel previous frame request if any
        if (this.rafId !== null) {
            cancelAnimationFrame(this.rafId);
        }

        // Schedule sync on next animation frame (60fps max)
        this.rafId = requestAnimationFrame(() => {
            syncCallback();
            this.rafId = null;
        });
    }

    cancelPendingSync(): void {
        if (this.rafId !== null) {
            cancelAnimationFrame(this.rafId);
            this.rafId = null;
        }
    }
}

/**
 * Creates streaming callback handlers with shared logic
 */
function createStreamingHandlers(
    manager: ClientConversationManager,
    conversationId: string,
    assistantPlaceholderId: string,
    streamingState: StreamingState,
    syncConversations: () => void,
) {
    const onChunk = (chunk: string) => {
        streamingState.addContent(chunk);

        // Update content immediately (no sync yet)
        manager.updateMessageContent(
            conversationId,
            assistantPlaceholderId,
            streamingState.content,
        );

        streamingState.scheduleSync(syncConversations);
    };

    const onReasoning = (reasoning: string) => {
        streamingState.addReasoning(reasoning);

        // Update reasoning immediately (no sync yet)
        const conv = manager.getConversation(conversationId);
        if (conv) {
            const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
            if (assistMsg) {
                assistMsg.reasoning = streamingState.reasoning;
            }
        }

        streamingState.scheduleSync(syncConversations);
    };

    const onToolCall = (toolCall: ToolCall) => {
        manager.addToolCall(
            conversationId,
            assistantPlaceholderId,
            toolCall,
        );

        // If there's accumulated reasoning and this is the first tool call event (no output yet),
        // append tool usage information to show when the model decided to use the tool
        if (streamingState.reasoning && !toolCall.tool_output) {
            streamingState.reasoning += `  \n\`using tool:${toolCall.name}\`\n  `;
            const conv = manager.getConversation(conversationId);
            if (conv) {
                const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
                if (assistMsg) {
                    assistMsg.reasoning = streamingState.reasoning;
                }
            }
        }

        streamingState.scheduleSync(syncConversations);
    };

    return { onChunk, onReasoning, onToolCall };
}

/**
 * Ensures backend conversation and messages structure exists
 */
function ensureBackendStructure(conv: ClientConversation, conversationId: string): void {
    if (!conv.backendConversation) {
        conv.backendConversation = {
            id: conversationId,
            userId: "",
            title: conv.title,
            messages: {},
        } as any;
    }
}

/**
 * Ensures a parent message exists in backend structure (creates stub if missing)
 */
function ensureBackendParentMessage(
    conv: ClientConversation,
    parentId: number,
    conversationId: string,
): void {
    ensureBackendStructure(conv, conversationId);
    const backendMsgs = conv.backendConversation!.messages!;

    if (!backendMsgs[parentId]) {
        backendMsgs[parentId] = {
            id: parentId,
            convId: conversationId,
            role: "user",
            content: "",
            status: "completed",
            parentId: undefined,
            children: [],
        };
    }
}

/**
 * Adds child to parent's children array (with defensive checks)
 */
function addChildToParent(
    conv: ClientConversation,
    parentId: number,
    childId: number,
): void {
    const parent = conv.backendConversation?.messages[parentId];
    if (parent) {
        if (!Array.isArray(parent.children)) parent.children = [];
        if (!parent.children.includes(childId)) {
            parent.children.push(childId);
        }
    }
}

/**
 * Updates user message with real ID and status after backend confirmation
 */
function updateUserMessageAfterSave(
    manager: ClientConversationManager,
    conversationId: string,
    tempMessageId: string,
    realMessageId: number,
    syncConversations: () => void,
): void {
    const conv = manager.getConversation(conversationId);
    if (!conv) return;

    const userMsg = conv.messages.find(m => m.id === tempMessageId);
    if (!userMsg) return;

    userMsg.id = realMessageId.toString();
    userMsg.status = "completed"; // Message is saved, show actions now!
    conv.pendingMessageIds.delete(tempMessageId);

    ensureBackendStructure(conv, conversationId);
    conv.backendConversation!.messages![realMessageId] = {
        id: realMessageId,
        convId: conversationId,
        role: userMsg.role,
        content: userMsg.content,
        attachments: userMsg.attachments,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
    } as any;

    syncConversations(); // Sync to show action buttons
}

/**
 * Updates assistant message with real ID, content, and status after streaming completes or errors
 * Treats errors the same as successful messages - they get real IDs and are saved to backend structure
 */
function updateAssistantMessageAfterComplete(
    manager: ClientConversationManager,
    conversationId: string,
    assistantPlaceholderId: string,
    realMessageId: number,
    streamingState: StreamingState,
    parentMessageId: number,
    syncConversations: () => void,
    error?: string,
    streamStats?: StreamStats,
    model?: string,
): void {
    streamingState.cancelPendingSync();

    const conv = manager.getConversation(conversationId);
    if (!conv) return;

    const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
    if (!assistMsg) return;

    const reasoningDuration = streamingState.getReasoningDuration();

    // Update frontend message
    assistMsg.id = realMessageId.toString();
    assistMsg.status = "completed";
    assistMsg.content = streamingState.content;
    assistMsg.reasoning = streamingState.reasoning;
    assistMsg.reasoningDuration = reasoningDuration;
    assistMsg.error = error;
    conv.pendingMessageIds.delete(assistantPlaceholderId);

    // Ensure backend structure and add the assistant message
    ensureBackendStructure(conv, conversationId);
    ensureBackendParentMessage(conv, parentMessageId, conversationId);

    conv.backendConversation!.messages![realMessageId] = {
        id: realMessageId,
        convId: conversationId,
        role: assistMsg.role,
        model: model,
        content: assistMsg.content,
        reasoning: assistMsg.reasoning,
        toolCalls: assistMsg.toolCalls,
        parentId: parentMessageId,
        // Persist stream stats metadata if present
        speed: streamStats?.Speed,
        tokenCount: streamStats?.CompletionTokens,
        contextSize: streamStats?.PromptTokens,
        error: error,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
    } as any;

    // Update parent-child relationship
    addChildToParent(conv, parentMessageId, realMessageId);

    // Set as active message
    conv.backendConversation!.activeMessageId = realMessageId;

    // Also persist metadata on the frontend message object for quick access
    if (streamStats) {
        assistMsg.speed = streamStats.Speed;
        assistMsg.tokenCount = streamStats.CompletionTokens;
        assistMsg.contextSize = streamStats.PromptTokens;
    }
    if (model) {
        assistMsg.model = model;
    }

    // Sync immediately to update UI
    syncConversations();
}

export const useConversations = () => {
    const { isAuthenticated } = useAuth();
    const [conversations, setConversations] = useState<ClientConversation[]>([]);
    const [activeConversationId, setActiveConversationId] = useState<
        string | null
    >(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const managerRef = useRef(new ClientConversationManager());
    const manager = managerRef.current;

    const currentConversation = conversations.find(
        (conv) => conv.id === activeConversationId,
    );

    const syncConversations = useCallback(() => {
        setConversations([...manager.getAllConversations()]);
    }, [manager]);

    // Only load conversations when authenticated
    useEffect(() => {
        if (isAuthenticated) {
            loadConversations();
        }
    }, [isAuthenticated]);

    const loadConversations = useCallback(async () => {
        // Skip if not authenticated to prevent 401 errors
        if (!isAuthenticated) {
            return;
        }

        try {
            setIsLoading(true);
            setError(null);

            const backendConversations = await conversationsAPI.fetchConversations();

            if (!backendConversations || !Array.isArray(backendConversations)) {
                console.warn("Backend returned null or invalid conversations data");
                syncConversations();
                return;
            }

            manager.loadBackendConversations(backendConversations);
            syncConversations();
        } catch (err) {
            const errorMessage = ApiErrorHandler.getUserFriendlyMessage(err);
            setError(errorMessage);
            console.error("Failed to load conversations:", err);
            syncConversations();
        } finally {
            setIsLoading(false);
        }
    }, [manager, syncConversations]);

    const getCurrentMessages = useCallback(
        (conversation: ClientConversation): FrontendMessage[] => {
            // Return a new array reference to ensure React detects changes
            // This is needed because messages are mutated in place in updateWithBackendMessages
            return [...(conversation.messages || [])];
        },
        [],
    );

    const selectConversation = useCallback(
        (conversationId: string) => {
            setActiveConversationId(conversationId);

            // Only lazy-load messages if they haven't been loaded yet
            if (!manager.hasLoadedMessages(conversationId)) {
                (async () => {
                    try {
                        const msgs =
                            await conversationsAPI.fetchConversationMessages(conversationId);
                        manager.updateWithChatResponse(conversationId, msgs);
                        // Sync conversations to update UI with loaded messages (but timestamps won't be updated)
                        syncConversations();
                    } catch (err) {
                        console.error("Failed to load conversation messages:", err);
                    }
                })();
            }
        },
        [manager, syncConversations],
    );

    const startNewChat = useCallback(() => {
        setActiveConversationId(null);
    }, []);

    const clearError = useCallback(() => {
        setError(null);
    }, []);

    const hasPendingMessages = useCallback(
        (conversationId?: string) => {
            if (conversationId) {
                return manager.hasPendingMessages(conversationId);
            }
            return manager
                .getAllConversations()
                .some((conv) => manager.hasPendingMessages(conv.id));
        },
        [manager],
    );

    const deleteConversation = useCallback(
        async (conversationId: string): Promise<void> => {
            try {
                setError(null);

                // Optimistically remove from local state
                manager.removeConversation(conversationId);

                // If this was the active conversation, clear it
                if (activeConversationId === conversationId) {
                    setActiveConversationId(null);
                }

                syncConversations();

                // Call backend API
                await conversationsAPI.deleteConversation(conversationId);
            } catch (err) {
                // On error, reload conversations to restore state
                await loadConversations();
                const errorMessage = ApiErrorHandler.getUserFriendlyMessage(err);
                setError(errorMessage);
                throw err;
            }
        },
        [manager, activeConversationId, syncConversations, loadConversations],
    );

    const renameConversation = useCallback(
        async (conversationId: string, newTitle: string): Promise<void> => {
            if (!newTitle || newTitle.trim() === "") {
                throw new Error("Valid title is required");
            }

            const conversation = manager.getConversation(conversationId);
            if (!conversation) {
                throw new Error("Conversation not found");
            }

            const originalTitle = conversation.title;

            try {
                setError(null);

                // Optimistically update local state
                manager.updateConversationTitle(conversationId, newTitle.trim());
                syncConversations();

                // Call backend API
                await conversationsAPI.renameConversation(
                    conversationId,
                    newTitle.trim(),
                );
            } catch (err) {
                // On error, revert the title change
                manager.updateConversationTitle(conversationId, originalTitle);
                syncConversations();

                const errorMessage = ApiErrorHandler.getUserFriendlyMessage(err);
                setError(errorMessage);
                throw err;
            }
        },
        [manager, syncConversations],
    );

    /**
     * Switch to a different branch (alternative response) for a message.
     *
     * When a user message has multiple assistant responses (created via retry),
     * this function allows switching between them. The conversation view will
     * update to show the selected branch and continue from that point.
     */
    const switchBranch = useCallback(
        (messageId: number, branchIndex: number): void => {
            if (!activeConversationId) return;

            manager.switchToBranch(activeConversationId, messageId, branchIndex);
            syncConversations();
        },
        [manager, activeConversationId, syncConversations],
    );

    /**
     * Get information about available branches for a message.
     *
     * Returns branch navigation information including:
     * - count: Total number of branches (alternatives)
     * - activeIndex: Currently displayed branch (0-based)
     * - hasMultiple: Whether there are multiple branches to navigate
     */
    const getBranchInfo = useCallback(
        (messageId: number) => {
            if (!activeConversationId) {
                return { count: 1, activeIndex: 0, hasMultiple: false };
            }

            const count = manager.getBranchCount(activeConversationId, messageId);
            const activeIndex = manager.getActiveBranchIndex(
                activeConversationId,
                messageId,
            );
            const hasMultiple = manager.hasMultipleBranches(
                activeConversationId,
                messageId,
            );

            return {
                count,
                activeIndex,
                hasMultiple,
            };
        },
        [manager, activeConversationId],
    );

    const updateMessage = useCallback(
        async (messageId: string, newContent: string): Promise<void> => {
            if (!activeConversationId) {
                throw new Error("No active conversation");
            }

            const conversation = manager.getConversation(activeConversationId);
            if (!conversation?.backendConversation) {
                throw new Error("Conversation not ready");
            }

            const numericMessageId = parseInt(messageId);
            if (isNaN(numericMessageId)) {
                throw new Error("Invalid message ID");
            }

            // Find the message in backend conversation (defensive: messages may not be loaded yet)
            const backendMessage =
                conversation.backendConversation.messages?.[numericMessageId];
            if (!backendMessage) {
                throw new Error("Message not found");
            }

            const originalContent = backendMessage.content;

            try {
                setError(null);

                // Optimistically update local state
                backendMessage.content = newContent;

                // Also update the frontend message
                const frontendMessage = conversation.messages.find(
                    (m) => m.id === messageId,
                );
                if (frontendMessage) {
                    frontendMessage.content = newContent;
                }

                syncConversations();

                // Call update API
                const updateResponse = await chatAPI.updateMessage(
                    activeConversationId,
                    numericMessageId,
                    newContent,
                );

                if (updateResponse.messages?.[numericMessageId]) {
                    const updatedMsg = updateResponse.messages[numericMessageId];
                    conversation.backendConversation.messages[numericMessageId] =
                        updatedMsg;

                    // Update frontend message as well
                    if (frontendMessage) {
                        frontendMessage.content = updatedMsg.content;
                    }
                }

                syncConversations();
            } catch (err) {
                console.error("Failed to update message:", err);

                // On error, revert the changes
                backendMessage.content = originalContent;
                const frontendMessage = conversation.messages.find(
                    (m) => m.id === messageId,
                );
                if (frontendMessage) {
                    frontendMessage.content = originalContent;
                }
                syncConversations();

                const errorMessage = ApiErrorHandler.getUserFriendlyMessage(err);
                setError(errorMessage);
                throw err;
            }
        },
        [manager, activeConversationId, syncConversations],
    );

    const sendMessageStream = useCallback(
        async (
            conversationId: string | null,
            message: string,
            model: string,
            webSearch: boolean = false,
            attachedFileIds?: string[],
        ): Promise<string> => {
            let tempMessageId: string | undefined;
            let assistantPlaceholderId: string | undefined;
            let clientConversationId = conversationId;

            try {
                setError(null);

                // Handle new conversation case
                let isNewConversation = false;
                if (!conversationId) {

                    isNewConversation = true;
                    // Create conversation optimistically
                    const clientConversation = manager.createConversation(
                        message,
                    );
                    clientConversationId = clientConversation.id;

                    // Get IDs for streaming updates
                    const userMessage = clientConversation.messages.find(
                        (m) => m.role === "user" && m.content === message,
                    );
                    tempMessageId = userMessage?.id;

                    const createdConv = await conversationsAPI.createConversation(clientConversation.title);
                   
                    manager.rekeyConversation(clientConversationId, createdConv.id);
                    clientConversation.backendConversation = {
                        ...createdConv,
                        messages: {},
                        activeMessageId: 0,
                    };

                    setActiveConversationId(createdConv.id);
                    syncConversations();

                    conversationId = createdConv.id;
                }

                // Handle existing conversation case
                const conversation = manager.getConversation(conversationId);
                if (!conversation) {
                    throw new Error("Conversation not found");
                }

                // Add user message optimistically
                if (conversation.backendConversation) {
                    if (!isNewConversation) {
                        tempMessageId = manager.addMessageOptimistically(
                            conversationId,
                            message,
                            undefined, // Attachments will be populated from backend response
                        );
                    }

                    // Get assistant placeholder ID
                    const assistantPlaceholder = conversation.messages.find(
                        (m) => m.role === "assistant" && m.status === "pending",
                    );
                    assistantPlaceholderId = assistantPlaceholder?.id;

                    syncConversations();
                }

                const activeMessageId = manager.getActiveMessageId(conversationId);
                if (activeMessageId === undefined) {
                    throw new Error("Cannot send message: conversation not ready");
                }

                // Initialize streaming state and handlers
                const streamingState = new StreamingState();
                const handlers = createStreamingHandlers(
                    manager,
                    conversationId,
                    assistantPlaceholderId!,
                    streamingState,
                    syncConversations,
                );

                let streamError: string | undefined;

                // Stream the message
                await chatAPI.sendMessageStream(
                    conversationId,
                    activeMessageId,
                    model,
                    message,
                    webSearch,
                    attachedFileIds,
                    handlers.onChunk,
                    handlers.onReasoning,
                    handlers.onToolCall,
                    // onMetadata - Update user message immediately
                    (metadata) => {
                        if (tempMessageId) {
                            updateUserMessageAfterSave(
                                manager,
                                conversationId!,
                                tempMessageId,
                                metadata.userMessageId,
                                syncConversations,
                            );
                        }
                    },
                    // onComplete - Update IDs (called even after errors!)
                    (data) => {
                        if (assistantPlaceholderId) {
                            updateAssistantMessageAfterComplete(
                                manager,
                                conversationId!,
                                assistantPlaceholderId,
                                data.assistantMessageId,
                                streamingState,
                                data.userMessageId,
                                syncConversations,
                                streamError, // Pass error if one occurred
                                data.streamStats,
                                model,
                            );
                        }
                    },
                    // onError - Just capture the error, onComplete will handle it
                    (error) => {
                        console.error("Stream error:", error);
                        streamingState.cancelPendingSync();
                        streamError = error;
                    },
                );
                return conversationId;
            } catch (err) {
                console.error("Failed to send message:", err);

                if (assistantPlaceholderId && clientConversationId) {
                    const errorMsg = ApiErrorHandler.getUserFriendlyMessage(err);
                    manager.markAssistantFailed(
                        clientConversationId,
                        assistantPlaceholderId,
                        errorMsg,
                    );
                    syncConversations();
                }

                throw err;
            }
        },
        [manager, syncConversations],
    );

    const retryMessageStream = useCallback(
        async (messageId: string, model: string): Promise<void> => {
            if (!activeConversationId) {
                throw new Error("No active conversation");
            }

            const conversation = manager.getConversation(activeConversationId);
            if (!conversation?.backendConversation) {
                throw new Error("Conversation not ready");
            }

            try {
                setError(null);

                // Determine the parent user message for retry
                const parentId = manager.getRetryParentId(
                    activeConversationId,
                    messageId,
                );
                if (parentId === undefined) {
                    throw new Error("Cannot determine parent message for retry");
                }

                // Add optimistic assistant placeholder
                const assistantPlaceholderId = manager.generateTempId();
                const assistantPlaceholder: FrontendMessage = {
                    id: assistantPlaceholderId,
                    role: "assistant",
                    content: "",
                    status: "pending",
                    timestamp: Date.now(),
                };

                // Remove the old assistant message being retried from the frontend messages array
                // This prevents it from showing alongside the new retry response
                const oldMessageIndex = conversation.messages.findIndex(
                    (m) => m.id === messageId
                );
                if (oldMessageIndex !== -1) {
                    // Insert the new placeholder at the same position as the old message
                    conversation.messages.splice(oldMessageIndex, 1, assistantPlaceholder);
                } else {
                    conversation.messages.push(assistantPlaceholder);
                }
                conversation.pendingMessageIds.add(assistantPlaceholderId);
                syncConversations();

                // Initialize streaming state and handlers
                const streamingState = new StreamingState();
                const handlers = createStreamingHandlers(
                    manager,
                    activeConversationId,
                    assistantPlaceholderId,
                    streamingState,
                    syncConversations,
                );

                let streamError: string | undefined;

                await chatAPI.retryMessageStream(
                    activeConversationId,
                    parentId,
                    model,
                    handlers.onChunk,
                    handlers.onReasoning,
                    handlers.onToolCall,
                    // onMetadata (not strictly needed here)
                    () => {
                    },
                    // onComplete - Update IDs (called even after errors!)
                    (data) => {
                        updateAssistantMessageAfterComplete(
                            manager,
                            activeConversationId,
                            assistantPlaceholderId,
                            data.assistantMessageId,
                            streamingState,
                            parentId,
                            syncConversations,
                            streamError,
                            data.streamStats,
                            model,
                        );

                        // After completing, refresh conversation to rebuild tree/branches accurately
                        (async () => {
                            try {
                                const msgs = await conversationsAPI.fetchConversationMessages(activeConversationId);
                                manager.updateWithChatResponse(activeConversationId, msgs);
                                // Set the active branch to the new message and rebuild messages array
                                manager.setActiveBranch(activeConversationId, parentId, data.assistantMessageId);
                                syncConversations();
                            } catch (e) {
                                console.error("Failed to refresh conversation after retry stream:", e);
                            }
                        })();
                    },
                    // onError - Just capture the error, onComplete will handle it
                    (err) => {
                        console.error("Retry stream error:", err);
                        streamingState.cancelPendingSync();
                        streamError = err;
                    },
                );
            } catch (err) {
                console.error("Failed to retry message (stream):", err);
                const errorMessage = ApiErrorHandler.getUserFriendlyMessage(err);
                setError(errorMessage);
                throw err;
            }
        },
        [manager, activeConversationId, syncConversations],
    );

    return {
        conversations,
        activeConversationId,
        currentConversation,
        isLoading,
        error,
        sendMessageStream,
        retryMessageStream,
        updateMessage,
        getCurrentMessages,
        selectConversation,
        startNewChat,
        clearError,
        hasPendingMessages,
        deleteConversation,
        renameConversation,
        switchBranch,
        getBranchInfo,
    };
};
