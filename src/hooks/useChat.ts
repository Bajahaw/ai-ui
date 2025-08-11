import { useCallback } from "react";
import { Message } from "./useConversations";

export const useChat = () => {
  const sendMessage = useCallback(
    async (
      messages: Message[],
      model: string,
      webSearch: boolean,
    ): Promise<Message> => {
      const response = await fetch("/api/chat", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          messages: messages
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

      return {
        id: Date.now().toString() + "_assistant",
        role: "assistant",
        content:
          data.message?.content ||
          data.message?.parts?.[0]?.text ||
          "Sorry, I couldn't process your request.",
        status: "success",
        timestamp: Date.now(),
      };
    },
    [],
  );

  return {
    sendMessage,
  };
};
