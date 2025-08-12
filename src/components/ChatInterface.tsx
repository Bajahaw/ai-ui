import { useState } from "react";
import {
  Conversation,
  ConversationContent,
  ConversationScrollButton,
} from "@/components/ai-elements/conversation";

import {
  Message as MessageComponent,
  MessageContent,
} from "@/components/ai-elements/message";
import {
  PromptInput,
  PromptInputButton,
  PromptInputModelSelect,
  PromptInputModelSelectContent,
  PromptInputModelSelectItem,
  PromptInputModelSelectTrigger,
  PromptInputModelSelectValue,
  PromptInputSubmit,
  PromptInputTextarea,
  PromptInputToolbar,
  PromptInputTools,
} from "@/components/ai-elements/prompt-input";
import { Welcome } from "@/components/ai-elements/welcome";
import { Response } from "@/components/ai-elements/response";
import {
  GlobeIcon,
  AlertCircleIcon,
  RotateCcwIcon,
  CopyIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
} from "lucide-react";
import { Loader } from "@/components/ai-elements/loader";
import { Actions, Action } from "@/components/ai-elements/actions";
import { Message } from "@/hooks/useConversations";

const models = [
  {
    name: "GPT 4o",
    value: "openai/gpt-4o",
  },
  {
    name: "Deepseek R1",
    value: "deepseek/deepseek-r1",
  },
];

interface ChatInterfaceProps {
  messages: Message[];
  isLoading: boolean;
  webSearch: boolean;
  onWebSearchToggle: (enabled: boolean) => void;
  onSendMessage: (
    message: string,
    webSearch: boolean,
    model: string,
  ) => Promise<void>;
  onRetryMessage: (messageId: string) => Promise<void>;
  // New props for branching
  getBranchInfo?: (messageId: string) => {
    hasBranches: boolean;
    currentIndex: number;
    totalBranches: number;
    branches: Message[];
  } | null;
  onBranchChange?: (messageId: string) => void;
  // Props for dynamic width adjustment
  sidebarCollapsed: boolean;
  sidebarWidth: number;
  isMobile: boolean;
}

