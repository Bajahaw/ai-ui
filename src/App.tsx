import { useState } from "react";
import {
  Conversation,
  ConversationContent,
  ConversationScrollButton,
} from "@/components/ai-elements/conversation";
import { Message, MessageContent } from "@/components/ai-elements/message";
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
import { Response } from "@/components/ai-elements/response";
import { GlobeIcon, AlertCircleIcon, RotateCcwIcon } from "lucide-react";
import { Loader } from "@/components/ai-elements/loader";
import { ThemeToggle } from "@/components/theme-toggle";
import { Actions, Action } from "@/components/ai-elements/actions";

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

interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  status?: "success" | "error" | "pending";
  error?: string;
  originalRequest?: string;
}

function App() {
  const [model, setModel] = useState<string>(models[0].value);
  const [webSearch, setWebSearch] = useState(false);
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;

    const userMessage: ChatMessage = {
      id: Date.now().toString(),
      role: "user",
      content: input,
      status: "success",
    };

    setMessages((prev) => [...prev, userMessage]);
    setInput("");
    setIsLoading(true);

    try {
      const response = await fetch("/api/chat", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          messages: [...messages, userMessage]
            .filter((msg) => msg.status !== "error")
            .map((msg) => ({
              role: msg.role,
              content: msg.content,
            })),
          model,
          webSearch,
        }),
      });

      if (!response.ok) {
        const errorText = await response
          .text()
          .catch(() => response.statusText);
        throw new Error(
          `Failed to get response (${response.status}): ${errorText}`,
        );
      }

      const data = await response.json();

      const assistantMessage: ChatMessage = {
        id: Date.now().toString() + "_assistant",
        role: "assistant",
        content:
          data.message?.content ||
          data.message?.parts?.[0]?.text ||
          "Sorry, I couldn't process your request.",
        status: "success",
      };

      setMessages((prev) => [...prev, assistantMessage]);
    } catch (error) {
      console.error("Error:", error);
      let errorMsg = "An unknown error occurred";
      if (error instanceof Error) {
        errorMsg = error.message;
      } else if (typeof error === "string") {
        errorMsg = error;
      }

      const errorMessage: ChatMessage = {
        id: Date.now().toString() + "_error",
        role: "assistant",
        content: "",
        status: "error",
        error: errorMsg,
        originalRequest: input,
      };
      setMessages((prev) => [...prev, errorMessage]);
    } finally {
      setIsLoading(false);
    }
  };

  const retryMessage = async (messageId: string) => {
    const message = messages.find((msg) => msg.id === messageId);
    if (!message || !message.originalRequest) return;

    // Set the input to the original request and trigger submit
    setInput(message.originalRequest);

    // Simulate form submission with the original request
    const userMessage: ChatMessage = {
      id: Date.now().toString(),
      role: "user",
      content: message.originalRequest,
      status: "success",
    };

    setMessages((prev) => [...prev, userMessage]);
    setIsLoading(true);

    try {
      const response = await fetch("/api/chat", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          messages: [...messages, userMessage]
            .filter((msg) => msg.status !== "error")
            .map((msg) => ({
              role: msg.role,
              content: msg.content,
            })),
          model,
          webSearch,
        }),
      });

      if (!response.ok) {
        const errorText = await response
          .text()
          .catch(() => response.statusText);
        throw new Error(`Retry failed (${response.status}): ${errorText}`);
      }

      const data = await response.json();

      const assistantMessage: ChatMessage = {
        id: Date.now().toString() + "_assistant",
        role: "assistant",
        content:
          data.message?.content ||
          data.message?.parts?.[0]?.text ||
          "Sorry, I couldn't process your request.",
        status: "success",
      };

      setMessages((prev) => [...prev, assistantMessage]);
    } catch (error) {
      console.error("Retry error:", error);
      let errorMsg = "Retry failed - An unknown error occurred";
      if (error instanceof Error) {
        errorMsg = `Retry failed: ${error.message}`;
      } else if (typeof error === "string") {
        errorMsg = `Retry failed: ${error}`;
      }

      const errorMessage: ChatMessage = {
        id: Date.now().toString() + "_error",
        role: "assistant",
        content: "",
        status: "error",
        error: errorMsg,
        originalRequest: message.originalRequest,
      };
      setMessages((prev) => [...prev, errorMessage]);
    } finally {
      setIsLoading(false);
      setInput("");
    }
  };

  return (
    <div className="max-w-4xl mx-auto p-6 relative size-full h-screen">
      <div className="flex flex-col h-full">
        <div className="flex justify-between items-center mb-4">
          <h1 className="text-2xl font-bold">AI Chat</h1>
          <ThemeToggle />
        </div>
        <Conversation className="h-full">
          <ConversationContent>
            {messages.map((message) => (
              <Message
                from={message.role}
                key={message.id}
                status={message.status}
              >
                <MessageContent>
                  {message.status === "error" ? (
                    <div className="space-y-3">
                      <div className="flex items-center gap-2 text-destructive">
                        <AlertCircleIcon className="size-4 flex-shrink-0" />
                        <span className="font-medium">
                          Failed to send message
                        </span>
                      </div>
                      <div className="text-sm text-destructive/80">
                        {message.error || "An unknown error occurred"}
                      </div>
                      <Actions>
                        <Action
                          tooltip="Retry sending this message"
                          onClick={() => retryMessage(message.id)}
                          disabled={isLoading}
                          className="text-destructive hover:text-destructive-foreground hover:bg-destructive"
                        >
                          <RotateCcwIcon className="size-4" />
                        </Action>
                      </Actions>
                    </div>
                  ) : (
                    <Response>{message.content}</Response>
                  )}
                </MessageContent>
              </Message>
            ))}
            {isLoading && <Loader />}
          </ConversationContent>
          <ConversationScrollButton />
        </Conversation>

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
                onClick={() => setWebSearch(!webSearch)}
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
}

export default App;
