"use client";

import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import {
  PlusIcon,
  MessageSquareIcon,
  ChevronLeftIcon,
  MoreHorizontalIcon,
  PencilIcon,
  TrashIcon,
  LogInIcon,
  LogOutIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { ComponentProps, useState } from "react";
import { ClientConversation } from "@/lib/clientConversationManager";
import { useAuth } from "@/hooks/useAuth";
import { LoginDialog } from "@/components/auth/LoginDialog";

export interface ConversationSidebarProps extends ComponentProps<"div"> {
  conversations?: ClientConversation[];
  activeConversationId?: string | null;
  onConversationSelect?: (conversationId: string) => void;
  onNewChat?: () => void;
  onDeleteConversation?: (conversationId: string) => void;
  onRenameConversation?: (conversationId: string, newTitle: string) => void;
  isCollapsed?: boolean;
  onToggleCollapse?: () => void;
  maxWidth?: string;
}

export const ConversationSidebar = ({
  conversations = [],
  activeConversationId,
  onConversationSelect,
  onNewChat,
  onDeleteConversation,
  onRenameConversation,
  isCollapsed = false,
  onToggleCollapse,
  className,
  ...props
}: ConversationSidebarProps) => {
  const width = 320; // Fixed width in pixels
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState<string>("");

  const truncateTitle = (title: string, maxLength: number = 30) => {
    return title.length > maxLength
      ? `${title.substring(0, maxLength)}...`
      : title;
  };

  const handleRename = (conversationId: string, currentTitle: string) => {
    setEditingId(conversationId);
    setEditingTitle(currentTitle);
  };

  const handleSaveRename = (conversationId: string) => {
    if (editingTitle.trim() && onRenameConversation) {
      onRenameConversation(conversationId, editingTitle.trim());
    }
    setEditingId(null);
    setEditingTitle("");
  };

  const handleCancelRename = () => {
    setEditingId(null);
    setEditingTitle("");
  };

  const handleDelete = (conversationId: string) => {
    if (onDeleteConversation) {
      onDeleteConversation(conversationId);
    }
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
                  {editingId === conversation.id ? (
                    <div className="flex items-center gap-2 p-2">
                      <MessageSquareIcon className="size-3" />
                      <Input
                        value={editingTitle}
                        onChange={(e) => setEditingTitle(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            handleSaveRename(conversation.id);
                          } else if (e.key === "Escape") {
                            handleCancelRename();
                          }
                        }}
                        onBlur={() => handleSaveRename(conversation.id)}
                        className="flex-1 h-7 text-sm"
                        autoFocus
                      />
                    </div>
                  ) : (
                    <div className="flex items-center group/item">
                      <Button
                        variant="ghost"
                        onClick={() => onConversationSelect?.(conversation.id)}
                        className={cn(
                          "flex-1 justify-start h-auto p-2 text-left hover:bg-transparent",
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
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="opacity-0 group-hover/item:opacity-100 h-8 w-8 p-0 shrink-0"
                          >
                            <MoreHorizontalIcon className="size-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem
                            onClick={() =>
                              handleRename(conversation.id, conversation.title)
                            }
                          >
                            <PencilIcon className="size-4 mr-2" />
                            Rename
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            onClick={() => handleDelete(conversation.id)}
                            className="text-destructive focus:text-destructive"
                          >
                            <TrashIcon className="size-4 mr-2" />
                            Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  )}
                </div>
              ))
            )}
          </div>
        </ScrollArea>

        {/* Login/Logout Section */}
        <div className="p-4 border-t">
          <AuthButton />
        </div>
      </div>
    </div>
  );
};

const AuthButton = () => {
  const { isAuthenticated, logout, isLoading } = useAuth();

  const handleLogout = async () => {
    try {
      await logout();
    } catch (err) {
      console.error("Logout failed:", err);
    }
  };

  if (isAuthenticated) {
    return (
      <Button
        variant="outline"
        onClick={handleLogout}
        disabled={isLoading}
        className="w-full justify-start gap-2"
      >
        {isLoading ? (
          <div className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
        ) : (
          <LogOutIcon className="size-4" />
        )}
        <span>{isLoading ? "Logging out..." : "Logout"}</span>
      </Button>
    );
  }

  return (
    <LoginDialog>
      <Button variant="outline" className="w-full justify-start gap-2">
        <LogInIcon className="size-4" />
        Login
      </Button>
    </LoginDialog>
  );
};
