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

  const createConversation = useCallback(
    async (
      firstMessage: string,
      model: string,
      webSearch: boolean = false,
    ): Promise<string> => {
      setError(null);

      // Create conversation optimistically
      const clientConversation = manager.createConversation(firstMessage);
      syncConversations();
      setActiveConversationId(clientConversation.id);

      try {
        // Use merged chat API that handles both conversation creation and first message
        const chatResponse = await chatAPI.sendMessage(
          null, // null conversationId triggers new conversation creation
          null, // null activeMessageId for new conversation
          model, // use the model selected by user
          firstMessage,
          webSearch, // use the webSearch setting from user
          clientConversation.title, // title for new conversation
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
        const conversation = manager.getConversation(clientConversation.id);
        if (conversation) {
          conversation.backendConversation = backendConversation;
        }

        manager.updateWithChatResponse(
          clientConversation.id,
          chatResponse.messages,
        );
        syncConversations();

        return clientConversation.id;
      } catch (error) {
        const tempMessage = clientConversation.messages[0];
        if (tempMessage) {
          const errorMsg = ApiErrorHandler.getUserFriendlyMessage(error);
          manager.markAssistantFailed(
            clientConversation.id,
            tempMessage.id,
            errorMsg,
          );
          syncConversations();
        }
        throw error;
      }
    },
    [manager, syncConversations],
  );

  const sendMessage = useCallback(
    async (
      conversationId: string,
      message: string,
      model: string,
      webSearch: boolean = false,
    ): Promise<void> => {
      let tempMessageId: string | undefined;

      try {
        setError(null);

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
      } catch (err) {
        console.error("Failed to send message:", err);

        if (tempMessageId) {
          const errorMsg = ApiErrorHandler.getUserFriendlyMessage(err);
          manager.markAssistantFailed(conversationId, tempMessageId, errorMsg);
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

  return {
    conversations,
    activeConversationId,
    currentConversation,
    isLoading,
    error,
    createConversation,
    sendMessage,
    getCurrentMessages,
    selectConversation,
    startNewChat,
    clearError,
    hasPendingMessages,
  };
};
