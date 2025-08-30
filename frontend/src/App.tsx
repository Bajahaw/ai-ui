import { useState, useMemo } from "react";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { ConversationSidebar } from "@/components/ai-elements/conversation-sidebar";
import { ChatInterface } from "@/components/ChatInterface";
import { useConversations } from "@/hooks/useConversations";
import { SettingsDialog } from "@/components/settings";

import { MessageSquareIcon, SettingsIcon } from "lucide-react";

// Utility function to extract timestamp from conversation ID (conv-20250815-182253)
// Example: conv-20250815-182253 → August 15, 2025 at 18:22:53 → timestamp for sorting
const getConversationTimestamp = (conversationId: string): number => {
  try {
    const match = conversationId.match(/^conv-(\d{8})-(\d{6})$/);
    if (!match) return 0;

    const [, dateStr, timeStr] = match;
    const year = parseInt(dateStr.substring(0, 4));
    const month = parseInt(dateStr.substring(4, 6)) - 1; // Month is 0-indexed
    const day = parseInt(dateStr.substring(6, 8));
    const hour = parseInt(timeStr.substring(0, 2));
    const minute = parseInt(timeStr.substring(2, 4));
    const second = parseInt(timeStr.substring(4, 6));

    return new Date(year, month, day, hour, minute, second).getTime();
  } catch {
    return 0;
  }
};

function App() {
  const [webSearch, setWebSearch] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(
    window.innerWidth < 768,
  );

  const [isProcessing, setIsProcessing] = useState(false);
  const [lastMessageContent, setLastMessageContent] = useState<string>("");
  const [lastMessageTime, setLastMessageTime] = useState<number>(0);
  const [showSettings, setShowSettings] = useState(false);

  // Conversation management with optimistic updates
  const {
    conversations,
    currentConversation,
    activeConversationId,
    sendMessage: sendChatMessage,
    retryMessage,
    selectConversation,
    startNewChat,
    getCurrentMessages,
    isLoading: conversationsLoading,
    error: conversationsError,
    clearError,
    deleteConversation,
    renameConversation,
    switchBranch,
    getBranchInfo,
    updateMessage,
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
      await sendChatMessage(
        activeConversationId,
        message,
        model,
        webSearchEnabled,
      );
    } catch (error) {
      console.error("Error in message flow:", error);
    } finally {
      setIsProcessing(false);
    }
  };

  const handleRetryMessage = async (messageId: string, model: string) => {
    if (!activeConversationId || !currentConversation) return;

    try {
      const failedMessage = currentConversation.messages.find(
        (m: any) => m.id === messageId,
      );
      if (!failedMessage) return;

      if (failedMessage.role === "user") {
        await sendChatMessage(
          activeConversationId,
          failedMessage.content,
          model,
          webSearch,
        );
      } else if (failedMessage.role === "assistant") {
        // Use retry API for assistant messages
        await retryMessage(messageId, model);
      }
    } catch (error) {
      console.error("Retry failed:", error);
    }
  };

  const handleUpdateMessage = async (messageId: string, newContent: string) => {
    if (!activeConversationId) return;

    try {
      await updateMessage(messageId, newContent);
    } catch (error) {
      console.error("Failed to update message:", error);
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

  const handleDeleteConversation = async (conversationId: string) => {
    try {
      await deleteConversation(conversationId);
    } catch (error) {
      console.error("Failed to delete conversation:", error);
    }
  };

  const handleRenameConversation = async (
    conversationId: string,
    newTitle: string,
  ) => {
    try {
      await renameConversation(conversationId, newTitle);
    } catch (error) {
      console.error("Failed to rename conversation:", error);
    }
  };

  // Get current messages for display
  const currentMessages = currentConversation
    ? getCurrentMessages(currentConversation)
    : [];

  // Fallback handling for empty or invalid conversations and sort by recency
  const safeConversations = useMemo(() => {
    return Array.isArray(conversations)
      ? [...conversations].sort((a, b) => {
          const timestampA = getConversationTimestamp(a.id);
          const timestampB = getConversationTimestamp(b.id);
          return timestampB - timestampA; // Most recent first
        })
      : [];
  }, [conversations]);

  return (
    <div className="flex h-[100dvh] relative overflow-hidden">
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
        onDeleteConversation={handleDeleteConversation}
        onRenameConversation={handleRenameConversation}
        isCollapsed={sidebarCollapsed}
        onToggleCollapse={handleToggleSidebar}
      />

      {/* Main Content */}
      <div className="chat-container overflow-hidden">
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
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowSettings(true)}
              className="hover:bg-accent"
              title="Settings"
            >
              <SettingsIcon className="size-4" />
            </Button>
            <ThemeToggle />
          </div>
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
            currentConversation={currentConversation}
            onWebSearchToggle={handleWebSearchToggle}
            onSendMessage={handleSendMessage}
            onRetryMessage={handleRetryMessage}
            onSwitchBranch={switchBranch}
            getBranchInfo={getBranchInfo}
            onUpdateMessage={handleUpdateMessage}
          />
      </div>

      {/* Settings Dialog */}
      <SettingsDialog open={showSettings} onOpenChange={setShowSettings} />
    </div>
  );
}

export default App;