export const ChatInterface = ({
  messages,
  isLoading,
  webSearch,
  onWebSearchToggle,
  onSendMessage,
  onRetryMessage,
  getBranchInfo,
  onBranchChange,
  sidebarCollapsed,
  sidebarWidth,
  isMobile,
}: ChatInterfaceProps) => {
  const [model, setModel] = useState<string>(models[0].value);
  const [input, setInput] = useState("");
  const [retryingMessageId, setRetryingMessageId] = useState<string | null>(
    null,
  );

  // Debug logging
  console.log("🎨 ChatInterface render:", {
    messagesCount: messages.length,
    isLoading,
    messages: messages.map((m) => ({
      id: m.id,
      role: m.role,
      content: m.content.slice(0, 50),
    })),
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;

    const message = input;
    setInput("");
    await onSendMessage(message, webSearch, model);
  };

  const copyMessage = async (content: string) => {
    try {
      await navigator.clipboard.writeText(content);
    } catch (error) {
      console.error("Failed to copy message:", error);
    }
  };

  const handleRetryMessage = async (messageId: string) => {
    setRetryingMessageId(messageId);
    try {
      await onRetryMessage(messageId);
    } finally {
      setRetryingMessageId(null);
    }
  };

  const handleBranchChange = (messageId: string, branchIndex: number) => {
    if (onBranchChange) {
      const branchInfo = getBranchInfo?.(messageId);
      if (branchInfo && branchInfo.branches[branchIndex]) {
        const targetBranchMessageId = branchInfo.branches[branchIndex].id;
        console.log("🔀 ChatInterface branch change:", {
          fromMessageId: messageId,
          toBranchIndex: branchIndex,
          targetMessageId: targetBranchMessageId,
          totalBranches: branchInfo.totalBranches,
          branchContent: branchInfo.branches[branchIndex].content.slice(0, 50),
        });
        onBranchChange(targetBranchMessageId);
      }
    }
  };

  const renderMessageActions = (message: Message) => {
    const branchInfo = getBranchInfo?.(message.id);
    const hasBranches = branchInfo && branchInfo.totalBranches > 1;

    return (
      <Actions className="opacity-60 hover:opacity-100 transition-opacity">
        <Action
          tooltip={message.status === "error" ? "Copy error" : "Copy message"}
          onClick={() =>
            copyMessage(
              message.status === "error"
                ? message.error || "Error occurred"
                : message.content,
            )
          }
        >
          <CopyIcon className="size-4" />
        </Action>

        {message.role === "assistant" && (
          <Action
            tooltip={
              message.status === "error"
                ? "Retry sending this message"
                : "Regenerate response"
            }
            onClick={() => handleRetryMessage(message.id)}
            disabled={isLoading || retryingMessageId === message.id}
            className={
              message.status === "error"
                ? "text-destructive hover:text-destructive-foreground hover:bg-destructive"
                : ""
            }
          >
            {retryingMessageId === message.id ? (
              <div className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
            ) : (
              <RotateCcwIcon className="size-4" />
            )}
          </Action>
        )}

        {/* Branch navigation controls */}
        {hasBranches && branchInfo && (
          <>
            <Action
              tooltip="Previous branch"
              onClick={() => {
                const prevIndex =
                  branchInfo.currentIndex === 0
                    ? branchInfo.totalBranches - 1
                    : branchInfo.currentIndex - 1;
                const targetBranchMessageId =
                  branchInfo.branches[prevIndex]?.id;
                console.log("⬅️ Previous branch clicked:", {
                  messageId: message.id,
                  currentIndex: branchInfo.currentIndex,
                  prevIndex,
                  targetMessageId: targetBranchMessageId,
                });
                if (targetBranchMessageId) {
                  handleBranchChange(message.id, prevIndex);
                }
              }}
            >
              <ChevronLeftIcon className="size-4" />
            </Action>

            <span className="text-xs text-muted-foreground px-1">
              {branchInfo.currentIndex + 1} of {branchInfo.totalBranches}
            </span>

            <Action
              tooltip="Next branch"
              onClick={() => {
                const nextIndex =
                  branchInfo.currentIndex === branchInfo.totalBranches - 1
                    ? 0
                    : branchInfo.currentIndex + 1;
                const targetBranchMessageId =
                  branchInfo.branches[nextIndex]?.id;
                console.log("➡️ Next branch clicked:", {
                  messageId: message.id,
                  currentIndex: branchInfo.currentIndex,
                  nextIndex,
                  targetMessageId: targetBranchMessageId,
                });
                if (targetBranchMessageId) {
                  handleBranchChange(message.id, nextIndex);
                }
              }}
            >
              <ChevronRightIcon className="size-4" />
            </Action>
          </>
        )}
      </Actions>
    );
  };

  const renderMessageContent = (message: Message) => {
    if (message.status === "error") {
      return (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-destructive">
            <AlertCircleIcon className="size-4 flex-shrink-0" />
            <span className="font-medium">Failed to send message</span>
          </div>
          <div className="text-sm text-destructive/80">
            {message.error || "An unknown error occurred"}
          </div>
        </div>
      );
    }

    return <Response>{message.content}</Response>;
  };

  const renderMessage = (message: Message) => {
    // Regular message rendering - branch controls are now in actions
    return (
      <div key={message.id}>
        <MessageComponent
          from={message.role}
          status={message.status}
          className={message.role === "user" ? "pb-1" : ""}
        >
          <MessageContent>
            {message.role === "user" ? (
              // User messages: content only, actions below
              renderMessageContent(message)
            ) : (
              // Assistant messages: content and actions together
              <div className="space-y-4">
                {renderMessageContent(message)}
                {renderMessageActions(message)}
              </div>
            )}
          </MessageContent>
        </MessageComponent>

        {/* Actions for user messages appear below the message bubble */}
        {message.role === "user" && (
          <div className="flex justify-end">
            {renderMessageActions(message)}
          </div>
        )}
      </div>
    );
  };

  // Dynamic width based on sidebar state and actual width
  const getMaxWidth = () => {
    if (isMobile) return "max-w-4xl"; // Full width on mobile since sidebar is overlay

    // Calculate available width based on viewport and sidebar
    const viewportWidth = window.innerWidth;
    const availableWidth = sidebarCollapsed
      ? viewportWidth
      : viewportWidth - sidebarWidth;
    const maxChatWidth = Math.min(availableWidth * 0.9, 1280); // Max 1280px or 90% of available space

    return sidebarCollapsed ? "max-w-6xl" : "";
  };

  // Get inline style for custom width when sidebar is open
  const getChatStyle = () => {
    if (isMobile || sidebarCollapsed) return {};

    const viewportWidth = window.innerWidth;
    const availableWidth = viewportWidth - sidebarWidth;
    const maxChatWidth = Math.min(availableWidth * 0.9, 1280);

    return { maxWidth: `${maxChatWidth}px` };
  };

  return (
    <div className="flex-1 flex flex-col min-h-0">
      {/* Scrollable conversation area */}
      <div className="flex-1 flex justify-center min-h-0 overflow-y-auto">
        <div
          className={`w-full ${getMaxWidth()} px-6 py-4`}
          style={getChatStyle()}
        >
          {messages.length === 0 ? (
            <div className="h-full flex items-center justify-center">
              <Welcome />
            </div>
          ) : (
            <div className="space-y-4">
              {messages.map((message) => renderMessage(message))}
              {isLoading && !retryingMessageId && <Loader />}
            </div>
          )}
        </div>
      </div>

      {/* Fixed prompt area at bottom */}
      <div className="flex-shrink-0 flex justify-center p-6 pt-4">
        <PromptInput
          onSubmit={handleSubmit}
          className={`w-full ${getMaxWidth()}`}
          style={getChatStyle()}
        >
          <PromptInputTextarea
            onChange={(e) => setInput(e.target.value)}
            value={input}
            placeholder="Type your message..."
          />
          <PromptInputToolbar>
            <PromptInputTools>
              <PromptInputButton
                variant={webSearch ? "default" : "ghost"}
                onClick={() => onWebSearchToggle(!webSearch)}
              >
                <GlobeIcon size={16} />
                <span>Search</span>
              </PromptInputButton>
              <PromptInputModelSelect
                onValueChange={(value) => {
                  setModel(value);
                }}
                value={model}
              >
                <PromptInputModelSelectTrigger>
                  <PromptInputModelSelectValue />
                </PromptInputModelSelectTrigger>
                <PromptInputModelSelectContent>
                  {models.map((model) => (
                    <PromptInputModelSelectItem
                      key={model.value}
                      value={model.value}
                    >
                      {model.name}
                    </PromptInputModelSelectItem>
                  ))}
                </PromptInputModelSelectContent>
              </PromptInputModelSelect>
            </PromptInputTools>
            <PromptInputSubmit disabled={!input || isLoading} />
          </PromptInputToolbar>
        </PromptInput>
      </div>
    </div>
  );
};
