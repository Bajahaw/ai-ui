import {
  WrenchIcon,
  SearchIcon,
  ImageIcon,
  type LucideIcon,
  BookIcon,
  Scan,
  GlobeIcon,
} from "lucide-react";

/**
 * Maps built-in tool names to lucide-react icons.
 * Falls back to WrenchIcon for unknown / MCP tools.
 */
const BUILT_IN_TOOL_ICONS: Record<string, LucideIcon> = {
  search_document: SearchIcon,
  read_document_page: BookIcon,
  view_document_page: Scan,
  generate_image: ImageIcon,
  http_request: GlobeIcon,
};

export const getToolIcon = (toolName: string): LucideIcon => {
  return BUILT_IN_TOOL_ICONS[toolName] ?? WrenchIcon;
};

/** Google s2 favicon URL for a page/API host (trims common subdomains). */
export const getFaviconUrl = (sourceUrl: string | undefined | null): string | null => {
  if (!sourceUrl) {
    return null;
  }

  try {
    const url = new URL(sourceUrl);
    const parts = url.hostname.split(".");
    const domain = parts.length > 2 ? parts.slice(-2).join(".") : url.hostname;
    return `https://www.google.com/s2/favicons?domain=https://${domain}&sz=32`;
  } catch {
    return null;
  }
};

/** Prefer http_request target URL; otherwise MCP server endpoint. */
export const getToolCallIconSourceUrl = (
  toolName: string,
  args: string | undefined,
  mcpEndpoint: string | undefined,
): string | undefined => {
  if (toolName === "http_request" && args) {
    try {
      const parsed = JSON.parse(args) as { url?: unknown };
      if (typeof parsed.url === "string" && parsed.url.trim()) {
        return parsed.url.trim();
      }
    } catch {
      // fall through
    }
  }
  return mcpEndpoint;
};
