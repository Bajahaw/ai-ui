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
  ChevronLeftIcon,
  MoreHorizontalIcon,
  PencilIcon,
  TrashIcon,
  LogInIcon,
  LogOutIcon,
  UserRoundIcon,
  SidebarIcon,
  SearchIcon,
  Loader2Icon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import {
  ComponentProps,
  useDeferredValue,
  useEffect,
  useLayoutEffect,
  useState,
  useMemo,
  useRef,
} from "react";
import { ClientConversation } from "@/lib/clientConversationManager";
import { useAuth } from "@/hooks/useAuth";
import { LoginDialog } from "@/components/auth/LoginDialog";
import { ProfilePickerDialog } from "@/components/auth/ProfilePickerDialog";
import { useVirtualizer } from "@tanstack/react-virtual";
import { conversationsAPI } from "@/lib/api/conversations";
import type { ConversationSearchHit } from "@/lib/api/types";

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
  isLoading?: boolean;
}

type ListConversation = ClientConversation & {
  /** Present when this row matched via message FTS */
  searchSnippet?: string;
};

type FlatItem =
  | { type: "header"; id: string; label: string }
  | { type: "item"; id: string; data: ListConversation };

const getConversationGroup = (conversation: ClientConversation): string => {
  const dateStr =
    conversation.backendConversation?.updatedAt ||
    conversation.backendConversation?.createdAt;
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

const updatedAtMs = (c: ClientConversation): number => {
  const raw =
    c.backendConversation?.updatedAt || c.backendConversation?.createdAt;
  if (!raw) return 0;
  const t = Date.parse(raw);
  return Number.isFinite(t) ? t : 0;
};

const sortByUpdatedDesc = (a: ClientConversation, b: ClientConversation) =>
  updatedAtMs(b) - updatedAtMs(a);

/** Render FTS snippet with [matched] markers from the backend. */
const SearchSnippet = ({ snippet }: { snippet: string }) => {
  const parts = snippet.split(/(\[[^\]]*\])/g);
  return (
    <span className="block text-xs text-muted-foreground/80 truncate leading-snug">
      {parts.map((part, i) => {
        if (part.startsWith("[") && part.endsWith("]") && part.length > 2) {
          return (
            <span key={i} className="text-foreground/70 font-medium">
              {part.slice(1, -1)}
            </span>
          );
        }
        return <span key={i}>{part}</span>;
      })}
    </span>
  );
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
  isLoading = false,
  className,
  ...props
}: ConversationSidebarProps) => {
  const { isAuthenticated, isCheckingAuth } = useAuth();
  const width = 272; // Fixed width in pixels
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState<string>("");
  // Input value updates immediately; list filtering is deferred so typing stays responsive.
  const [searchTerm, setSearchTerm] = useState<string>("");
  const deferredSearch = useDeferredValue(searchTerm);
  const [ftsHits, setFtsHits] = useState<ConversationSearchHit[]>([]);
  const [ftsLoading, setFtsLoading] = useState(false);

  const trimmedSearch = deferredSearch.trim();
  const isSearching = trimmedSearch.length > 0;
  const isSearchPending = searchTerm.trim() !== trimmedSearch;

  // Title filter uses deferred query so the input never waits on list work.
  const titleMatches = useMemo(() => {
    if (!isSearching) return conversations;
    const q = trimmedSearch.toLowerCase();
    return conversations.filter((c) => c.title.toLowerCase().includes(q));
  }, [conversations, isSearching, trimmedSearch]);

  // Debounced FTS over message bodies. Loading flag only flips when fetch starts
  // (not on every keystroke) to avoid extra re-renders while typing.
  useEffect(() => {
    if (!isSearching || !isAuthenticated) {
      setFtsHits((prev) => (prev.length === 0 ? prev : []));
      setFtsLoading(false);
      return;
    }

    const controller = new AbortController();
    const query = trimmedSearch;

    const timer = window.setTimeout(async () => {
      setFtsLoading(true);
      try {
        const hits = await conversationsAPI.searchConversations(
          query,
          controller.signal,
        );
        if (!controller.signal.aborted) {
          setFtsHits(hits);
        }
      } catch (err) {
        if (controller.signal.aborted) return;
        if (err instanceof DOMException && err.name === "AbortError") return;
        if (err instanceof Error && err.name === "AbortError") return;
        if (typeof navigator !== "undefined" && navigator.onLine === false) {
          setFtsHits([]);
        } else {
          console.error("Conversation search failed:", err);
          setFtsHits([]);
        }
      } finally {
        if (!controller.signal.aborted) {
          setFtsLoading(false);
        }
      }
    }, 220);

    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [trimmedSearch, isSearching, isAuthenticated]);

  // Merge title matches + FTS hits into a single list for display.
  const filteredConversations = useMemo((): ListConversation[] => {
    // Manager already keeps sidebar order — avoid copy+sort on every idle render.
    if (!isSearching) {
      return conversations as ListConversation[];
    }

    const byId = new Map(conversations.map((c) => [c.id, c]));
    const snippetByConv = new Map(
      ftsHits.map((h) => [h.conversationId, h.snippet]),
    );
    const result = new Map<string, ListConversation>();

    for (const c of titleMatches) {
      const snippet = snippetByConv.get(c.id);
      // Shallow wrapper only when a snippet is needed (keeps list work cheap).
      result.set(c.id, snippet ? { ...c, searchSnippet: snippet } : c);
    }

    for (const hit of ftsHits) {
      if (result.has(hit.conversationId)) {
        const existing = result.get(hit.conversationId)!;
        if (!existing.searchSnippet && hit.snippet) {
          result.set(hit.conversationId, {
            ...existing,
            searchSnippet: hit.snippet,
          });
        }
        continue;
      }
      const known = byId.get(hit.conversationId);
      if (known) {
        result.set(hit.conversationId, {
          ...known,
          searchSnippet: hit.snippet,
        });
      } else {
        result.set(hit.conversationId, {
          id: hit.conversationId,
          title: hit.title || "New Chat",
          messages: [],
          pendingMessageIds: new Set(),
          activeBranches: new Map(),
          searchSnippet: hit.snippet,
          backendConversation: {
            id: hit.conversationId,
            userId: "",
            title: hit.title,
            createdAt: hit.updatedAt,
            updatedAt: hit.updatedAt,
            messages: {},
          },
        });
      }
    }

    return Array.from(result.values()).sort(sortByUpdatedDesc);
  }, [conversations, isSearching, titleMatches, ftsHits]);

  // Group conversations (skip date groups while searching — flat results feel faster)
  const groupedConversations = useMemo(() => {
    if (isSearching) {
      return { Results: filteredConversations } as Record<
        string,
        ListConversation[]
      >;
    }

    const groups: Record<string, ListConversation[]> = {
      Today: [],
      Yesterday: [],
      "Last 7 Days": [],
      Older: [],
    };

    filteredConversations.forEach((conversation) => {
      const group = getConversationGroup(conversation);
      if (group in groups) {
        groups[group].push(conversation);
      } else {
        groups["Older"].push(conversation);
      }
    });

    return groups;
  }, [filteredConversations, isSearching]);

  const flatItems = useMemo(() => {
    const items: FlatItem[] = [];
    const groupOrder = isSearching
      ? ["Results"]
      : ["Today", "Yesterday", "Last 7 Days", "Older"];

    groupOrder.forEach((group) => {
      const groupItems = groupedConversations[group];
      if (!groupItems || groupItems.length === 0) return;
      if (!isSearching) {
        items.push({ type: "header", id: `header-${group}`, label: group });
      }
      groupItems.forEach((conversation) => {
        items.push({ type: "item", id: conversation.id, data: conversation });
      });
    });
    return items;
  }, [groupedConversations, isSearching]);

  const scrollRef = useRef<HTMLDivElement>(null);
  // Stable estimate lookup without closing over a new flatItems array identity
  // in a way that forces virtualizer thrash every keystroke.
  const flatItemsRef = useRef(flatItems);
  flatItemsRef.current = flatItems;

  const rowVirtualizer = useVirtualizer({
    // count=0 while collapsed avoids measureElement caching 0px row heights under
    // display:none (caused mid-list start + jumpy upward scroll on reopen).
    count: isCollapsed ? 0 : flatItems.length,
    getScrollElement: () => (isCollapsed ? null : scrollRef.current),
    estimateSize: (index) => {
      const item = flatItemsRef.current[index];
      if (!item || item.type === "header") return 40;
      return item.data.searchSnippet ? 56 : 40;
    },
    // Skip expensive DOM remeasure on every filter tick; estimates are close enough.
    // measureElement still corrects when rows mount.
    overscan: 8,
  });

  // Pin to top only on collapse→open so Today is visible (stale scroll survives collapse).
  const wasCollapsedRef = useRef(isCollapsed);
  useLayoutEffect(() => {
    const justOpened = wasCollapsedRef.current && !isCollapsed;
    wasCollapsedRef.current = isCollapsed;
    if (!justOpened) return;

    const el = scrollRef.current;
    if (el) el.scrollTop = 0;
    rowVirtualizer.scrollToOffset(0, { align: "start" });
  }, [isCollapsed, rowVirtualizer]);

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

  const showEmptySearch =
    isSearching &&
    filteredConversations.length === 0 &&
    !ftsLoading;

  return (
    <div
      className={cn(
        "sidebar flex h-full bg-background/95 backdrop-blur-sm border-r border-muted-foreground/20 relative", // Removed overflow-hidden
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
              <span className="ml-1 mt-3 text-[10px] text-muted-foreground tracking-wide font-light">
                v{__APP_VERSION__}
              </span>
            </Button>
          </div>
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
        <div className="px-3 pt-4 pb-2 flex-shrink-0">
          <Button
            variant="outline"
            onClick={onNewChat}
            disabled={!isAuthenticated || isCheckingAuth}
            title={
              !isAuthenticated ? "Sign in to start a chat" : "Start a new chat"
            }
            className="w-full justify-start gap-2 rounded-lg text-foreground/80 font-semibold"
          >
            <PlusIcon className="size-4 text-foreground/80" />
            New Chat
          </Button>
        </div>

        {/* Search Input */}
        <div className="px-3 pt-3 flex-shrink-0">
          <div className="relative">
            {ftsLoading ? (
              <Loader2Icon className="absolute left-2 top-2.5 size-4 text-muted-foreground animate-spin pointer-events-none" />
            ) : (
              <SearchIcon className="absolute left-2 top-2.5 size-4 text-muted-foreground pointer-events-none" />
            )}
            <Input
              placeholder="Search titles & messages…"
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
            maskImage:
              "linear-gradient(to bottom, black 90%, transparent 100%)",
            WebkitMaskImage:
              "linear-gradient(to bottom, black 90%, transparent 100%)",
          }}
        >
          <ScrollArea className="h-full" type="scroll" viewportRef={scrollRef}>
            {(isCheckingAuth || isLoading) &&
            conversations.length === 0 ? null : conversations.length === 0 ? (
              <div className="px-3 py-4">
                <div className="text-center py-8 text-muted-foreground text-sm">
                  {isAuthenticated ? (
                    <>
                      No conversations yet.
                      <br />
                      Start a new chat to begin.
                    </>
                  ) : (
                    <>
                      Sign in to load your conversations.
                      <br />
                      Use the button below to continue.
                    </>
                  )}
                </div>
              </div>
            ) : showEmptySearch ? (
              <div className="px-3 py-4">
                <div className="text-center py-8 text-muted-foreground text-sm">
                  No matching conversations found.
                </div>
              </div>
            ) : (
              <div
                className={cn(
                  "px-3 py-4 transition-opacity duration-100",
                  isSearchPending && "opacity-70",
                )}
              >
                <div
                  className="relative w-full"
                  style={{
                    height: `${rowVirtualizer.getTotalSize()}px`,
                  }}
                >
                  {rowVirtualizer.getVirtualItems().map((virtualItem) => {
                    const item = flatItems[virtualItem.index];

                    return (
                      <div
                        key={virtualItem.key}
                        data-index={virtualItem.index}
                        ref={rowVirtualizer.measureElement}
                        style={{
                          position: "absolute",
                          top: 0,
                          left: 0,
                          width: "100%",
                          transform: `translateY(${virtualItem.start}px)`,
                        }}
                      >
                        {item.type === "header" ? (
                          <h3 className="px-1 text-xs font-semibold text-muted-foreground/70 pb-2 pt-4">
                            {item.label}
                          </h3>
                        ) : (
                          <div
                            className={cn(
                              // No animate-fade-in while filtering — CSS animations
                              // on remounted rows are a major source of typing jank.
                              "group relative w-full rounded-lg transition-colors py-[0.1rem]",
                              !isSearching && "animate-fade-in",
                              activeConversationId === item.data.id
                                ? "bg-secondary/80"
                                : "hover:bg-secondary/80",
                            )}
                          >
                            {editingId === item.data.id ? (
                              <div className="flex items-center gap-2">
                                <Input
                                  value={editingTitle}
                                  onChange={(e) =>
                                    setEditingTitle(e.target.value)
                                  }
                                  onKeyDown={(e) => {
                                    if (e.key === "Enter") {
                                      handleSaveRename(item.data.id);
                                    } else if (e.key === "Escape") {
                                      handleCancelRename();
                                    }
                                  }}
                                  onBlur={() => handleSaveRename(item.data.id)}
                                  className="flex-1 text-sm border-0"
                                  autoFocus
                                />
                              </div>
                            ) : (
                              <div className="flex items-center group/item">
                                <Button
                                  variant="ghost"
                                  onClick={() =>
                                    onConversationSelect?.(item.data.id)
                                  }
                                  className={cn(
                                    "flex-1 justify-start h-auto p-2 text-left hover:!bg-transparent max-w-[240px] transition-all !duration-100 ease-in-out",
                                    "group-hover/item:max-w-[210px] group-focus-within/item:max-w-[210px]",
                                    activeConversationId === item.data.id &&
                                      "max-w-[210px]",
                                  )}
                                >
                                  <div className="flex flex-col gap-0.5 w-full min-w-0">
                                    <span className="text-sm flex-1 truncate text-foreground/80">
                                      {item.data.title}
                                    </span>
                                    {item.data.searchSnippet ? (
                                      <SearchSnippet
                                        snippet={item.data.searchSnippet}
                                      />
                                    ) : null}
                                  </div>
                                </Button>
                                <DropdownMenu>
                                  <DropdownMenuTrigger asChild>
                                    <Button
                                      variant="ghost"
                                      size="sm"
                                      className={cn(
                                        "h-8 w-8 p-0 shrink-0 absolute right-2 top-1/2 -translate-y-1/2 hover:!bg-secondary",
                                        "opacity-0 pointer-events-none",
                                        "group-hover/item:opacity-100 group-hover/item:pointer-events-auto",
                                        "group-focus-within/item:opacity-100 group-focus-within/item:pointer-events-auto",
                                        // Always show for the open conversation (touch-friendly).
                                        activeConversationId === item.data.id &&
                                          "opacity-100 pointer-events-auto",
                                      )}
                                    >
                                      <MoreHorizontalIcon className="size-4" />
                                    </Button>
                                  </DropdownMenuTrigger>
                                  <DropdownMenuContent align="end">
                                    <DropdownMenuItem
                                      onClick={() =>
                                        handleRename(
                                          item.data.id,
                                          item.data.title,
                                        )
                                      }
                                    >
                                      <PencilIcon className="size-4 mr-2" />
                                      Rename
                                    </DropdownMenuItem>
                                    <DropdownMenuItem
                                      onClick={() => handleDelete(item.data.id)}
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
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </ScrollArea>
        </div>

        {/* Login/Logout Section */}
        <div className="px-3 pb-4 flex-shrink-0 font-semibold text-foreground/80">
          <AuthButton />
        </div>
      </div>
    </div>
  );
};

const AuthButton = () => {
  const { isAuthenticated, isCheckingAuth, logout, isLoading, isProfilesMode, activeProfile } =
    useAuth();
  const [loginOpen, setLoginOpen] = useState(false);

  useEffect(() => {
    if (!isCheckingAuth && !isAuthenticated) {
      setLoginOpen(true);
    }
  }, [isAuthenticated, isCheckingAuth]);

  const handleLogout = async () => {
    try {
      await logout();
    } catch (err) {
      console.error("Logout failed:", err);
    }
  };

  if (isCheckingAuth) return null;

  if (isAuthenticated) {
    if (isProfilesMode) {
      return (
        <ProfilePickerDialog>
          <Button
            variant="outline"
            className="w-full justify-start rounded-lg gap-2"
          >
            <UserRoundIcon className="size-4" />
            <span className="truncate">{activeProfile ?? "Switch profile"}</span>
          </Button>
        </ProfilePickerDialog>
      );
    }
    return (
      <Button
        variant="outline"
        onClick={handleLogout}
        disabled={isLoading}
        className="w-full justify-start rounded-lg gap-2"
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
    <LoginDialog open={loginOpen} onOpenChange={setLoginOpen}>
      <Button
        variant="outline"
        className="w-full rounded-lg justify-start gap-2"
      >
        <LogInIcon className="size-4" />
        Login
      </Button>
    </LoginDialog>
  );
};
