"use client";

import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  PlusIcon,
  MessageSquareIcon,
  ChevronLeftIcon,
  XIcon,
  GripVerticalIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { ComponentProps, useEffect, useState, useCallback } from "react";
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
  onWidthChange?: (width: number) => void;
}

export const ConversationSidebar = ({
  conversations = [],
  activeConversationId,
  onConversationSelect,
  onNewChat,
  isCollapsed = false,
  onToggleCollapse,
  maxWidth = "20%", // 1/5 of page
  isMobile = false,
  onWidthChange,
  className,
  ...props
}: ConversationSidebarProps) => {
  const [width, setWidth] = useState(320); // Default width in pixels
  const [isResizing, setIsResizing] = useState(false);
  const truncateTitle = (title: string, maxLength: number = 30) => {
    return title.length > maxLength
      ? `${title.substring(0, maxLength)}...`
      : title;
  };

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    setIsResizing(true);
    e.preventDefault();
  }, []);

  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!isResizing) return;

      const newWidth = Math.min(
        Math.max(280, e.clientX),
        window.innerWidth * 0.4,
      );
      setWidth(newWidth);
      onWidthChange?.(newWidth);
    },
    [isResizing, onWidthChange],
  );

  const handleMouseUp = useCallback(() => {
    setIsResizing(false);
  }, []);

  useEffect(() => {
    if (isResizing) {
      document.addEventListener("mousemove", handleMouseMove);
      document.addEventListener("mouseup", handleMouseUp);
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";

      return () => {
        document.removeEventListener("mousemove", handleMouseMove);
        document.removeEventListener("mouseup", handleMouseUp);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
      };
    }
  }, [isResizing, handleMouseMove, handleMouseUp]);

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
    return null;
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
          "flex h-full bg-background/95 backdrop-blur-sm transition-all duration-200 border-r relative",
          // Mobile styles
          isMobile
            ? cn(
                "fixed top-0 left-0 z-50 md:static md:z-auto",
                "shadow-lg md:border-r",
                isCollapsed
                  ? "-translate-x-full md:translate-x-0"
                  : "translate-x-0",
              )
            : "",
          className,
        )}
        style={{
          width: isMobile ? "80vw" : `${width}px`,
          minWidth: isMobile ? "280px" : "280px",
          maxWidth: isMobile ? "320px" : `${window.innerWidth * 0.4}px`,
        }}
        {...props}
      >
        <div className="flex flex-col h-full flex-1">
          {/* Header */}
          <div className="flex items-center justify-between p-4">
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

        {/* Resize Handle - Only show on desktop */}
        {!isMobile && (
          <div
            className="absolute right-0 top-0 w-1 h-full cursor-col-resize hover:bg-border transition-colors flex items-center justify-center group"
            onMouseDown={handleMouseDown}
          >
            <div className="w-1 h-8 bg-border rounded-full opacity-0 group-hover:opacity-100 transition-opacity">
              <GripVerticalIcon className="w-1 h-4 text-muted-foreground" />
            </div>
          </div>
        )}
      </div>
    </>
  );
};
