import { useState } from "react";
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
  const [sidebarCollapsed, setSidebarCollapsed] = useState(
    window.innerWidth < 768,
  );

  const [isProcessing, setIsProcessing] = useState(false);
  const [lastMessageContent, setLastMessageContent] = useState<string>("");
  const [lastMessageTime, setLastMessageTime] = useState<number>(0);

  // Conversation management with branching support
  const {
    conversations,
    currentConversation,
    activeConversationId,
    createConversation,
    addMessage,
    addBranchMessage,
    setActiveMessage,
    selectConversation,
    startNewChat,
    getCurrentMessages,
    getBranchInfo,
  } = useConversations();

  // Chat API
  const { sendMessage } = useChat();

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

    // Create conversation if none exists and get the ID
    let conversationId = activeConversationId;
    if (!conversationId) {
      conversationId = createConversation(message);
    }

    console.log("🚀 Starting message send...", { conversationId, message });
    setIsLoading(true);

    try {
      // Get current conversation context
      const conversation = conversations.find((c) => c.id === conversationId);
      const currentMessages = conversation
        ? getCurrentMessages(conversation)
        : [];

      console.log("📊 Current context:", {
        foundConversation: !!conversation,
        messageCount: currentMessages.length,
        conversationId,
        activeConversationId,
      });

      // Prepare API call with current context + new user message
      const apiMessages = [
        ...currentMessages,
        {
          id: "",
          role: "user" as const,
          content: message,
          timestamp: Date.now(),
        },
      ];

      console.log("📤 Calling API with", apiMessages.length, "messages");

      // Get AI response
      const assistantMessage = await sendMessage(
        apiMessages,
        model,
        webSearchEnabled,
      );

      console.log("📥 Got response:", assistantMessage.content.slice(0, 50));

      // Add both messages to the conversation
      const userMsgId = addMessage(conversationId, message, "user");
      const assistantMsgId = addMessage(
        conversationId,
        assistantMessage.content,
        "assistant",
      );

      console.log("💾 Added messages:", {
        userMsgId,
        assistantMsgId,
        conversationId,
        userMsgContent: message.slice(0, 50),
        assistantMsgContent: assistantMessage.content.slice(0, 50),
        totalMessagesAfter: getCurrentMessages(
          conversations.find((c) => c.id === conversationId) ||
            ({ branchingConversation: { getActivePath: () => [] } } as any),
        ).length,
      });
    } catch (error) {
      console.error("❌ Error:", error);
      const errorMsg =
        error instanceof Error ? error.message : "An unknown error occurred";

      // Add user message and error response
      const errorUserMsgId = addMessage(conversationId, message, "user");
      const errorAssistantMsgId = addMessage(
        conversationId,
        "",
        "assistant",
        "error",
        errorMsg,
      );
      console.log("❌ Added error messages:", {
        errorUserMsgId,
        errorAssistantMsgId,
        conversationId,
        error: errorMsg,
      });
    } finally {
      setIsLoading(false);
      setIsProcessing(false);
      console.log("🏁 Send complete", {
        conversationId,
        totalConversations: conversations.length,
        currentMessageCount: currentConversation
          ? getCurrentMessages(currentConversation).length
          : 0,
      });
    }
  };

  const handleRetryMessage = async (messageId: string) => {
    if (!activeConversationId || !currentConversation) return;

    setIsLoading(true);
    try {
      // Get the message to retry
      const failedMessage =
        currentConversation.branchingConversation.getMessage(messageId);

      if (!failedMessage || !failedMessage.parentId) {
        console.error("Cannot retry message: no parent found");
        return;
      }

      console.log("🔄 Retrying message:", {
        messageId,
        role: failedMessage.role,
        content: failedMessage.content.slice(0, 50),
        parentId: failedMessage.parentId,
      });

      // Get all messages up to the parent (for context)
      const activePath =
        currentConversation.branchingConversation.getActivePath();
      const parentIndex = activePath.findIndex(
        (msg) => msg.id === failedMessage.parentId,
      );

      if (parentIndex === -1) {
        console.error("Parent message not found in active path");
        return;
      }

      const contextMessages = activePath
        .slice(0, parentIndex + 1)
        .map((msg) => ({
          id: msg.id,
          role: msg.role,
          content: msg.content,
          timestamp: msg.timestamp,
          status: msg.status,
          error: msg.error,
        }));

      console.log("📤 Retry API call with context:", {
        contextLength: contextMessages.length,
        lastContext: contextMessages[contextMessages.length - 1]?.content.slice(
          0,
          50,
        ),
      });

      // Send to API
      const response = await sendMessage(
        contextMessages,
        "openai/gpt-4o",
        webSearch,
      );

      // Create a branch with the new response
      const newMessageId = addBranchMessage(
        activeConversationId,
        response.content,
        "assistant",
        failedMessage.parentId,
        "success",
      );

      console.log("✅ Retry successful, created branch:", {
        newMessageId,
        responseContent: response.content.slice(0, 50),
        totalBranches:
          currentConversation.branchingConversation.getTotalBranches(
            newMessageId || "",
          ),
      });
    } catch (error) {
      console.error("Retry failed:", error);
      const errorMsg = error instanceof Error ? error.message : "Retry failed";

      // For retry errors, also create a branch with error message
      const failedMessage =
        currentConversation.branchingConversation.getMessage(messageId);
      if (failedMessage && failedMessage.parentId) {
        const errorBranchId = addBranchMessage(
          activeConversationId,
          "",
          "assistant",
          failedMessage.parentId,
          "error",
          errorMsg,
        );
        console.log("❌ Retry failed, created error branch:", {
          errorBranchId,
          error: errorMsg,
        });
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleNewChat = () => {
    startNewChat();
    setWebSearch(false);
  };

  const handleConversationSelect = (conversationId: string) => {
    console.log("🔄 Switching conversation:", {
      from: activeConversationId,
      to: conversationId,
      isProcessing,
    });
    selectConversation(conversationId);
  };

  const handleToggleSidebar = () => {
    setSidebarCollapsed(!sidebarCollapsed);
  };

  const handleWebSearchToggle = (enabled: boolean) => {
    setWebSearch(enabled);
  };

  const handleBranchChange = (messageId: string) => {
    if (activeConversationId) {
      setActiveMessage(activeConversationId, messageId);
    }
  };

  const getBranchInfoForMessage = (messageId: string) => {
    if (!activeConversationId) return null;
    return getBranchInfo(activeConversationId, messageId);
  };

  // Compute current messages on every render
  const currentMessages = currentConversation
    ? getCurrentMessages(currentConversation)
    : [];

  // Debug logging for message state
  console.log("🔍 App render state:", {
    activeConversationId,
    messagesCount: currentMessages.length,
    isLoading,
    isProcessing,
    lastMessageContent: lastMessageContent.slice(0, 30),
    conversations: conversations.map((c) => ({ id: c.id, title: c.title })),
  });

  return (
    <div className="flex h-screen relative">
      {/* Overlay for mobile */}
      {!sidebarCollapsed && window.innerWidth < 768 && (
        <div className="sidebar-overlay" onClick={handleToggleSidebar} />
      )}

      {/* Sidebar */}
      <ConversationSidebar
        conversations={conversations}
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

        <ChatInterface
          messages={currentMessages}
          isLoading={isLoading}
          webSearch={webSearch}
          onWebSearchToggle={handleWebSearchToggle}
          onSendMessage={handleSendMessage}
          onRetryMessage={handleRetryMessage}
          getBranchInfo={getBranchInfoForMessage}
          onBranchChange={handleBranchChange}
        />
      </div>
    </div>
  );
}

export default App;
