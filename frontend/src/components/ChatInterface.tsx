import { useState, useEffect, useRef, useMemo, useCallback } from "react";
import { useModels } from "@/hooks/useModels";
import { useSettings } from "@/hooks/useSettings";
import { useAutoSelectDefaultModel } from "@/hooks/useAutoSelectDefaultModel";

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
import {
  FileUpload,
  FilePreview,
  AttachmentMessage,
  UploadedFile,
} from "@/components/ui/file-upload";

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
    attachment?: string,
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

  // Auto-select default model when models become available
  const { autoSelectedModel } = useAutoSelectDefaultModel();
  const [model, setModel] = useState<string>("");
  const [input, setInput] = useState("");
  const [retryingMessageId, setRetryingMessageId] = useState<string | null>(
    null,
  );
  const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
  const [updatingMessageId, setUpdatingMessageId] = useState<string | null>(
    null,
  );
  const [uploadedFile, setUploadedFile] = useState<UploadedFile | null>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const editableMessageRefs = useRef<Record<string, EditableMessageRef | null>>(
    {},
  );

  /**
   * Synchronize local model state with the default model setting
   * This ensures the prompt input always reflects the current default model
   */
  useEffect(() => {
    if (models.length > 0 && !settingsLoading) {
      const savedModel = getSingleSetting("defaultModel");

      // Check if saved model is still available in current providers
      const isModelAvailable =
        savedModel && models.find((m) => m.id === savedModel);

      if (isModelAvailable && model !== savedModel) {
        // Sync local state with the saved default model
        setModel(savedModel);
      } else if (!savedModel && !model && models.length > 0) {
        // No default model exists and no local model - let auto-select hook handle this
        // This prevents race conditions between auto-select and this component
      } else if (savedModel && !isModelAvailable && models.length > 0) {
        // Saved model is no longer available, update to first available
        const fallbackModel = models[0].id;
        setModel(fallbackModel);
        updateSingleSetting("defaultModel", fallbackModel).catch((error) => {
          console.error("Failed to update default model setting:", error);
        });
        console.warn(
          `Saved model "${savedModel}" is no longer available. Falling back to "${fallbackModel}".`,
        );
      }
    }
  }, [models, settingsLoading, getSingleSetting, updateSingleSetting, model]);

  /**
   * Sync with auto-selected model from the auto-select hook
   * This ensures the prompt input gets updated when auto-select sets a default model
   */
  useEffect(() => {
    if (autoSelectedModel && !model) {
      setModel(autoSelectedModel);
    }
  }, [autoSelectedModel, model]);

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

  // Validate selected model exists in the loaded models list
  const isModelValid = useMemo(() => {
    return !!model && models.some((m) => m.id === model);
  }, [model, models]);

  // Clear inline warning when model becomes valid
  // (no-op — model warning state removed)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() && !uploadedFile) return;

    // Prevent sending when model is not selected or invalid
    if (!isModelValid) {
      return;
    }

    const message = input;
    const attachment = uploadedFile?.url;
    setInput("");
    setUploadedFile(null);
    setUploadError(null);
    await onSendMessage(message, webSearch, model, attachment);
  };

  const handleFileUploaded = useCallback((fileUrl: string, file: File) => {
    setUploadedFile({ file, url: fileUrl });
    setUploadError(null);
  }, []);

  const handleFileUploadError = useCallback((error: string) => {
    setUploadError(error);
    setUploadedFile(null);
  }, []);

  const handleRemoveFile = useCallback(() => {
    setUploadedFile(null);
    setUploadError(null);
  }, []);

  const copyMessage = async (content: string) => {
    try {
      await navigator.clipboard.writeText(content);
    } catch (error) {
      console.error("Failed to copy message:", error);
    }
  };

  const handleRetryMessage = async (messageId: string) => {
    // Prevent retry when model is invalid
    if (!isModelValid) {
      return;
    }

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
        // messages may be undefined until fetched — use optional chaining
        const assistantMessage =
          currentConversation.backendConversation.messages?.[messageId];

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
                disabled={
                  retryingMessageId === message.id ||
                  !isModelValid ||
                  models.length === 0
                }
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
          <div className="text-base text-destructive/80">
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
          <span className="text-base">Thinking...</span>
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
    const hasAttachment =
      message.attachment && message.attachment.trim() !== "";

    return (
      <div key={message.id}>
        {/* Render attachment message first if exists (not counted in conversation tree) */}
        {hasAttachment && (
          <MessageComponent
            from={message.role}
            status="success"
            className="pb-1"
          >
            <MessageContent className="!p-2 ">
              <AttachmentMessage
                attachment={message.attachment!}
                filename={message.attachment!.split("/").pop()}
              />
            </MessageContent>
          </MessageComponent>
        )}

        {/* Render main message */}
        <MessageComponent
          from={message.role}
          status={message.status}
          className={message.role === "user" ? "pb-1" : ""}
        >
          <MessageContent content={message.content}>
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

  return (
    <div className="flex-1 flex flex-col min-h-0">
      <Conversation className="flex-1">
        <ConversationContent className="chat-interface w-full max-w-3xl mx-auto !px-5 lg:!px-3">
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

      <div className="flex-shrink-0 flex justify-center !p-6 !pt-4">
        <PromptInput
          onSubmit={handleSubmit}
          className="chat-interface w-full max-w-3xl mx-auto"
        >
          {/* File preview area */}
          {uploadedFile && (
            <div className="p-3 border-b">
              <FilePreview file={uploadedFile} onRemove={handleRemoveFile} />
            </div>
          )}

          {/* Upload error display */}
          {uploadError && (
            <div className="p-3 border-b">
              <div className="flex items-center gap-2 text-destructive text-base">
                <AlertCircleIcon size={16} />
                <span>{uploadError}</span>
                <button
                  onClick={() => setUploadError(null)}
                  className="ml-auto text-muted-foreground hover:text-foreground"
                >
                  <XIcon size={14} />
                </button>
              </div>
            </div>
          )}

          <PromptInputTextarea
            onChange={(e) => setInput(e.target.value)}
            value={input}
            placeholder="Ask anything here ..."
          />
          <PromptInputToolbar>
            <PromptInputTools>
              <FileUpload
                onFileUploaded={handleFileUploaded}
                onError={handleFileUploadError}
                disabled={hasPendingMessages}
              />
              <PromptInputButton
                variant={webSearch ? "default" : "ghost"}
                onClick={() => onWebSearchToggle(!webSearch)}
              >
                <GlobeIcon size={16} />
                <span className="hidden sm:inline">Search</span>
              </PromptInputButton>

              <PromptInputModelSelect
                onValueChange={handleModelChange}
                value={isModelValid ? model : undefined}
                disabled={
                  modelsLoading || settingsLoading || models.length === 0
                }
              >
                <PromptInputModelSelectTrigger>
                  <PromptInputModelSelectValue
                    placeholder={
                      modelsLoading
                        ? "Loading models..."
                        : models.length === 0
                          ? "No models available"
                          : "Select a model"
                    }
                  />
                </PromptInputModelSelectTrigger>

                <PromptInputModelSelectContent>
                  {models.length === 0 ? (
                    <div className="px-3 py-4 text-center">
                      <div className="text-base text-muted-foreground">
                        No models available
                      </div>
                      <div className="text-xs text-muted-foreground mt-1">
                        Add AI providers in settings
                      </div>
                    </div>
                  ) : (
                    <div className="space-y-1">
                      {models.map((modelItem) => (
                        <PromptInputModelSelectItem
                          key={modelItem.id}
                          value={modelItem.id}
                        >
                          <div className="max-w-[300px] overflow-hidden text-ellipsis text-nowrap pr-2">
                            {modelItem.name}
                          </div>
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
              disabled={
                (!input.trim() && !uploadedFile) ||
                !model ||
                models.length === 0
              }
              status={hasPendingMessages ? "submitted" : undefined}
            />
          </PromptInputToolbar>
        </PromptInput>
      </div>
    </div>
  );
};
