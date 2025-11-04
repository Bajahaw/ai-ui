"use client";

import { Button } from "@/components/ui/button";
import { ArrowDownIcon } from "lucide-react";
import type { ComponentProps } from "react";
import { useCallback, createContext, useContext, useState, useEffect, useRef, forwardRef } from "react";
import { cn } from "@/lib/utils";

interface ConversationContextValue {
  isAtBottom: boolean;
  scrollToBottom: () => void;
}

const ConversationContext = createContext<ConversationContextValue | null>(null);

const useConversationContext = () => {
  const context = useContext(ConversationContext);
  if (!context) {
    throw new Error("Conversation components must be used within Conversation");
  }
  return context;
};

export type ConversationProps = ComponentProps<"div"> & {
  onScrollToBottom?: () => void;
};

export const Conversation = forwardRef<HTMLDivElement, ConversationProps>(
  ({ className, onScrollToBottom, children, ...props }, ref) => {
    const [isAtBottom, setIsAtBottom] = useState(true);
    const scrollContainerRef = useRef<HTMLDivElement>(null);

    const checkIfAtBottom = useCallback(() => {
      const container = scrollContainerRef.current;
      if (!container) return;

      const threshold = 100;
      const bottom =
        container.scrollHeight - container.scrollTop - container.clientHeight <=
        threshold;
      setIsAtBottom(bottom);
    }, []);

    const scrollToBottom = useCallback(() => {
      const container = scrollContainerRef.current;
      if (!container) return;

      container.scrollTo({
        top: container.scrollHeight,
        behavior: "smooth",
      });
      onScrollToBottom?.();
    }, [onScrollToBottom]);

    useEffect(() => {
      const container = scrollContainerRef.current;
      if (!container) return;

      checkIfAtBottom();
      container.addEventListener("scroll", checkIfAtBottom);
      return () => container.removeEventListener("scroll", checkIfAtBottom);
    }, [checkIfAtBottom]);

    return (
      <ConversationContext.Provider value={{ isAtBottom, scrollToBottom }}>
        <div
          className={cn("relative flex-1 min-h-0", className)}
          {...props}
        >
          <div
            ref={(node) => {
              scrollContainerRef.current = node;
              if (typeof ref === "function") {
                ref(node);
              } else if (ref) {
                ref.current = node;
              }
            }}
            className="h-full overflow-y-auto bg-background"
            style={{ scrollbarGutter: 'stable' }}
            role="log"
          >
            {children}
          </div>
        </div>
      </ConversationContext.Provider>
    );
  }
);

Conversation.displayName = "Conversation";

export type ConversationContentProps = ComponentProps<"div">;

export const ConversationContent = ({
  className,
  ...props
}: ConversationContentProps) => (
  <div className={cn("p-4 w-full max-w-full", className)} {...props} />
);

export type ConversationScrollButtonProps = ComponentProps<typeof Button>;

export const ConversationScrollButton = ({
  className,
  ...props
}: ConversationScrollButtonProps) => {
  const { isAtBottom, scrollToBottom } = useConversationContext();

  const handleScrollToBottom = useCallback(() => {
    scrollToBottom();
  }, [scrollToBottom]);

  return (
    !isAtBottom && (
      <Button
        className={cn(
          "absolute bottom-4 left-[50%] translate-x-[-50%] rounded-full",
          className,
        )}
        onClick={handleScrollToBottom}
        size="icon"
        type="button"
        variant="outline"
        {...props}
      >
        <ArrowDownIcon className="size-4" />
      </Button>
    )
  );
};

export { useConversationContext };
