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
}

export const ChatInterface = ({
  messages,
  isLoading,
  webSearch,
  onWebSearchToggle,
  onSendMessage,
  onRetryMessage,
}: ChatInterfaceProps) => {
  const [model, setModel] = useState<string>(models[0].value);
  const [input, setInput] = useState("");
  const [retryingMessageId, setRetryingMessageId] = useState<string | null>(
    null,
  );

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

  const renderMessageActions = (message: Message) => (
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
    </Actions>
  );

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

  return (
    <div className="flex-1 flex flex-col p-6 pt-4">
      {messages.length === 0 ? (
        <Welcome />
      ) : (
        <Conversation className="h-full">
          <ConversationContent>
            {messages.map((message) => (
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
            ))}
            {isLoading && !retryingMessageId && <Loader />}
          </ConversationContent>
          <ConversationScrollButton />
        </Conversation>
      )}

      <PromptInput onSubmit={handleSubmit} className="mt-4">
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
  );
};
