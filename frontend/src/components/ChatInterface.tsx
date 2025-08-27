import { useState, useEffect, useRef, useMemo, useCallback } from "react";
import { useModels } from "@/hooks/useModels";
import { useSettings } from "@/hooks/useSettings";

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

import {
  Conversation,
  ConversationContent,
  ConversationScrollButton,
} from "@/components/ai-elements/conversation";
import {
  GlobeIcon,
  AlertCircleIcon,
  RotateCcwIcon,
  CopyIcon,
  EditIcon,
  CheckIcon,
  XIcon,
} from "lucide-react";
import { Loader } from "@/components/ai-elements/loader";
import { Actions, Action } from "@/components/ai-elements/actions";
import { FrontendMessage } from "@/lib/api/types";
import { BranchNavigation } from "@/components/BranchNavigation";
import { ClientConversation } from "@/lib/clientConversationManager";
import {
  EditableMessage,
  EditableMessageRef,
} from "@/components/ai-elements/editable-message";

// Dynamic models are now loaded from providers via useModels hook

interface ChatInterfaceProps {
  messages: FrontendMessage[];
  webSearch: boolean;
  currentConversation: ClientConversation | undefined;
  onWebSearchToggle: (enabled: boolean) => void;
  onSendMessage: (
    message: string,
    webSearch: boolean,
    model: string,
  ) => Promise<void>;
  onRetryMessage: (messageId: string, model: string) => Promise<void>;
  onSwitchBranch: (messageId: number, branchIndex: number) => void;
  getBranchInfo: (messageId: number) => {
    count: number;
    activeIndex: number;
    hasMultiple: boolean;
  };
  onUpdateMessage: (messageId: string, newContent: string) => Promise<void>;
}

