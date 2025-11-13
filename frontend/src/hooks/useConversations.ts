import {useCallback, useEffect, useRef, useState} from "react";
import {chatAPI, conversationsAPI, FrontendMessage, ToolCall} from "@/lib/api";
import {ApiErrorHandler} from "@/lib/api/errorHandler";
import {ClientConversation, ClientConversationManager,} from "@/lib/clientConversationManager";

export const useConversations = () => {
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

    useEffect(() => {
        loadConversations();
    }, []);

    const loadConversations = useCallback(async () => {
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

    // Removed non-streaming sendMessage: use sendMessageStream exclusively

    /**
     * Retry an assistant message to generate an alternative response.
     *
     * This creates a new branch in the conversation tree:
     * - Finds the user message that preceded the assistant message
     * - Sends a retry request to generate a new response from that point
     * - The new response becomes an alternative branch (child) of the user message
     * - Users can navigate between branches using the branch navigation controls
     * - Backend returns both the parent message (with updated children) and new assistant message
     */
        // Removed non-streaming retryMessage: use retryMessageStream exclusively

        // Streamed retry to generate an alternative assistant response with chunks

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
                return {count: 1, activeIndex: 0, hasMultiple: false};
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
                    if (!conversation.backendConversation.messages) {
                        conversation.backendConversation.messages = {};
                    }
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
            attachment?: string,
        ): Promise<string> => {
            let tempMessageId: string | undefined;
            let assistantPlaceholderId: string | undefined;
            let clientConversationId = conversationId;

            try {
                setError(null);

                // Handle new conversation case
                if (!conversationId) {
                    // Create conversation optimistically
                    const clientConversation = manager.createConversation(
                        message,
                        attachment,
                    );
                    clientConversationId = clientConversation.id;

                    // Get IDs for streaming updates
                    const userMessage = clientConversation.messages.find(
                        (m) => m.role === "user" && m.content === message,
                    );
                    tempMessageId = userMessage?.id;

                    const assistantPlaceholder = clientConversation.messages.find(
                        (m) => m.role === "assistant" && m.status === "pending",
                    );  
                    assistantPlaceholderId = assistantPlaceholder?.id;

                    syncConversations();
                    setActiveConversationId(clientConversationId);

                    // Create conversation on server first to get real UUID
                    const title =
                        message.length > 60 ? message.substring(0, 60) + "..." : message;
                    const createdConv = await conversationsAPI.createConversation(title);

                    // Use a simple variable to accumulate content and reasoning
                    let accumulatedContent = "";
                    let accumulatedReasoning = "";
                    let reasoningStartTime: number | null = null; // Track when reasoning starts
                    let realConvId = createdConv.id;
                    let rafId: number | null = null;
                    let realAssistantMessageId: number | null = null;

                    // Stream the message
                    await chatAPI.sendMessageStream(
                        createdConv.id,
                        null,
                        model,
                        message,
                        webSearch,
                        attachment,
                        // onChunk - Update content and request animation frame for smooth rendering
                        (chunk: string) => {
                            accumulatedContent += chunk;

                            // Update content immediately (no sync yet)
                            if (assistantPlaceholderId && clientConversationId) {
                                manager.updateMessageContent(
                                    clientConversationId,
                                    assistantPlaceholderId,
                                    accumulatedContent,
                                );
                            }

                            // Cancel previous frame request if any
                            if (rafId !== null) {
                                cancelAnimationFrame(rafId);
                            }

                            // Schedule sync on next animation frame (60fps max)
                            rafId = requestAnimationFrame(() => {
                                syncConversations();
                                rafId = null;
                            });
                        },
                        // onReasoning - Update reasoning and request animation frame for smooth rendering
                        (reasoning: string) => {
                            accumulatedReasoning += reasoning;

                            // Track start time on first reasoning chunk
                            if (reasoningStartTime === null && reasoning) {
                                reasoningStartTime = Date.now();
                            }

                            // Update reasoning immediately (no sync yet)
                            if (assistantPlaceholderId && clientConversationId) {
                                const conv = manager.getConversation(clientConversationId);
                                if (conv) {
                                    const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
                                    if (assistMsg) {
                                        assistMsg.reasoning = accumulatedReasoning;
                                    }
                                }
                            }

                            // Cancel previous frame request if any
                            if (rafId !== null) {
                                cancelAnimationFrame(rafId);
                            }

                            // Schedule sync on next animation frame (60fps max)
                            rafId = requestAnimationFrame(() => {
                                syncConversations();
                                rafId = null;
                            });
                        },
                        // onToolCall - Add tool call to message
                        (toolCall: ToolCall) => {
                            if (assistantPlaceholderId && clientConversationId) {
                                manager.addToolCall(
                                    clientConversationId,
                                    assistantPlaceholderId,
                                    toolCall,
                                );

                                // If there's accumulated reasoning and this is the first tool call event (no output yet),
                                // append tool usage information to show when the model decided to use the tool
                                if (accumulatedReasoning && !toolCall.tool_output) {
                                    accumulatedReasoning += ` \n\`using tool:${toolCall.name}\`\n `;
                                    const conv = manager.getConversation(clientConversationId);
                                    if (conv) {
                                        const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
                                        if (assistMsg) {
                                            assistMsg.reasoning = accumulatedReasoning;
                                        }
                                    }
                                }
                            }

                            // Cancel previous frame request if any
                            if (rafId !== null) {
                                cancelAnimationFrame(rafId);
                            }

                            // Schedule sync on next animation frame (60fps max)
                            rafId = requestAnimationFrame(() => {
                                syncConversations();
                                rafId = null;
                            });
                        },
                        // onMetadata - Get the real backend ID and update user message immediately
                        (metadata) => {
                            realConvId = metadata.conversationId;

                            // User message is saved! Update its ID and status immediately
                            if (tempMessageId && clientConversationId) {
                                const conv = manager.getConversation(clientConversationId);
                                if (conv) {
                                    const userMsg = conv.messages.find(m => m.id === tempMessageId);
                                    if (userMsg) {
                                        userMsg.id = metadata.userMessageId.toString();
                                        userMsg.status = "success"; // Message is saved, show actions now!
                                        conv.pendingMessageIds.delete(tempMessageId);
                                        
                                        // Ensure backendConversation.messages exists and add the user message
                                        if (!conv.backendConversation) {
                                            conv.backendConversation = {
                                                id: realConvId,
                                                userId: "",
                                                title: conv.title,
                                                messages: {},
                                            } as any;
                                        }
                                        if (!conv.backendConversation!.messages) {
                                            conv.backendConversation!.messages = {};
                                        }
                                        conv.backendConversation!.messages[metadata.userMessageId] = {
                                            id: metadata.userMessageId,
                                            convId: realConvId,
                                            role: userMsg.role,
                                            content: userMsg.content,
                                            attachment: userMsg.attachment,
                                            createdAt: new Date().toISOString(),
                                            updatedAt: new Date().toISOString(),
                                        } as any;
                                        
                                        syncConversations(); // Sync to show action buttons
                                    }
                                }
                            }
                        },
                        // onComplete - Update IDs and sync ONCE
                        (data) => {
                            // Cancel any pending animation frame
                            if (rafId !== null) {
                                cancelAnimationFrame(rafId);
                                rafId = null;
                            }

                            // Store the real assistant message ID
                            realAssistantMessageId = data.assistantMessageId;

                            // Calculate reasoning duration if reasoning was used
                            const reasoningDuration = reasoningStartTime !== null && accumulatedReasoning
                                ? Math.round((Date.now() - reasoningStartTime) / 1000)
                                : undefined;

                            // Re-key conversation from temp to real ID
                            if (clientConversationId && realConvId !== clientConversationId) {
                                manager.rekeyConversation(clientConversationId, realConvId);

                                // Update assistant message ID and status
                                if (assistantPlaceholderId) {
                                    const conv = manager.getConversation(realConvId);
                                    if (conv) {
                                        const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
                                        if (assistMsg) {
                                            assistMsg.id = data.assistantMessageId.toString();
                                            // Only update status to success if not already in error state
                                            if (assistMsg.status !== "error") {
                                                assistMsg.status = "success";
                                                // Ensure final content and reasoning are set (in case last RAF was cancelled)
                                                assistMsg.content = accumulatedContent;
                                                assistMsg.reasoning = accumulatedReasoning;
                                                assistMsg.reasoningDuration = reasoningDuration;
                                            }
                                            conv.pendingMessageIds.delete(assistantPlaceholderId);
                                            
                                            // Ensure backendConversation.messages exists and add the assistant message
                                            if (!conv.backendConversation) {
                                                conv.backendConversation = {
                                                    id: realConvId,
                                                    userId: "",
                                                    title: conv.title,
                                                    messages: {},
                                                } as any;
                                            }
                                            if (!conv.backendConversation!.messages) {
                                                conv.backendConversation!.messages = {};
                                            }
                                            conv.backendConversation!.messages[data.assistantMessageId] = {
                                                id: data.assistantMessageId,
                                                convId: realConvId,
                                                role: assistMsg.role,
                                                content: assistMsg.content,
                                                reasoning: assistMsg.reasoning,
                                                toolCalls: assistMsg.toolCalls,
                                                parentId: data.userMessageId,
                                                createdAt: new Date().toISOString(),
                                                updatedAt: new Date().toISOString(),
                                            } as any;
                                        }
                                    }
                                }
                            }
                            
                            // Sync immediately to update UI (change "thinking..." to "thought for X seconds")
                            syncConversations();
                        },
                        // onError
                        (error) => {
                            console.error("Stream error:", error);
                            if (assistantPlaceholderId && clientConversationId) {
                                manager.markAssistantFailed(
                                    clientConversationId,
                                    assistantPlaceholderId,
                                    error,
                                );
                            }
                        },
                    );

                    // Use the conversation data we got from createConversation (includes timestamps)
                    // This avoids the "blink" caused by refetching and rebuilding messages from backend
                    const conv = manager.getConversation(realConvId);
                    if (conv && realAssistantMessageId) {
                        // Use the createdConv data which has all the proper fields including timestamps
                        if (!conv.backendConversation) {
                            conv.backendConversation = {
                                ...createdConv,
                                messages: {},
                                activeMessageId: realAssistantMessageId,
                            };
                        } else {
                            // Update timestamps and activeMessageId
                            conv.backendConversation.createdAt = createdConv.createdAt;
                            conv.backendConversation.updatedAt = createdConv.updatedAt;
                            conv.backendConversation.activeMessageId = realAssistantMessageId;
                        }
                    }

                    // Final sync ONCE after streaming completes
                    setActiveConversationId(realConvId);
                    syncConversations();

                    return realConvId;
                }

                // Handle existing conversation case
                const conversation = manager.getConversation(conversationId);
                if (!conversation) {
                    throw new Error("Conversation not found");
                }

                // Add user message optimistically
                if (conversation.backendConversation) {
                    tempMessageId = manager.addMessageOptimistically(
                        conversationId,
                        message,
                        attachment,
                    );

                    // Get assistant placeholder ID
                    const assistantPlaceholder = conversation.messages.find(
                        (m) => m.role === "assistant" && m.status === "pending" && !m.content,
                    );
                    assistantPlaceholderId = assistantPlaceholder?.id;

                    syncConversations();
                }

                const activeMessageId = manager.getActiveMessageId(conversationId);
                if (activeMessageId === undefined) {
                    throw new Error("Cannot send message: conversation not ready");
                }

                const finalActiveMessageId = manager.getActiveMessageId(conversationId);
                if (finalActiveMessageId === undefined) {
                    throw new Error("Cannot determine active message ID");
                }

                // Use a simple variable to accumulate content and reasoning
                let accumulatedContent = "";
                let accumulatedReasoning = "";
                let reasoningStartTime: number | null = null; // Track when reasoning starts
                let rafId: number | null = null;

                // Stream the message
                await chatAPI.sendMessageStream(
                    conversationId,
                    finalActiveMessageId,
                    model,
                    message,
                    webSearch,
                    attachment,
                    // onChunk - Update on animation frame for smooth rendering
                    (chunk: string) => {
                        accumulatedContent += chunk;

                        // Update content immediately (no sync yet)
                        if (assistantPlaceholderId) {
                            manager.updateMessageContent(
                                conversationId,
                                assistantPlaceholderId,
                                accumulatedContent,
                            );
                        }

                        // Cancel previous frame request if any
                        if (rafId !== null) {
                            cancelAnimationFrame(rafId);
                        }

                        // Schedule sync on next animation frame (60fps max)
                        rafId = requestAnimationFrame(() => {
                            syncConversations();
                            rafId = null;
                        });
                    },
                    // onReasoning - Update reasoning on animation frame for smooth rendering
                    (reasoning: string) => {
                        accumulatedReasoning += reasoning;

                        // Track start time on first reasoning chunk
                        if (reasoningStartTime === null && reasoning) {
                            reasoningStartTime = Date.now();
                        }

                        // Update reasoning immediately (no sync yet)
                        if (assistantPlaceholderId) {
                            const conv = manager.getConversation(conversationId);
                            if (conv) {
                                const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
                                if (assistMsg) {
                                    assistMsg.reasoning = accumulatedReasoning;
                                }
                            }
                        }

                        // Cancel previous frame request if any
                        if (rafId !== null) {
                            cancelAnimationFrame(rafId);
                        }

                        // Schedule sync on next animation frame (60fps max)
                        rafId = requestAnimationFrame(() => {
                            syncConversations();
                            rafId = null;
                        });
                    },
                    // onToolCall - Add tool call to message
                    (toolCall: ToolCall) => {
                        if (assistantPlaceholderId) {
                            manager.addToolCall(
                                conversationId,
                                assistantPlaceholderId,
                                toolCall,
                            );

                            // If there's accumulated reasoning and this is the first tool call event (no output yet),
                            // append tool usage information to show when the model decided to use the tool
                            if (accumulatedReasoning && !toolCall.tool_output) {
                                accumulatedReasoning += ` \n\`using tool:${toolCall.name}\`\n `;
                                const conv = manager.getConversation(conversationId);
                                if (conv) {
                                    const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
                                    if (assistMsg) {
                                        assistMsg.reasoning = accumulatedReasoning;
                                    }
                                }
                            }
                        }

                        // Cancel previous frame request if any
                        if (rafId !== null) {
                            cancelAnimationFrame(rafId);
                        }

                        // Schedule sync on next animation frame (60fps max)
                        rafId = requestAnimationFrame(() => {
                            syncConversations();
                            rafId = null;
                        });
                    },
                    // onMetadata - Update user message immediately
                    (metadata) => {
                        // User message is saved! Update its ID and status immediately
                        if (tempMessageId) {
                            const conv = manager.getConversation(conversationId);
                            if (conv) {
                                const userMsg = conv.messages.find(m => m.id === tempMessageId);
                                if (userMsg) {
                                    userMsg.id = metadata.userMessageId.toString();
                                    userMsg.status = "success"; // Message is saved, show actions now!
                                    conv.pendingMessageIds.delete(tempMessageId);
                                    
                                    // Ensure backendConversation.messages exists and add the user message
                                    if (conv.backendConversation) {
                                        if (!conv.backendConversation.messages) {
                                            conv.backendConversation.messages = {};
                                        }
                                        conv.backendConversation.messages[metadata.userMessageId] = {
                                            id: metadata.userMessageId,
                                            convId: conversationId,
                                            role: userMsg.role,
                                            content: userMsg.content,
                                            attachment: userMsg.attachment,
                                            createdAt: new Date().toISOString(),
                                            updatedAt: new Date().toISOString(),
                                        } as any;
                                    }
                                    
                                    syncConversations(); // Sync to show action buttons
                                }
                            }
                        }
                    },
                    // onComplete - Update IDs and sync ONCE
                    (data) => {
                        // Cancel any pending animation frame
                        if (rafId !== null) {
                            cancelAnimationFrame(rafId);
                            rafId = null;
                        }

                        // Calculate reasoning duration if reasoning was used
                        const reasoningDuration = reasoningStartTime !== null && accumulatedReasoning
                            ? Math.round((Date.now() - reasoningStartTime) / 1000)
                            : undefined;

                        // Update assistant message ID and status
                        if (assistantPlaceholderId) {
                            const conv = manager.getConversation(conversationId);
                            if (conv) {
                                const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
                                if (assistMsg) {
                                    assistMsg.id = data.assistantMessageId.toString();
                                    assistMsg.status = "success";
                                    // Ensure final content and reasoning are set (in case last RAF was cancelled)
                                    assistMsg.content = accumulatedContent;
                                    assistMsg.reasoning = accumulatedReasoning;
                                    assistMsg.reasoningDuration = reasoningDuration;
                                    conv.pendingMessageIds.delete(assistantPlaceholderId);
                                    
                                    // Critical: set the active parent to the latest assistant message
                                    if (conv.backendConversation) {
                                        conv.backendConversation.activeMessageId = data.assistantMessageId;
                                        
                                        // Ensure backendConversation.messages exists and add the assistant message
                                        if (!conv.backendConversation.messages) {
                                            conv.backendConversation.messages = {};
                                        }
                                        conv.backendConversation.messages[data.assistantMessageId] = {
                                            id: data.assistantMessageId,
                                            convId: conversationId,
                                            role: assistMsg.role,
                                            content: assistMsg.content,
                                            reasoning: assistMsg.reasoning,
                                            toolCalls: assistMsg.toolCalls,
                                            parentId: data.userMessageId,
                                            createdAt: new Date().toISOString(),
                                            updatedAt: new Date().toISOString(),
                                        } as any;
                                    }
                                }
                            }
                        }
                        
                        // Sync immediately to update UI (change "thinking..." to "thought for X seconds")
                        syncConversations();
                    },
                    // onError
                    (error) => {
                        console.error("Stream error:", error);
                        if (assistantPlaceholderId) {
                            manager.markAssistantFailed(
                                conversationId,
                                assistantPlaceholderId,
                                error,
                            );
                        }
                    },
                );

                // Final sync ONCE after streaming completes
                syncConversations();
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

                conversation.messages.push(assistantPlaceholder);
                conversation.pendingMessageIds.add(assistantPlaceholderId);
                syncConversations();

                // Accumulate streamed content and reasoning
                let accumulatedContent = "";
                let accumulatedReasoning = "";
                let reasoningStartTime: number | null = null; // Track when reasoning starts
                let rafId: number | null = null;

                let completedAssistantId: number | null = null;
                await chatAPI.retryMessageStream(
                    activeConversationId,
                    parentId,
                    model,
                    // onChunk
                    (chunk: string) => {
                        accumulatedContent += chunk;
                        const conv = manager.getConversation(activeConversationId);
                        if (conv) {
                            manager.updateMessageContent(
                                activeConversationId,
                                assistantPlaceholderId,
                                accumulatedContent,
                            );
                        }
                        if (rafId !== null) cancelAnimationFrame(rafId);
                        rafId = requestAnimationFrame(() => {
                            syncConversations();
                            rafId = null;
                        });
                    },
                    // onReasoning
                    (reasoning: string) => {
                        accumulatedReasoning += reasoning;

                        // Track start time on first reasoning chunk
                        if (reasoningStartTime === null && reasoning) {
                            reasoningStartTime = Date.now();
                        }

                        const conv = manager.getConversation(activeConversationId);
                        if (conv) {
                            const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
                            if (assistMsg) {
                                assistMsg.reasoning = accumulatedReasoning;
                            }
                        }
                        if (rafId !== null) cancelAnimationFrame(rafId);
                        rafId = requestAnimationFrame(() => {
                            syncConversations();
                            rafId = null;
                        });
                    },
                    // onToolCall
                    (toolCall: ToolCall) => {
                        manager.addToolCall(
                            activeConversationId,
                            assistantPlaceholderId,
                            toolCall,
                        );

                        // If there's accumulated reasoning and this is the first tool call event (no output yet),
                        // append tool usage information to show when the model decided to use the tool
                        if (accumulatedReasoning && !toolCall.tool_output) {
                            accumulatedReasoning += ` \n\`using tool:${toolCall.name}\`\n `;
                            const conv = manager.getConversation(activeConversationId);
                            if (conv) {
                                const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
                                if (assistMsg) {
                                    assistMsg.reasoning = accumulatedReasoning;
                                }
                            }
                        }

                        if (rafId !== null) cancelAnimationFrame(rafId);
                        rafId = requestAnimationFrame(() => {
                            syncConversations();
                            rafId = null;
                        });
                    },
                    // onMetadata (not strictly needed here)
                    () => {
                    },
                    // onComplete
                    (data) => {
                        // Capture assistant message id for post-stream refresh
                        completedAssistantId = data.assistantMessageId;
                        if (rafId !== null) {
                            cancelAnimationFrame(rafId);
                            rafId = null;
                        }

                        // Calculate reasoning duration if reasoning was used
                        const reasoningDuration = reasoningStartTime !== null && accumulatedReasoning
                            ? Math.round((Date.now() - reasoningStartTime) / 1000)
                            : undefined;

                        const conv = manager.getConversation(activeConversationId);
                        if (!conv) return;

                        const assistMsg = conv.messages.find(
                            (m) => m.id === assistantPlaceholderId,
                        );
                        if (assistMsg) {
                            assistMsg.id = data.assistantMessageId.toString();
                            assistMsg.status = "success";
                            assistMsg.content = accumulatedContent;
                            assistMsg.reasoning = accumulatedReasoning;
                            assistMsg.reasoningDuration = reasoningDuration;
                            conv.pendingMessageIds.delete(assistantPlaceholderId);
                        }

                        if (conv.backendConversation) {
                            if (!conv.backendConversation.messages) {
                                conv.backendConversation.messages = {};
                            }
                            const backendMsgs = conv.backendConversation.messages;
                            if (!backendMsgs[parentId]) {
                                backendMsgs[parentId] = {
                                    id: parentId,
                                    convId: activeConversationId,
                                    role: "user",
                                    content: "",
                                    parentId: undefined,
                                    children: [],
                                };
                            }
                            backendMsgs[data.assistantMessageId] = {
                                id: data.assistantMessageId,
                                convId: activeConversationId,
                                role: "assistant",
                                content: accumulatedContent,
                                reasoning: accumulatedReasoning,
                                parentId: parentId,
                                children: [],
                            };
                            const parent = backendMsgs[parentId];
                            if (parent) {
                                if (!Array.isArray(parent.children)) parent.children = [];
                                if (!parent.children.includes(data.assistantMessageId)) {
                                    parent.children.push(data.assistantMessageId);
                                }
                            }
                            conv.backendConversation.activeMessageId = data.assistantMessageId;
                        }
                        
                        // Sync immediately to update UI (change "thinking..." to "thought for X seconds")
                        syncConversations();
                    },
                    // onError
                    (err) => {
                        console.error("Retry stream error:", err);
                        const conv = manager.getConversation(activeConversationId);
                        if (conv) {
                            manager.markAssistantFailed(
                                activeConversationId,
                                assistantPlaceholderId,
                                err,
                            );
                        }
                    },
                );

                // After stream completes, refresh full conversation to rebuild tree/branches accurately
                try {
                    const msgs = await conversationsAPI.fetchConversationMessages(activeConversationId);
                    manager.updateWithChatResponse(activeConversationId, msgs);
                    // Ensure activeMessageId remains set to the latest assistant we just created
                    if (completedAssistantId !== null) {
                        const refreshed = manager.getConversation(activeConversationId);
                        if (refreshed?.backendConversation) {
                            refreshed.backendConversation.activeMessageId = completedAssistantId;
                        }
                    }
                } catch (e) {
                    console.error("Failed to refresh conversation after retry stream:", e);
                }

                syncConversations();
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
