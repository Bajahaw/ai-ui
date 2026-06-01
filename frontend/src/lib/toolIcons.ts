import {
  WrenchIcon,
  SearchIcon,
  ListIcon,
  FileCodeIcon,
  FilePlusIcon,
  FileEditIcon,
  FileXIcon,
  ImageIcon,
  type LucideIcon,
  BookIcon,
  CloudIcon,
  Scan,
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
  list_document_parts: ListIcon,
  read_document_part: FileCodeIcon,
  create_document: FilePlusIcon,
  write_document_part: FileEditIcon,
  delete_document_part: FileXIcon,
  generate_image: ImageIcon,
};

export const getToolIcon = (toolName: string): LucideIcon => {
  return BUILT_IN_TOOL_ICONS[toolName] ?? WrenchIcon;
};
