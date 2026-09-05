import { useEffect, useLayoutEffect, useMemo, useState, useRef } from "react";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { ConversationSidebar } from "@/components/ai-elements/conversation-sidebar";
import { ChatInterface } from "@/components/ChatInterface";
import { useConversations } from "@/hooks/useConversations";
import { useAuth } from "@/hooks/useAuth";
import { SettingsDialog } from "@/components/settings";
import { Attachment } from "@/lib/api/types";
import { NavigateFunction, useNavigate, useParams } from "react-router-dom";

import { MessageSquareIcon, SettingsIcon } from "lucide-react";

function historyIdx(): number {
  return typeof window.history.state?.idx === "number"
    ? window.history.state.idx
    : 0;
}

function goToNewChat(navigate: NavigateFunction) {
  if (historyIdx() > 0) {
    navigate(-1);
    return;
  }
  navigate("/", { replace: true });
}

function seedHomeUnderConversation(convId: string) {
  if (historyIdx() > 0) {
    return;
  }
  const state = window.history.state ?? {};
  window.history.replaceState({ ...state, idx: 0 }, "", "/");
  window.history.pushState({ ...state, idx: 1 }, "", `/c/${convId}`);
}

function App() {
  const { isAuthenticated, isCheckingAuth } = useAuth();
  const { convId } = useParams<{ convId?: string }>();
  const navigate = useNavigate();
  const [webSearch, setWebSearch] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(
    window.innerWidth < 768,
  );

  const [isProcessing, setIsProcessing] = useState(false);
  const [lastMessageSignature, setLastMessageSignature] = useState<string>("");
  const [lastMessageTime, setLastMessageTime] = useState<number>(0);
  const [showSettings, setShowSettings] = useState(false);

  // Touch handling for swipe gestures
  const touchStartRef = useRef<number | null>(null);
  const minSwipeDistance = 50;
  const maxEdgeStart = 50; // Only allow opening swipe from the left edge

  const onTouchStart = (e: React.TouchEvent) => {
    touchStartRef.current = e.targetTouches[0].clientX;
  };

  const onTouchEnd = (e: React.TouchEvent) => {
    if (!touchStartRef.current) return;

    const touchEnd = e.changedTouches[0].clientX;
    const distance = touchStartRef.current - touchEnd;
    const isLeftSwipe = distance > minSwipeDistance;
    const isRightSwipe = distance < -minSwipeDistance;
    const startFromLeftEdge = touchStartRef.current < maxEdgeStart;

    // Reset touch start
    touchStartRef.current = null;

    // Only handle gestures on mobile
    if (window.innerWidth >= 768) return;

    if (isRightSwipe && startFromLeftEdge && sidebarCollapsed) {
      setSidebarCollapsed(false);
    } else if (isLeftSwipe && !sidebarCollapsed) {
      setSidebarCollapsed(true);
    }
  };

  // Conversation management with optimistic updates
  const {
    conversations,
    currentConversation,
    activeConversationId,
    sendMessageStream,
    retryMessageStream,
    selectConversation,
    startNewChat,
    getCurrentMessages,
    isLoading: conversationsLoading,
    isConversationLoading,
    hasHydrated,
    error: conversationsError,
    clearError,
    deleteConversation,
    renameConversation,
    switchBranch,
    getBranchInfo,
    updateMessage,
    cancelStream,
    stats,
  } = useConversations();

  useLayoutEffect(() => {
    if (convId) {
      seedHomeUnderConversation(convId);
    }
  }, [convId]);

  useLayoutEffect(() => {
    if (!isAuthenticated) {
      return;
    }

    if (!convId) {
      if (activeConversationId) {
        startNewChat();
      }
      return;
    }

    if (activeConversationId !== convId) {
      selectConversation(convId);
    }
  }, [
    convId,
    activeConversationId,
    isAuthenticated,
    selectConversation,
    startNewChat,
  ]);

  useEffect(() => {
    if (
      !convId ||
      !isAuthenticated ||
      isCheckingAuth ||
      !hasHydrated ||
      conversationsLoading
    ) {
      return;
    }

    const exists = conversations.some(
      (conversation) => conversation.id === convId,
    );
    if (!exists) {
      goToNewChat(navigate);
    }
  }, [
    convId,
    conversations,
    hasHydrated,
    conversationsLoading,
    isAuthenticated,
    isCheckingAuth,
    navigate,
  ]);

  const handleSendMessage = async (
    message: string,
    webSearchEnabled: boolean,
    model: string,
    attachments?: Attachment[],
  ) => {
    // Enhanced duplicate prevention for StrictMode and race conditions
    const currentTime = Date.now();
    const timeSinceLastMessage = currentTime - lastMessageTime;

    if (isProcessing) {
      return;
    }

    // Prevent duplicate messages within 1 second with same content (StrictMode protection)
    const attachmentKey = attachments?.map((a) => a.file.id).join(",") ?? "";
    const messageSignature = `${message}|${attachmentKey}`;
    if (messageSignature === lastMessageSignature && timeSinceLastMessage < 1000) {
      return;
    }

    setIsProcessing(true);
    setLastMessageSignature(messageSignature);
    setLastMessageTime(currentTime);

    try {
      // Use streaming for better UX
      await sendMessageStream(
        activeConversationId,
        message,
        model,
        webSearchEnabled,
        attachments,
        (newConvId) =>
          navigate(`/c/${newConvId}`, { replace: Boolean(convId) }),
      );
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
        await sendMessageStream(
          activeConversationId,
          failedMessage.content,
          model,
          webSearch,
          failedMessage.attachments,
        );
      } else if (failedMessage.role === "assistant") {
        // Use streaming retry for assistant messages
        await retryMessageStream(messageId, model);
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
    if (convId) {
      goToNewChat(navigate);
    }
    setWebSearch(false);
    clearError();
  };

  const handleConversationSelect = (conversationId: string) => {
    if (conversationId === convId) {
      return;
    }
    navigate(`/c/${conversationId}`, { replace: Boolean(convId) });
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
      if (convId === conversationId) {
        goToNewChat(navigate);
      }
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

  // Use convId (URL) as the source of truth for what to display.
  // This prevents the old conversation from briefly flashing during navigation
  // when activeConversationId hasn't been cleared yet by React's state flush.
  const displayedConversation = convId
    ? conversations.find((conversation) => conversation.id === convId)
    : undefined;

  const currentMessages = displayedConversation
    ? getCurrentMessages(displayedConversation)
    : [];

  const isConvPendingRoute =
    !!convId &&
    (!displayedConversation || currentMessages.length === 0) &&
    (isCheckingAuth || conversationsLoading || isConversationLoading || !hasHydrated);

  // Use the ordering provided by the conversation manager directly.
  // The manager tracks createdAt/updatedAt and maintains the intended order,
  // so avoid re-sorting here which can ignore client-side timestamps or temporary ids.
  const safeConversations = useMemo(() => {
    return Array.isArray(conversations) ? [...conversations] : [];
  }, [conversations]);

  return (
    <div
      className="flex fixed inset-0 overflow-hidden overscroll-none"
      onTouchStart={onTouchStart}
      onTouchEnd={onTouchEnd}
    >
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
        isLoading={conversationsLoading}
      />

      {/* Main Content */}
      <div className="chat-container overflow-hidden relative">
        {/* Solid header below lg; floating overlay on lg+ */}
        <div className="flex justify-between items-center p-4 pb-2 shrink-0 lg:absolute lg:top-0 lg:inset-x-0 lg:z-20 lg:pointer-events-none">
          <div className="flex items-center gap-3 lg:pointer-events-auto">
            <Button
              variant="ghost"
              size="sm"
              onClick={handleToggleSidebar}
              className="hover:bg-accent"
            >
              <MessageSquareIcon className="size-4" />
            </Button>
          </div>
          <div className="flex items-center gap-2 lg:pointer-events-auto">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowSettings(true)}
              disabled={!isAuthenticated || isCheckingAuth}
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
          <div className="flex justify-center px-6 mb-4 lg:absolute lg:top-12 lg:inset-x-0 lg:z-20 lg:mb-0 lg:pointer-events-none">
            <div className="w-full max-w-3xl p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg lg:pointer-events-auto lg:shadow-sm">
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

        <ChatInterface
          messages={currentMessages}
          webSearch={webSearch}
          currentConversation={displayedConversation}
          stats={stats}
          isAuthenticated={isAuthenticated}
          isAuthChecking={isCheckingAuth}
          isConversationLoading={isConversationLoading || isConvPendingRoute}
          onWebSearchToggle={handleWebSearchToggle}
          onSendMessage={handleSendMessage}
          onRetryMessage={handleRetryMessage}
          onSwitchBranch={switchBranch}
          getBranchInfo={getBranchInfo}
          onUpdateMessage={handleUpdateMessage}
          onCancelStream={cancelStream}
        />
      </div>

      {/* Settings Dialog */}
      <SettingsDialog open={showSettings} onOpenChange={setShowSettings} />
    </div>
  );
}

export default App;
