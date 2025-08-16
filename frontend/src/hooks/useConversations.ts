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
    ): Promise<string> => {
      let tempMessageId: string | undefined;
      let clientConversationId = conversationId;

      try {
        setError(null);

        // Handle new conversation case
        if (!conversationId) {
          // Create conversation optimistically
          const clientConversation = manager.createConversation(message);
          clientConversationId = clientConversation.id;
          syncConversations();
          setActiveConversationId(clientConversationId);

          // Send message to server - server will create conversation on the fly
          const chatResponse = await chatAPI.sendMessage(
            null, // null conversationId triggers server-side conversation creation
            null, // null activeMessageId for new conversation
            model,
            message,
            webSearch,
          );

          // Set up backend conversation structure from chat response
          const messageIds = Object.keys(chatResponse.messages).map(Number);
          const maxMessageId =
            messageIds.length > 0 ? Math.max(...messageIds) : 1;

          const backendConversation = {
            id: chatResponse.conversationId,
            title: clientConversation.title,
            messages: chatResponse.messages,
            root: messageIds.length > 0 ? [Math.min(...messageIds)] : [1],
            activeMessageId: maxMessageId,
          };

          // Set backend conversation and process messages in one go
          const conversation = manager.getConversation(clientConversationId);
          if (conversation) {
            conversation.backendConversation = backendConversation;
          }

          manager.updateWithChatResponse(
            clientConversationId,
            chatResponse.messages,
          );
          syncConversations();

          return clientConversationId;
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
          );
          syncConversations();
        }

        const activeMessageId = manager.getActiveMessageId(conversationId);
        if (activeMessageId === undefined) {
          throw new Error("Cannot send message: conversation not ready");
        }

        const chatResponse = await chatAPI.sendMessage(
          conversationId,
          activeMessageId,
          model,
          message,
          webSearch,
        );

        manager.updateWithChatResponse(conversationId, chatResponse.messages);
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

  const getCurrentMessages = useCallback(
    (conversation: ClientConversation): FrontendMessage[] => {
      return conversation.messages || [];
    },
    [],
  );

  const selectConversation = useCallback((conversationId: string) => {
    setActiveConversationId(conversationId);
  }, []);

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

  return {
    conversations,
    activeConversationId,
    currentConversation,
    isLoading,
    error,
    sendMessage,
    getCurrentMessages,
    selectConversation,
    startNewChat,
    clearError,
    hasPendingMessages,
    deleteConversation,
    renameConversation,
  };
};
