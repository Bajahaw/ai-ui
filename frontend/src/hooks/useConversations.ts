import { useState, useCallback, useEffect, useRef } from "react";
import { FrontendMessage, conversationsAPI, chatAPI } from "@/lib/api";
import { ApiErrorHandler } from "@/lib/api/errorHandler";
import {
  ClientConversationManager,
  ClientConversation,
} from "@/lib/clientConversationManager";

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

  const sendMessage = useCallback(
    async (
      conversationId: string | null,
      message: string,
      model: string,
      webSearch: boolean = false,
      attachment?: string,
    ): Promise<string> => {
      let tempMessageId: string | undefined;
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

          // Extract the user message ID for error handling
          const userMessage = clientConversation.messages.find(
            (m) => m.role === "user" && m.content === message,
          );
          tempMessageId = userMessage?.id;

          syncConversations();
          setActiveConversationId(clientConversationId);

          // Create conversation on server first to get real UUID
          const title =
            message.length > 50 ? message.substring(0, 47) + "..." : message;
          const createdConv = await conversationsAPI.createConversation(title);

          // Send first message using the real conversation UUID
          const chatResponse = await chatAPI.sendMessage(
            createdConv.id,
            null, // null parentId for new conversation

            model,

            message,

            webSearch,

            attachment,
          );

          // Update local state with returned messages; manager will re-key if needed
          manager.updateWithChatResponse(
            clientConversationId,
            chatResponse.messages,
            false,
          );

          // Optionally fetch full conversation messages (empty map if none)
          try {
            const allMessages =
              await conversationsAPI.fetchConversationMessages(createdConv.id);
            manager.updateWithChatResponse(createdConv.id, allMessages, true);
          } catch (e) {
            console.warn("Failed to fetch conversation messages:", e);
          }

          // Switch active conversation to the real UUID

          setActiveConversationId(createdConv.id);
          syncConversations();

          return createdConv.id;
        }

        // Handle existing conversation case
        const conversation = manager.getConversation(conversationId);
        if (!conversation) {
          throw new Error("Conversation not found");
        }

        // Add user message optimistically for existing conversations
        if (conversation.backendConversation) {
          tempMessageId = manager.addMessageOptimistically(
            conversationId,
            message,
            attachment,
          );
          syncConversations();
        }

        const activeMessageId = manager.getActiveMessageId(conversationId);
        if (activeMessageId === undefined) {
          // Fallback: if no activeMessageId, use the latest assistant message
          const backendConv = conversation.backendConversation;

          if (backendConv) {
            const msgs = backendConv.messages || {};
            const assistantMessages = Object.values(msgs).filter(
              (msg) => msg.role === "assistant",
            );
            if (assistantMessages.length > 0) {
              const latestAssistant =
                assistantMessages[assistantMessages.length - 1];

              const fallbackActiveId = latestAssistant.id;
              // Update the conversation's activeMessageId
              backendConv.activeMessageId = fallbackActiveId;
            } else {
              throw new Error("Cannot send message: conversation not ready");
            }
          } else {
            throw new Error("Cannot send message: conversation not ready");
          }
        }

        const finalActiveMessageId = manager.getActiveMessageId(conversationId);

        if (finalActiveMessageId === undefined) {
          throw new Error(
            "Cannot determine active message ID for conversation",
          );
        }

        const chatResponse = await chatAPI.sendMessage(
          conversationId,
          finalActiveMessageId, // Use activeMessageId as parentId for the new message
          model,
          message,
          webSearch,
          attachment,
        );

        manager.updateWithChatResponse(
          conversationId,
          chatResponse.messages,
          false,
        );
        syncConversations();

        return conversationId;
      } catch (err) {
        console.error("Failed to send message:", err);

        if (tempMessageId && clientConversationId) {
          const errorMsg = ApiErrorHandler.getUserFriendlyMessage(err);
          manager.markAssistantFailed(
            clientConversationId,
            tempMessageId,
            errorMsg,
          );
          syncConversations();
        }

        throw err;
      }
    },
    [manager, syncConversations],
  );

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
  const retryMessage = useCallback(
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

        // Get the parent ID for retry (the user message before the assistant response)
        const parentId = manager.getRetryParentId(
          activeConversationId,
          messageId,
        );
        if (parentId === undefined) {
          throw new Error("Cannot determine parent message for retry");
        }

        // Add optimistic placeholder for new assistant response
        const assistantPlaceholderId = manager.generateTempId();
        const assistantPlaceholder: FrontendMessage = {
          id: assistantPlaceholderId,
          role: "assistant",
          content: "",
          status: "pending",
          timestamp: Date.now(),
        };

        // Add placeholder to conversation
        conversation.messages.push(assistantPlaceholder);
        conversation.pendingMessageIds.add(assistantPlaceholderId);
        syncConversations();

        // Call retry API - this returns both parent and new assistant messages
        const retryResponse = await chatAPI.retryMessage(
          activeConversationId,
          parentId,
          model,
        );

        // Update conversation with both messages (parent with updated children + new assistant)
        manager.updateWithRetryResponse(
          activeConversationId,
          retryResponse.messages,
        );

        // Force a conversation refresh to ensure branch navigation appears
        syncConversations();
      } catch (err) {
        console.error("Failed to retry message:", err);

        // Remove placeholder on error
        const conversation = manager.getConversation(activeConversationId);
        if (conversation) {
          conversation.messages = conversation.messages.filter(
            (m) => !conversation.pendingMessageIds.has(m.id),
          );
          conversation.pendingMessageIds.clear();
          syncConversations();
        }

        throw err;
      }
    },
    [manager, activeConversationId, syncConversations],
  );

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
            manager.updateWithChatResponse(conversationId, msgs, true);
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
            message.length > 50 ? message.substring(0, 47) + "..." : message;
          const createdConv = await conversationsAPI.createConversation(title);

          // Use a simple variable to accumulate content
          let accumulatedContent = "";
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
                      assistMsg.status = "success";
                      // Ensure final content is set (in case last RAF was cancelled)
                      assistMsg.content = accumulatedContent;
                      conv.pendingMessageIds.delete(assistantPlaceholderId);
                    }
                  }
                }
              }
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

          // After streaming completes, set up the backend conversation properly
          try {
            // Fetch the full conversation to set up backend structure
            const fullConversation = await conversationsAPI.fetchConversation(realConvId);
            const conv = manager.getConversation(realConvId);
            if (conv && realAssistantMessageId) {
              conv.backendConversation = fullConversation;
              // Set activeMessageId to the assistant message so next message can be sent
              conv.backendConversation.activeMessageId = realAssistantMessageId;
            }
          } catch (e) {
            console.error("Failed to fetch conversation after streaming:", e);
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
          // Fallback logic
          const backendConv = conversation.backendConversation;
          if (backendConv) {
            const msgs = backendConv.messages || {};
            const assistantMessages = Object.values(msgs).filter(
              (msg) => msg.role === "assistant",
            );
            if (assistantMessages.length > 0) {
              const latestAssistant =
                assistantMessages[assistantMessages.length - 1];
              backendConv.activeMessageId = latestAssistant.id;
            } else {
              throw new Error("Cannot send message: conversation not ready");
            }
          } else {
            throw new Error("Cannot send message: conversation not ready");
          }
        }

        const finalActiveMessageId = manager.getActiveMessageId(conversationId);
        if (finalActiveMessageId === undefined) {
          throw new Error("Cannot determine active message ID");
        }

        // Use a simple variable to accumulate content
        let accumulatedContent = "";
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
            
            // Update assistant message ID and status
            if (assistantPlaceholderId) {
              const conv = manager.getConversation(conversationId);
              if (conv) {
                const assistMsg = conv.messages.find(m => m.id === assistantPlaceholderId);
                if (assistMsg) {
                  assistMsg.id = data.assistantMessageId.toString();
                  assistMsg.status = "success";
                  // Ensure final content is set (in case last RAF was cancelled)
                  assistMsg.content = accumulatedContent;
                  conv.pendingMessageIds.delete(assistantPlaceholderId);
                }
              }
            }
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

  return {
    conversations,
    activeConversationId,
    currentConversation,
    isLoading,
    error,
    sendMessage,
    sendMessageStream,
    retryMessage,
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
