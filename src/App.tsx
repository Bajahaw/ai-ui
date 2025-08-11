import { useState, useEffect, useMemo } from "react";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { ConversationSidebar } from "@/components/ai-elements/conversation-sidebar";
import { ChatInterface } from "@/components/ChatInterface";
import { useConversations } from "@/hooks/useConversations";

import { useChat } from "@/hooks/useChat";
import { MessageSquareIcon } from "lucide-react";

function App() {
  const [isLoading, setIsLoading] = useState(false);
  const [webSearch, setWebSearch] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(true);
  const [isMobile, setIsMobile] = useState(false);

  // Conversation management
  const {
    conversations,
    currentConversation,
    activeConversationId,
    createConversation,
    addMessage,
    replaceLastAssistantMessage,
    selectConversation,
    startNewChat,
    getCurrentMessages,
  } = useConversations();

  // Chat API
  const { sendMessage } = useChat();

  // Detect mobile screen size and set initial sidebar state
  useEffect(() => {
    const checkMobile = () => {
      const mobile = window.innerWidth < 768;
      setIsMobile(mobile);
      if (mobile) {
        setSidebarCollapsed(true);
      }
    };

    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  const handleSendMessage = async (
    message: string,
    webSearchEnabled: boolean,
    model: string,
  ) => {
    // Create conversation if none exists
    let conversationId = activeConversationId;
    if (!conversationId) {
      conversationId = createConversation(message);
    }

    // Add user message
    addMessage(conversationId, message, "user");
    setIsLoading(true);

    try {
      // Get current messages for API call
      const currentMessages = currentConversation
        ? getCurrentMessages(currentConversation)
        : [];

      // Send to API
      const assistantMessage = await sendMessage(
        [
          ...currentMessages,
          {
            id: "",
            role: "user",
            content: message,
            timestamp: Date.now(),
          },
        ],
        model,
        webSearchEnabled,
      );

      // Add assistant message
      addMessage(conversationId, assistantMessage.content, "assistant");
    } catch (error) {
      console.error("Error:", error);
      const errorMsg =
        error instanceof Error ? error.message : "An unknown error occurred";
      addMessage(conversationId, "", "assistant", "error", errorMsg);
    } finally {
      setIsLoading(false);
    }
  };

  const handleRetryMessage = async (messageId: string) => {
    if (!activeConversationId || !currentConversation) return;

    setIsLoading(true);
    try {
      // Get current messages for retry context
      const currentMessages = getCurrentMessages(currentConversation);

      // Find the message to retry and get context up to that point
      const messageIndex = currentMessages.findIndex(
        (msg) => msg.id === messageId,
      );
      const contextMessages = currentMessages.slice(0, messageIndex);

      // Send to API
      const response = await sendMessage(
        contextMessages,
        "openai/gpt-4o",
        webSearch,
      );

      // Replace the failed message with new response
      replaceLastAssistantMessage(activeConversationId, response.content);
    } catch (error) {
      console.error("Retry failed:", error);
      const errorMsg = error instanceof Error ? error.message : "Retry failed";
      replaceLastAssistantMessage(activeConversationId, "", "error", errorMsg);
    } finally {
      setIsLoading(false);
    }
  };

  const handleNewChat = () => {
    startNewChat();
    setWebSearch(false);
  };

  const handleConversationSelect = (conversationId: string) => {
    selectConversation(conversationId);
  };

  const handleToggleSidebar = () => {
    setSidebarCollapsed(!sidebarCollapsed);
  };

  const handleConversationSelectMobile = (conversationId: string) => {
    handleConversationSelect(conversationId);
    if (isMobile) {
      setSidebarCollapsed(true);
    }
  };

  const handleWebSearchToggle = (enabled: boolean) => {
    setWebSearch(enabled);
  };

  return (
    <div className="flex h-screen relative">
      {/* Sidebar */}
      <ConversationSidebar
        conversations={conversations}
        activeConversationId={activeConversationId}
        onConversationSelect={
          isMobile ? handleConversationSelectMobile : handleConversationSelect
        }
        onNewChat={handleNewChat}
        isCollapsed={sidebarCollapsed}
        onToggleCollapse={handleToggleSidebar}
        isMobile={isMobile}
      />

      {/* Main Content */}
      <div className="flex-1 flex flex-col max-w-none">
        <div className="flex justify-between items-center p-6 pb-4 border-b">
          <div className="flex items-center gap-3">
            {isMobile && (
              <Button
                variant="ghost"
                size="sm"
                onClick={handleToggleSidebar}
                className="md:hidden"
              >
                <MessageSquareIcon className="size-4" />
              </Button>
            )}
            <h1 className="text-2xl font-bold">AI Chat</h1>
          </div>
          <ThemeToggle />
        </div>

        <ChatInterface
          messages={useMemo(
            () =>
              currentConversation
                ? getCurrentMessages(currentConversation)
                : [],
            [currentConversation, getCurrentMessages],
          )}
          isLoading={isLoading}
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
