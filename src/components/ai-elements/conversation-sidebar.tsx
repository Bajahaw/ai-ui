"use client";

import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { PlusIcon, MessageSquareIcon, ChevronLeftIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { ComponentProps } from "react";
import { ClientConversation } from "@/lib/clientConversationManager";

export interface ConversationSidebarProps extends ComponentProps<"div"> {
  conversations?: ClientConversation[];
  activeConversationId?: string | null;
  onConversationSelect?: (conversationId: string) => void;
  onNewChat?: () => void;
  isCollapsed?: boolean;
  onToggleCollapse?: () => void;
  maxWidth?: string;
}

export const ConversationSidebar = ({
  conversations = [],
  activeConversationId,
  onConversationSelect,
  onNewChat,
  isCollapsed = false,
  onToggleCollapse,

  className,
  ...props
}: ConversationSidebarProps) => {
  const width = 320; // Fixed width in pixels
  const truncateTitle = (title: string, maxLength: number = 30) => {
    return title.length > maxLength
      ? `${title.substring(0, maxLength)}...`
      : title;
  };

  return (
    <div
      className={cn(
        "sidebar flex h-full bg-background/95 backdrop-blur-sm border-r relative overflow-hidden",
        isCollapsed && "collapsed",
        className,
      )}
      style={{
        width: isCollapsed ? "0px" : `${width}px`,
        minWidth: isCollapsed ? "0px" : "280px",
        maxWidth: isCollapsed
          ? "0px"
          : `${Math.min(window.innerWidth * 0.4, 500)}px`,
        flexShrink: 0,
        transition:
          "width 300ms cubic-bezier(0.4, 0, 0.2, 1), min-width 300ms cubic-bezier(0.4, 0, 0.2, 1), max-width 300ms cubic-bezier(0.4, 0, 0.2, 1), opacity 300ms cubic-bezier(0.4, 0, 0.2, 1), transform 300ms cubic-bezier(0.4, 0, 0.2, 1)",
      }}
      {...props}
    >
      <div
        className={cn(
          "flex flex-col h-full w-full transition-opacity duration-200",
          isCollapsed
            ? "opacity-0 pointer-events-none invisible"
            : "opacity-100",
        )}
        style={{
          transitionDelay: isCollapsed ? "0ms" : "100ms",
          display: isCollapsed ? "none" : "flex",
        }}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4">
          <h2 className="font-semibold text-sm">Conversations</h2>
          <Button
            variant="ghost"
            size="sm"
            onClick={onToggleCollapse}
            className="h-8 w-8 p-0"
          >
            <ChevronLeftIcon className="size-4" />
          </Button>
        </div>

        {/* New Chat Button */}
        <div className="p-4">
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
    </div>
  );
};
