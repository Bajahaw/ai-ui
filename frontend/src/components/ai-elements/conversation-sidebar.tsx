"use client";

import {Button} from "@/components/ui/button";
import {ScrollArea} from "@/components/ui/scroll-area";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {Input} from "@/components/ui/input";
import {
    PlusIcon,
    ChevronLeftIcon,
    MoreHorizontalIcon,
    PencilIcon,
    TrashIcon,
    LogInIcon,
    LogOutIcon,
    SidebarIcon,
    SearchIcon,
} from "lucide-react";
import {cn} from "@/lib/utils";
import {ComponentProps, useState} from "react";
import {ClientConversation} from "@/lib/clientConversationManager";
import {useAuth} from "@/hooks/useAuth";
import {LoginDialog} from "@/components/auth/LoginDialog";

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

const getConversationGroup = (conversation: ClientConversation): string => {
    const dateStr = conversation.backendConversation?.updatedAt || conversation.backendConversation?.createdAt;
    if (!dateStr) return "Today";

    const date = new Date(dateStr);
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);
    const last7Days = new Date(today);
    last7Days.setDate(last7Days.getDate() - 7);

    if (date >= today) return "Today";
    if (date >= yesterday) return "Yesterday";
    if (date >= last7Days) return "Last 7 Days";
    return "Older";
};

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
    const width = 272; // Fixed width in pixels
    const [editingId, setEditingId] = useState<string | null>(null);
    const [editingTitle, setEditingTitle] = useState<string>("");
    const [searchTerm, setSearchTerm] = useState<string>("");

    // Sort and filter conversations
    const filteredConversations = conversations
        .filter(conversation =>
            conversation.title.toLowerCase().includes(searchTerm.toLowerCase())
        )
        .sort((a, b) => {
            const dateA = new Date(a.backendConversation?.updatedAt || a.backendConversation?.createdAt || Date.now());
            const dateB = new Date(b.backendConversation?.updatedAt || b.backendConversation?.createdAt || Date.now());
            return dateB.getTime() - dateA.getTime();
        });

    // Group conversations
    const groupedConversations = {
        "Today": [] as ClientConversation[],
        "Yesterday": [] as ClientConversation[],
        "Last 7 Days": [] as ClientConversation[],
        "Older": [] as ClientConversation[],
    };

    filteredConversations.forEach(conversation => {
        const group = getConversationGroup(conversation);
        if (group in groupedConversations) {
            groupedConversations[group as keyof typeof groupedConversations].push(conversation);
        } else {
             groupedConversations["Older"].push(conversation);
        }
    });

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
                "sidebar flex h-full bg-background/95 backdrop-blur-sm border-r relative", // Removed overflow-hidden
                isCollapsed && "collapsed",
                className,
            )}
            style={{
                width: isCollapsed ? "0px" : `${width}px`,
                minWidth: isCollapsed ? "0px" : "272px",
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
                <div className="flex items-center justify-between px-3 py-4 flex-shrink-0">
                    <div className="flex items-center gap-1">
						<Button
							variant="ghost"
							size="sm"
							onClick={onToggleCollapse}
							className="hover:bg-accent"
						>
							<SidebarIcon className="size-4 mr-1 text-foreground/80" />
                            <h2 className="text-xl font-bold text-foreground/80">AI Chat</h2>
						</Button>
					</div>
                    <Button
                        variant="ghost"
                        size="sm"
                        onClick={onToggleCollapse}
                        className="h-8 w-8 p-0"
                    >
                        <ChevronLeftIcon className="size-4"/>
                    </Button>
                </div>

                {/* New Chat Button */}
                <div className="px-3 pt-4 pb-2 flex-shrink-0">
                    <Button
                        variant="outline"
                        onClick={onNewChat}
                        className="w-full justify-start gap-2 rounded-lg text-foreground/80 font-semibold"
                    >
                        <PlusIcon className="size-4 text-foreground/80"/>
                        New Chat
                    </Button>
                </div>

                {/* Search Input */}
                <div className="px-3 pt-3 flex-shrink-0"> 
                    <div className="relative">
                        <SearchIcon className="absolute left-2 top-2.5 size-4 text-muted-foreground pointer-events-none"/>
                        <Input
                            placeholder="Search conversations ..."
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            className="pl-8 h-9 text-sm border-0 border-b rounded-none focus-visible:ring-0 focus-visible:border-muted-foreground !bg-transparent"
                        />
                    </div>
                </div>

                {/* Conversations List */}
                <div 
                    className="flex-1 min-h-0"
                    style={{
                        maskImage: 'linear-gradient(to bottom, black 90%, transparent 100%)',
                        WebkitMaskImage: 'linear-gradient(to bottom, black 90%, transparent 100%)'
                    }}
                >
                    
                    <ScrollArea className="h-full" type="scroll">
                        <div className="px-3 py-4 space-y-6">
                            {conversations.length === 0 ? (
                                <div className="text-center py-8 text-muted-foreground text-sm">
                                    No conversations yet.
                                    <br/>
                                    Start a new chat to begin.
                                </div>
                            ) : filteredConversations.length === 0 ? (
                                <div className="text-center py-8 text-muted-foreground text-sm">
                                    No matching conversations found.
                                </div>
                            ) : (
                                (Object.keys(groupedConversations) as Array<keyof typeof groupedConversations>).map((group) => {
                                    const groupItems = groupedConversations[group];
                                    if (groupItems.length === 0) return null;

                                    return (
                                        <div key={group}>
                                            <h3 className="px-1 text-xs font-semibold text-muted-foreground/70 mb-2">
                                                {group}
                                            </h3>
                                            {groupItems.map((conversation) => (
                                                <div
                                                    key={conversation.id}
                                                    className={cn(
                                                        "group relative w-full rounded-lg transition-colors py-[0.1rem]",
                                                        activeConversationId === conversation.id
                                                            ? "bg-secondary/80"
                                                            : "hover:bg-secondary/80",
                                                    )}
                                                >
                                                    {editingId === conversation.id ? (
                                                        <div className="flex items-center gap-2">
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
                                                                className="flex-1 text-sm border-0"
                                                                autoFocus
                                                            />
                                                        </div>
                                                    ) : (
                                                        <div className="flex items-center group/item">
                                                            <Button
                                                                variant="ghost"
                                                                onClick={() => onConversationSelect?.(conversation.id)}
                                                                className={cn(
                                                                    "flex-1 justify-start h-auto p-2 text-left hover:!bg-transparent",
                                                                )}
                                                            >
                                                                <div className="flex items-center gap-2 w-full">
                                                                    <span className="text-sm flex-1 max-w-[200px] truncate text-foreground/80">
                                                                        {conversation.title}
                                                                    </span>
                                                                </div>
                                                            </Button>
                                                            <DropdownMenu>
                                                                <DropdownMenuTrigger asChild>
                                                                    <Button
                                                                        variant="ghost"
                                                                        size="sm"
                                                                        className="opacity-0 group-hover/item:opacity-100 hover:!bg-secondary h-8 w-8 p-0 shrink-0"
                                                                    >
                                                                        <MoreHorizontalIcon className="size-4"/>
                                                                    </Button>
                                                                </DropdownMenuTrigger>
                                                                <DropdownMenuContent align="end">
                                                                    <DropdownMenuItem
                                                                        onClick={() =>
                                                                            handleRename(conversation.id, conversation.title)
                                                                        }
                                                                    >
                                                                        <PencilIcon className="size-4 mr-2"/>
                                                                        Rename
                                                                    </DropdownMenuItem>
                                                                    <DropdownMenuItem
                                                                        onClick={() => handleDelete(conversation.id)}
                                                                        className="text-destructive focus:text-destructive"
                                                                    >
                                                                        <TrashIcon className="size-4 mr-2"/>
                                                                        Delete
                                                                    </DropdownMenuItem>
                                                                </DropdownMenuContent>
                                                            </DropdownMenu>
                                                        </div>
                                                    )}
                                                </div>
                                            ))}
                                        </div>
                                    );
                                })
                            )}
                        </div>
                    </ScrollArea>
                </div>

                {/* Login/Logout Section */}
                <div className="px-3 pb-4 flex-shrink-0 font-semibold text-foreground/80">
                    <AuthButton/>
                </div>
            </div>
        </div>
    );
};

const AuthButton = () => {
    const {isAuthenticated, logout, isLoading} = useAuth();

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
                className="w-full justify-start rounded-lg gap-2"
            >
                {isLoading ? (
                    <div className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent"/>
                ) : (
                    <LogOutIcon className="size-4"/>
                )}
                <span>{isLoading ? "Logging out..." : "Logout"}</span>
            </Button>
        );
    }

    return (
        <LoginDialog>
            <Button variant="outline" className="w-full justify-start gap-2">
                <LogInIcon className="size-4"/>
                Login
            </Button>
        </LoginDialog>
    );
};
