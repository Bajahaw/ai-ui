"use client";

import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  PlusIcon,
  MessageSquareIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  XIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { ComponentProps, useEffect } from "react";
import { Conversation } from "@/hooks/useConversations";

export interface ConversationSidebarProps extends ComponentProps<"div"> {
  conversations?: Conversation[];
  activeConversationId?: string | null;
  onConversationSelect?: (conversationId: string) => void;
  onNewChat?: () => void;
  isCollapsed?: boolean;
  onToggleCollapse?: () => void;
  maxWidth?: string;
  isMobile?: boolean;
}

export const ConversationSidebar = ({
  conversations = [],
  activeConversationId,
  onConversationSelect,
  onNewChat,
  isCollapsed = false,
  onToggleCollapse,
  maxWidth = "33.333333%", // 1/3 of page
  isMobile = false,
  className,
  ...props
}: ConversationSidebarProps) => {
  const truncateTitle = (title: string, maxLength: number = 30) => {
    return title.length > maxLength
      ? `${title.substring(0, maxLength)}...`
      : title;
  };

  // Handle mobile overlay behavior
  useEffect(() => {
    if (isMobile && !isCollapsed) {
      document.body.style.overflow = "hidden";
      return () => {
        document.body.style.overflow = "unset";
      };
    }
  }, [isMobile, isCollapsed]);

  if (isCollapsed) {
    return (
      <div
        className={cn(
          "flex flex-col h-full w-12 border-r bg-background/50 backdrop-blur-sm",
          "md:static md:translate-x-0",
          className,
        )}
        {...props}
      >
        <div className="p-2 border-b">
          <Button
            variant="ghost"
            size="sm"
            onClick={onToggleCollapse}
            className="w-full h-8 p-0"
          >
            <ChevronRightIcon className="size-4" />
          </Button>
        </div>
        <div className="p-2">
          <Button
            variant="outline"
            size="sm"
            onClick={onNewChat}
            className="w-full h-8 p-0"
          >
            <PlusIcon className="size-4" />
          </Button>
        </div>
      </div>
    );
  }

  return (
    <>
      {/* Mobile Overlay */}
      {isMobile && !isCollapsed && (
        <div
          className="fixed inset-0 bg-black/50 z-40 md:hidden"
          onClick={onToggleCollapse}
        />
      )}

      <div
        className={cn(
          "flex flex-col h-full border-r bg-background/95 backdrop-blur-sm transition-all duration-200",
          // Mobile styles
          isMobile
            ? cn(
                "fixed top-0 left-0 z-50 md:static md:z-auto",
                "shadow-lg md:shadow-none",
                isCollapsed
                  ? "-translate-x-full md:translate-x-0"
                  : "translate-x-0",
              )
            : "",
          className,
        )}
        style={{
          width: isMobile ? "80vw" : maxWidth,
          minWidth: isMobile ? "280px" : "280px",
          maxWidth: isMobile ? "320px" : "none",
        }}
        {...props}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b">
          <h2 className="font-semibold text-sm">Conversations</h2>
          <Button
            variant="ghost"
            size="sm"
            onClick={onToggleCollapse}
            className="h-8 w-8 p-0"
          >
            {isMobile ? (
              <XIcon className="size-4" />
            ) : (
              <ChevronLeftIcon className="size-4" />
            )}
          </Button>
        </div>

        {/* New Chat Button */}
        <div className="p-4 border-b">
          <Button
            variant="outline"
            onClick={onNewChat}
            className="w-full justify-start gap-2"
          >
            <PlusIcon className="size-4" />
            New Chat
          </Button>
        </div>

        {/* Conversations List */}
        <ScrollArea className="flex-1">
          <div className="p-2 space-y-1">
            {conversations.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground text-sm">
                No conversations yet.
                <br />
                Start a new chat to begin.
              </div>
            ) : (
              conversations.map((conversation) => (
                <div
                  key={conversation.id}
                  className={cn(
                    "group relative w-full rounded-md transition-colors",
                    activeConversationId === conversation.id
                      ? "bg-secondary"
                      : "hover:bg-accent",
                  )}
                >
                  <Button
                    variant="ghost"
                    onClick={() => onConversationSelect?.(conversation.id)}
                    className={cn(
                      "w-full justify-start h-auto p-2 text-left hover:bg-transparent",
                      activeConversationId === conversation.id &&
                        "bg-transparent",
                    )}
                  >
                    <div className="flex items-center gap-2 w-full">
                      <MessageSquareIcon className="size-3" />
                      <span className="font-medium text-sm flex-1 truncate">
                        {truncateTitle(conversation.title)}
                      </span>
                    </div>
                  </Button>
                </div>
              ))
            )}
          </div>
        </ScrollArea>
      </div>
    </>
  );
};
