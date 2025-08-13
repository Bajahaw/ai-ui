import { useState } from "react";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { ConversationSidebar } from "@/components/ai-elements/conversation-sidebar";
import { ChatInterface } from "@/components/ChatInterface";
import { useConversations } from "@/hooks/useConversations";

import { MessageSquareIcon } from "lucide-react";

function App() {
  const [webSearch, setWebSearch] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(
    window.innerWidth < 768,
  );

  const [isProcessing, setIsProcessing] = useState(false);
  const [lastMessageContent, setLastMessageContent] = useState<string>("");
  const [lastMessageTime, setLastMessageTime] = useState<number>(0);

  // Conversation management with optimistic updates
  const {
    conversations,
    currentConversation,
    activeConversationId,
    createConversation,
    sendMessage: sendChatMessage,
    selectConversation,
    startNewChat,
    getCurrentMessages,
    isLoading: conversationsLoading,
    error: conversationsError,
    clearError,
  } = useConversations();

  const handleSendMessage = async (
    message: string,
    webSearchEnabled: boolean,
    model: string,
  ) => {
    // Enhanced duplicate prevention for StrictMode and race conditions
    const currentTime = Date.now();
    const timeSinceLastMessage = currentTime - lastMessageTime;

    if (isProcessing) {
      console.log("⚠️ Already processing a message, ignoring duplicate call");
      return;
    }

    // Prevent duplicate messages within 1 second with same content (StrictMode protection)
    if (message === lastMessageContent && timeSinceLastMessage < 1000) {
      console.log("⚠️ Duplicate message detected within 1 second, ignoring");
      return;
    }

    setIsProcessing(true);
    setLastMessageContent(message);
    setLastMessageTime(currentTime);

    try {
      const conversationId = activeConversationId;

      if (!conversationId) {
        await createConversation(message, model, webSearchEnabled);
      } else {
        await sendChatMessage(conversationId, message, model, webSearchEnabled);
      }
    } catch (error) {
      console.error("Error in message flow:", error);
    } finally {
      setIsProcessing(false);
    }
  };

  const handleRetryMessage = async (messageId: string) => {
    if (!activeConversationId || !currentConversation) return;

    try {
      const failedMessage = currentConversation.messages.find(
        (m) => m.id === messageId,
      );
      if (!failedMessage) return;

      if (failedMessage.role === "user") {
        await sendChatMessage(
          activeConversationId,
          failedMessage.content,
          "openai/gpt-4o",
          webSearch,
        );
      }
    } catch (error) {
      console.error("Retry failed:", error);
    }
  };

  const handleNewChat = () => {
    startNewChat();
    setWebSearch(false);
    clearError(); // Clear any existing errors
  };

  const handleConversationSelect = (conversationId: string) => {
    if (isProcessing) return;
    selectConversation(conversationId);
  };

  const handleToggleSidebar = () => {
    setSidebarCollapsed(!sidebarCollapsed);
  };

  const handleWebSearchToggle = (enabled: boolean) => {
    setWebSearch(enabled);
  };

  // Get current messages for display
  const currentMessages = currentConversation
    ? getCurrentMessages(currentConversation)
    : [];

  // Fallback handling for empty or invalid conversations
  const safeConversations = Array.isArray(conversations) ? conversations : [];

  return (
    <div className="flex h-screen relative">
      {/* Overlay for mobile */}
      {!sidebarCollapsed && window.innerWidth < 768 && (
        <div className="sidebar-overlay" onClick={handleToggleSidebar} />
      )}

      {/* Sidebar */}
      <ConversationSidebar
        conversations={safeConversations}
        activeConversationId={activeConversationId}
        onConversationSelect={handleConversationSelect}
        onNewChat={handleNewChat}
        isCollapsed={sidebarCollapsed}
        onToggleCollapse={handleToggleSidebar}
      />

      {/* Main Content */}
      <div className="chat-container">
        <div className="flex justify-between items-center p-6 pb-4">
          <div className="flex items-center gap-3">
            <Button
              variant="ghost"
              size="sm"
              onClick={handleToggleSidebar}
              className="hover:bg-accent"
            >
              <MessageSquareIcon className="size-4" />
            </Button>
            <h1 className="text-2xl font-bold">AI Chat</h1>
          </div>
          <ThemeToggle />
        </div>

        {/* Error Display */}
        {conversationsError && (
          <div className="flex justify-center px-6 mb-4">
            <div className="w-full max-w-3xl p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
              <div className="flex justify-between items-center">
                <p className="text-red-700 dark:text-red-300 text-sm">
                  {conversationsError}
                </p>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={clearError}
                  className="text-red-700 dark:text-red-300 hover:bg-red-100 dark:hover:bg-red-800/30"
                >
                  ✕
                </Button>
              </div>
            </div>
          </div>
        )}

        {/* Loading State */}
        {conversationsLoading && safeConversations.length === 0 && (
          <div className="flex items-center justify-center h-32">
            <div className="text-muted-foreground">
              Loading conversations...
            </div>
          </div>
        )}

        <ChatInterface
          messages={currentMessages}
          webSearch={webSearch}
          onWebSearchToggle={handleWebSearchToggle}
          onSendMessage={handleSendMessage}
          onRetryMessage={handleRetryMessage}
        />
      </div>
    </div>
  );
}

export default App;
