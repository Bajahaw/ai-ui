import {
  WrenchIcon,
  SearchIcon,
  ImageIcon,
  type LucideIcon,
  BookIcon,
  CloudIcon,
  Scan,
  GlobeIcon,
} from "lucide-react";

/**
 * Maps built-in tool names to lucide-react icons.
 * Falls back to WrenchIcon for unknown / MCP tools.
 */
const BUILT_IN_TOOL_ICONS: Record<string, LucideIcon> = {
  search_ddgs: SearchIcon,
  get_weather: CloudIcon,
  search_document: SearchIcon,
  read_document_page: BookIcon,
  view_document_page: Scan,
  generate_image: ImageIcon,
  http_request: GlobeIcon,
};

export const getToolIcon = (toolName: string): LucideIcon => {
  return BUILT_IN_TOOL_ICONS[toolName] ?? WrenchIcon;
};
