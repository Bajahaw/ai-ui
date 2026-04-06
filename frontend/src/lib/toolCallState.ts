import { Tool, ToolCall } from "@/lib/api/types";

export type ToolCallDisplayState =
  | "awaiting-approval"
  | "input-available"
  | "output-available";

export const getToolCallDisplayState = (
  toolCall: ToolCall,
  tools: Tool[],
): ToolCallDisplayState => {
  const tool = tools.find((candidate) => candidate.name === toolCall.name);

  if (toolCall.tool_output) {
    return "output-available";
  }

  if (tool?.require_approval) {
    return "awaiting-approval";
  }

  return "input-available";
};