export const ChatInterface = ({
  messages,
  webSearch,
  currentConversation,
  onWebSearchToggle,
  onSendMessage,
  onRetryMessage,
  onSwitchBranch,
  getBranchInfo,
  onUpdateMessage,
}: ChatInterfaceProps) => {
  const { models, isLoading: modelsLoading } = useModels();
  const {
    updateSingleSetting,
    getSingleSetting,
    isLoading: settingsLoading,
  } = useSettings();
  const [model, setModel] = useState<string>("");
  const [input, setInput] = useState("");
  const [retryingMessageId, setRetryingMessageId] = useState<string | null>(
    null,
  );
  const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
  const [updatingMessageId, setUpdatingMessageId] = useState<string | null>(
    null,
  );
  const editableMessageRefs = useRef<Record<string, EditableMessageRef | null>>(
    {},
  );

  /**
   * Initialize model selection with persistence
   * Prioritizes saved user preference, falls back to first available model
   * Ensures model choice persists across page refreshes and sessions
   */
  useEffect(() => {
    if (models.length > 0 && !model && !settingsLoading) {
      const savedModel = getSingleSetting("defaultModel");

      // Check if saved model is still available in current providers
      const isModelAvailable =
        savedModel && models.find((m) => m.id === savedModel);

      if (isModelAvailable) {
        // Use saved model if it exists in available models
        setModel(savedModel);
      } else {
        // Fallback to first available model when:
        // - No saved model exists (backend doesn't have defaultModel setting)
        // - Saved model is no longer available (provider removed/changed)
        const fallbackModel = models[0].id;
        setModel(fallbackModel);

        // Create/update the default model setting in backend
        // This handles the case where backend doesn't have defaultModel setting yet
        updateSingleSetting("defaultModel", fallbackModel).catch((error) => {
          console.error(
            "Failed to create/update default model setting:",
            error,
          );
        });

        // Log when we fall back due to unavailable model (but not for missing setting)
        if (savedModel && !isModelAvailable) {
          console.warn(
            `Saved model "${savedModel}" is no longer available. Falling back to "${fallbackModel}".`,
          );
        }
      }
    }
  }, [models, model, settingsLoading, getSingleSetting, updateSingleSetting]);

  /**
   * Handle model selection change and persist to settings
   * Updates both local state and saves preference to backend
   * This ensures the model choice is remembered across sessions
   */
  const handleModelChange = async (newModel: string) => {
    setModel(newModel);
    try {
      await updateSingleSetting("defaultModel", newModel);
    } catch (error) {
      console.error("Failed to save model preference:", error);
    }
  };

  // Check if there are any pending messages
  const hasPendingMessages = messages.some(
    (message) => message.status === "pending",
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim()) return;

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
      await onRetryMessage(messageId, model);
    } finally {
      setRetryingMessageId(null);
    }
  };

  const handleUpdateMessage = async (messageId: string, newContent: string) => {
    setUpdatingMessageId(messageId);
    try {
      await onUpdateMessage(messageId, newContent);
      setEditingMessageId(null);
    } catch (error) {
      console.error("Failed to update message:", error);
    } finally {
      setUpdatingMessageId(null);
    }
  };

  // Create a simple cache for branch info that only updates when conversation data changes
  const branchInfoCache = useMemo(() => {
    const cache = new Map<string, { branchInfo: any; parentId: number }>();

    if (!currentConversation?.backendConversation) {
      return cache;
    }

    // Pre-compute branch info for all messages to avoid repeated calculations
    for (const message of messages) {
      if (message.role === "assistant") {
        const messageId = parseInt(message.id);
        const assistantMessage =
          currentConversation.backendConversation.messages[messageId];

        if (assistantMessage?.parentId) {
          const parentId = assistantMessage.parentId;
          const branchInfo = getBranchInfo(parentId);
          cache.set(message.id, { branchInfo, parentId });
        } else {
          cache.set(message.id, {
            branchInfo: { count: 1, activeIndex: 0, hasMultiple: false },
            parentId: messageId,
          });
        }
      } else {
        cache.set(message.id, {
          branchInfo: { count: 1, activeIndex: 0, hasMultiple: false },
          parentId: parseInt(message.id),
        });
      }
    }

    return cache;
  }, [
    messages,
    currentConversation?.backendConversation?.messages,
    currentConversation?.activeBranches,
    getBranchInfo,
  ]);

  const renderMessageActions = useCallback(
    (message: FrontendMessage) => {
      const messageInfo = branchInfoCache.get(message.id);
      if (!messageInfo) return null;

      const { branchInfo, parentId } = messageInfo;

      return (
        <div className="flex items-center gap-2">
          {/* Action buttons */}
          <Actions className="opacity-60 hover:opacity-100 transition-opacity">
            {message.status !== "pending" &&
              editingMessageId !== message.id && (
                <Action
                  tooltip="Edit message"
                  onClick={() => setEditingMessageId(message.id)}
                  disabled={updatingMessageId === message.id}
                >
                  <EditIcon className="size-4" />
                </Action>
              )}

            {editingMessageId === message.id && (
              <>
                <Action
                  tooltip="Save changes"
                  onClick={() =>
                    editableMessageRefs.current[message.id]?.triggerSave()
                  }
                  disabled={updatingMessageId === message.id}
                >
                  <CheckIcon className="size-4" />
                </Action>
                <Action
                  tooltip="Cancel editing"
                  onClick={() => setEditingMessageId(null)}
                  disabled={updatingMessageId === message.id}
                >
                  <XIcon className="size-4" />
                </Action>
              </>
            )}

            <Action
              tooltip={
                message.status === "error" ? "Copy error" : "Copy message"
              }
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

            {message.role === "assistant" && message.status !== "pending" && (
              <Action
                tooltip={
                  message.status === "error"
                    ? "Retry getting response"
                    : "Regenerate response"
                }
                onClick={() => handleRetryMessage(message.id)}
                disabled={retryingMessageId === message.id}
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
          </Actions>

          {/* Branch navigation for assistant messages with multiple branches */}
          {message.role === "assistant" && branchInfo.hasMultiple && (
            <BranchNavigation
              currentIndex={branchInfo.activeIndex}
              totalCount={branchInfo.count}
              onPrevious={() => {
                const newIndex = Math.max(0, branchInfo.activeIndex - 1);
                onSwitchBranch(parentId, newIndex);
              }}
              onNext={() => {
                const newIndex = Math.min(
                  branchInfo.count - 1,
                  branchInfo.activeIndex + 1,
                );
                onSwitchBranch(parentId, newIndex);
              }}
            />
          )}
        </div>
      );
    },
    [
      branchInfoCache,
      editingMessageId,
      updatingMessageId,
      retryingMessageId,
      onSwitchBranch,
      handleRetryMessage,
      copyMessage,
      setEditingMessageId,
      editableMessageRefs,
    ],
  );

  const renderMessageContent = (message: FrontendMessage) => {
    if (message.status === "error") {
      return (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-destructive">
            <AlertCircleIcon className="size-4 flex-shrink-0" />
            <span className="font-medium">
              {message.role === "assistant"
                ? "Failed to get response"
                : "Failed to send message"}
            </span>
          </div>
          <div className="text-sm text-destructive/80">
            {message.error || "An unknown error occurred"}
          </div>
        </div>
      );
    }

    if (
      message.status === "pending" &&
      message.role === "assistant" &&
      message.content === ""
    ) {
      return (
        <div className="flex items-center gap-2 text-muted-foreground">
          <Loader size={16} />
          <span className="text-sm">Thinking...</span>
        </div>
      );
    }

    return (
      <EditableMessage
        ref={(ref) => {
          if (ref) {
            editableMessageRefs.current[message.id] = ref;
          }
        }}
        content={message.content}
        isEditing={editingMessageId === message.id}
        onSave={(newContent) => handleUpdateMessage(message.id, newContent)}
        onCancel={() => setEditingMessageId(null)}
        disabled={updatingMessageId === message.id}
      />
    );
  };

  const renderMessage = (message: FrontendMessage) => {
    return (
      <div key={message.id}>
        <MessageComponent
          from={message.role}
          status={message.status}
          className={message.role === "user" ? "pb-1" : ""}
        >
          <MessageContent>
            {message.role === "user" ? (
              renderMessageContent(message)
            ) : (
              <div className="space-y-4">
                {renderMessageContent(message)}
                {message.status !== "pending" && renderMessageActions(message)}
              </div>
            )}
          </MessageContent>
        </MessageComponent>

        {message.role === "user" && message.status !== "pending" && (
          <div className="flex justify-end">
            {renderMessageActions(message)}
          </div>
        )}
      </div>
    );
  };

  // Simple scroll to bottom when messages change
  useEffect(() => {
    const timer = setTimeout(() => {
      const scrollContainer = document.querySelector('[role="log"]');
      if (scrollContainer) {
        scrollContainer.scrollTop = scrollContainer.scrollHeight;
      }
    }, 50);
    return () => clearTimeout(timer);
  }, [messages]);

  return (
    <div className="flex-1 flex flex-col min-h-0">
      <Conversation className="flex-1">
        <ConversationContent className="chat-interface w-full max-w-3xl mx-auto px-4 sm:px-6">
          {messages.length === 0 ? (
            <div className="h-full flex items-center justify-center">
              <Welcome />
            </div>
          ) : (
            <div className="space-y-4">
              {messages.map((message) => renderMessage(message))}
            </div>
          )}
        </ConversationContent>
        <ConversationScrollButton />
      </Conversation>

      <div className="flex-shrink-0 flex justify-center p-6 pt-4">
        <PromptInput
          onSubmit={handleSubmit}
          className="chat-interface w-full max-w-3xl mx-auto px-4 sm:px-6"
        >
          <PromptInputTextarea
            onChange={(e) => setInput(e.target.value)}
            value={input}
            placeholder="Ask anything here ..."
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
                onValueChange={handleModelChange}
                value={model}
                disabled={
                  modelsLoading || models.length === 0 || settingsLoading
                }
              >
                <PromptInputModelSelectTrigger>
                  <PromptInputModelSelectValue
                    placeholder={
                      modelsLoading ? "Loading models..." : "Select a model"
                    }
                  />
                </PromptInputModelSelectTrigger>
                <PromptInputModelSelectContent>
                  {models.length === 0 ? (
                    <div className="px-3 py-4 text-center">
                      <div className="text-sm text-muted-foreground">
                        No models available
                      </div>
                      <div className="text-xs text-muted-foreground mt-1">
                        Add AI providers in settings
                      </div>
                    </div>
                  ) : (
                    <div className="space-y-1 p-1">
                      {models.map((modelItem) => (
                        <PromptInputModelSelectItem
                          key={modelItem.id}
                          value={modelItem.id}
                        >
                          {modelItem.name}
                        </PromptInputModelSelectItem>
                      ))}
                      {models.length > 5 && (
                        <div className="border-t pt-2 mt-2">
                          <div className="text-xs text-muted-foreground text-center px-2">
                            {models.length} models available
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </PromptInputModelSelectContent>
              </PromptInputModelSelect>
            </PromptInputTools>
            <PromptInputSubmit
              disabled={!input || !model || models.length === 0}
              status={hasPendingMessages ? "submitted" : undefined}
            />
          </PromptInputToolbar>
        </PromptInput>
      </div>
    </div>
  );
};
