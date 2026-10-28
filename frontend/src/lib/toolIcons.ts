import {
  WrenchIcon,
  SearchIcon,
  ImageIcon,
  type LucideIcon,
  BookIcon,
  Scan,
  GlobeIcon,
  Terminal,
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
  browser_sandbox: Terminal,
};

export const getToolIcon = (toolName: string): LucideIcon => {
  return BUILT_IN_TOOL_ICONS[toolName] ?? WrenchIcon;
};

/**
 * Google favicon URL for a page/API host (trims common subdomains).
 *
 * Hits faviconV2 directly instead of www.google.com/s2/favicons because s2
 * rewrites the request and drops query params. `drop_404_icon=true` makes the
 * service answer 404 (empty body) when it has no icon, instead of serving a
 * generic globe PNG with a 404 status — browsers still render that body and
 * fire `load`, so `onError` fallbacks would never trigger.
 */
export const getFaviconUrl = (sourceUrl: string | undefined | null): string | null => {
  if (!sourceUrl) {
    return null;
  }

  try {
    const url = new URL(sourceUrl);
    const parts = url.hostname.split(".");
    const domain = parts.length > 2 ? parts.slice(-2).join(".") : url.hostname;

    const params = new URLSearchParams({
      client: "SOCIAL",
      type: "FAVICON",
      fallback_opts: "TYPE,SIZE,URL",
      url: `https://${domain}`,
      size: "32",
      drop_404_icon: "true",
    });

    return `https://t0.gstatic.com/faviconV2?${params.toString()}`;
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
