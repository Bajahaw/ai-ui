import { useState, useEffect } from "react";
import { useModels } from "@/hooks/useModels";

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
  Conversation,
  ConversationContent,
  ConversationScrollButton,
} from "@/components/ai-elements/conversation";
import {
  GlobeIcon,
  AlertCircleIcon,
  RotateCcwIcon,
  CopyIcon,
} from "lucide-react";
import { Loader } from "@/components/ai-elements/loader";
import { Actions, Action } from "@/components/ai-elements/actions";
import { FrontendMessage } from "@/lib/api/types";

// Dynamic models are now loaded from providers via useModels hook

interface ChatInterfaceProps {
  messages: FrontendMessage[];
  webSearch: boolean;
  onWebSearchToggle: (enabled: boolean) => void;
  onSendMessage: (
    message: string,
    webSearch: boolean,
    model: string,
  ) => Promise<void>;
  onRetryMessage: (messageId: string) => Promise<void>;
}

export const ChatInterface = ({
  messages,
  webSearch,
  onWebSearchToggle,
  onSendMessage,
  onRetryMessage,
}: ChatInterfaceProps) => {
  const { models, isLoading: modelsLoading, getModelDisplayName } = useModels();
  const [model, setModel] = useState<string>("");
  const [input, setInput] = useState("");
  const [retryingMessageId, setRetryingMessageId] = useState<string | null>(
    null,
  );

  // Set default model when models are loaded
  useEffect(() => {
    if (models.length > 0 && !model) {
      setModel(models[0].id);
    }
  }, [models, model]);

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
      await onRetryMessage(messageId);
    } finally {
      setRetryingMessageId(null);
    }
  };

  const renderMessageActions = (message: FrontendMessage) => {
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
    );
  };

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

    return <Response>{message.content}</Response>;
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
                onValueChange={(value) => {
                  setModel(value);
                }}
                value={model}
                disabled={modelsLoading || models.length === 0}
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
                    <div className="px-2 py-1 text-sm text-muted-foreground">
                      No models available
                    </div>
                  ) : (
                    models.map((modelItem) => (
                      <PromptInputModelSelectItem
                        key={modelItem.id}
                        value={modelItem.id}
                      >
                        {modelItem.name}
                      </PromptInputModelSelectItem>
                    ))
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
