import { useEffect, useMemo, useRef, useState } from "react";
import { BrainIcon, WrenchIcon, ClockIcon } from "lucide-react";
import {
  Reasoning,
  ReasoningContent,
  ReasoningTrigger,
} from "@/components/ai-elements/reasoning";
import {
  Tool,
  ToolApproval,
  ToolContent,
  ToolHeader,
  ToolInput,
  ToolOutput,
} from "@/components/ai-elements/tool";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  FrontendMessage,
  MCPServerResponse,
  Tool as ToolDefinition,
  ToolCall,
} from "@/lib/api/types";
import {
  ToolCallDisplayState,
  getToolCallDisplayState,
} from "@/lib/toolCallState";
import { cn } from "@/lib/utils";
import { Badge } from "../ui/badge";

type SettingsDataLike = {
  tools: ToolDefinition[];
  mcpServers: MCPServerResponse[];
};

type ThoughtsToolsGroupProps = {
  message: FrontendMessage;
  settingsData: SettingsDataLike;
  className?: string;
};

const MAX_VISIBLE_TOOL_ICONS = 3;
const ICON_CHIP_CLASS =
  "flex size-4.5 items-center justify-center overflow-hidden rounded-full border bg-muted";

const safeParseJSON = (jsonString: string | undefined) => {
  if (!jsonString) return {};

  try {
    return JSON.parse(jsonString);
  } catch (e) {
    console.error("Failed to parse JSON:", e);
    return { error: "Invalid JSON", raw: jsonString };
  }
};

const ToolCallItem = ({
  toolCall,
  settingsData,
}: {
  toolCall: ToolCall;
  settingsData: SettingsDataLike;
}) => {
  const tool = settingsData.tools.find(
    (candidate) => candidate.name === toolCall.name,
  );
  const initialState = getToolCallDisplayState(toolCall, settingsData.tools);

  const [localState, setLocalState] =
    useState<ToolCallDisplayState>(initialState);

  useEffect(() => {
    setLocalState(initialState);
  }, [initialState]);

  return (
    <Tool key={toolCall.id} defaultOpen={false}>
      <ToolHeader
        type={`tool-${toolCall.name}` as `tool-${string}`}
        state={localState}
        mcpUrl={(() => {
          if (!tool || !tool.mcp_server_id) return undefined;
          const server = settingsData.mcpServers.find(
            (mcpServer) => mcpServer.id === tool.mcp_server_id,
          );
          return server?.endpoint;
        })()}
      />
      <ToolContent>
        <ToolInput input={safeParseJSON(toolCall.args)} />
        {localState === "awaiting-approval" && (
          <ToolApproval
            toolCallId={toolCall.id}
            onAction={(approved) => {
              if (approved) {
                setLocalState("input-available");
              }
            }}
          />
        )}
        {toolCall.tool_output && (
          <ToolOutput output={toolCall.tool_output} errorText={undefined} />
        )}
      </ToolContent>
    </Tool>
  );
};

const getMcpUrlForToolCall = (
  toolCall: ToolCall,
  settingsData: SettingsDataLike,
) => {
  const tool = settingsData.tools.find(
    (candidate) => candidate.name === toolCall.name,
  );
  if (!tool?.mcp_server_id) {
    return undefined;
  }

  const server = settingsData.mcpServers.find(
    (mcpServer) => mcpServer.id === tool.mcp_server_id,
  );

  return server?.endpoint;
};

const getFaviconUrl = (mcpUrl: string | undefined) => {
  if (!mcpUrl) {
    return null;
  }

  try {
    const url = new URL(mcpUrl);
    const parts = url.hostname.split(".");
    const domain = parts.length > 2 ? parts.slice(-2).join(".") : url.hostname;
    return `https://www.google.com/s2/favicons?domain=https://${domain}&sz=32`;
  } catch {
    return null;
  }
};

const ToolSummaryIcon = ({
  toolCall,
  settingsData,
}: {
  toolCall: ToolCall;
  settingsData: SettingsDataLike;
}) => {
  const [imageError, setImageError] = useState(false);
  const mcpUrl = useMemo(
    () => getMcpUrlForToolCall(toolCall, settingsData),
    [toolCall, settingsData],
  );
  const faviconUrl = useMemo(
    () => (!imageError ? getFaviconUrl(mcpUrl) : null),
    [mcpUrl, imageError],
  );

  useEffect(() => {
    setImageError(false);
  }, [mcpUrl]);

  if (!faviconUrl) {
    return (
      <span className={ICON_CHIP_CLASS} aria-hidden="true">
        <WrenchIcon className="size-3 text-muted-foreground" />
      </span>
    );
  }

  return (
    <span className={ICON_CHIP_CLASS} aria-hidden="true">
      <img
        src={faviconUrl}
        className="size-full object-cover"
        onError={() => setImageError(true)}
        alt={`${toolCall.name} icon`}
      />
    </span>
  );
};

export const ThoughtsToolsGroup = ({
  message,
  settingsData,
  className,
}: ThoughtsToolsGroupProps) => {
  const toolCalls = message.toolCalls ?? [];
  const hasReasoning = Boolean(message.reasoning?.trim());
  const isStreaming = message.status === "pending";

  const toolStates = useMemo(
    () =>
      toolCalls.map((toolCall) =>
        getToolCallDisplayState(toolCall, settingsData.tools),
      ),
    [toolCalls, settingsData.tools],
  );

  const hasAwaitingApproval = toolStates.some(
    (toolState) => toolState === "awaiting-approval",
  );

  const [isOpen, setIsOpen] = useState(() => hasAwaitingApproval);
  const previousAwaitingApprovalRef = useRef(hasAwaitingApproval);

  // Force tool call text to be visible for a brief moment even if it completes instantly
  const [briefToolName, setBriefToolName] = useState<string | null>(null);

  useEffect(() => {
    const lastCall = toolCalls[toolCalls.length - 1];
    if (lastCall) {
      setBriefToolName(lastCall.name);

      const timer = setTimeout(() => {
        setBriefToolName(null);
      }, 1500); // 1.5s delay

      return () => clearTimeout(timer);
    }
  }, [toolCalls.length]);

  useEffect(() => {
    if (hasAwaitingApproval && !previousAwaitingApprovalRef.current) {
      setIsOpen(true);
    }

    previousAwaitingApprovalRef.current = hasAwaitingApproval;
  }, [hasAwaitingApproval]);

  if (!hasReasoning && toolCalls.length === 0) {
    return null;
  }

  const visibleToolCalls = toolCalls.slice(-MAX_VISIBLE_TOOL_ICONS);

  const lastToolCall = toolCalls[toolCalls.length - 1];
  const isCallingTool = lastToolCall && !lastToolCall.tool_output;
  const displayToolName =
    briefToolName || (isCallingTool ? lastToolCall.name : null);

  const statusText = isStreaming
    ? displayToolName
      ? `Calling ${displayToolName}...`
      : hasReasoning
        ? "Thinking..."
        : "Thoughts and tools"
    : "Thoughts and tools";

  return (
    <Collapsible
      open={isOpen}
      onOpenChange={setIsOpen}
      className={cn("not-prose mb-2 rounded-md", className)}
    >
      <CollapsibleTrigger className="flex w-full items-center text-sm text-muted-foreground">
        <div className="flex min-w-0 items-center gap-2">
          {hasReasoning && (
            <BrainIcon
              className={cn(
                "size-4",
                "text-muted-foreground",
                isStreaming && "animate-pulse",
              )}
              aria-hidden="true"
            />
          )}
          {visibleToolCalls.length > 0 && (
            <div className="flex items-center -space-x-1.5">
              {visibleToolCalls.map((toolCall) => (
                <ToolSummaryIcon
                  key={`tool-icon-${toolCall.id}`}
                  toolCall={toolCall}
                  settingsData={settingsData}
                />
              ))}
            </div>
          )}
          <div className="flex min-w-0 items-center gap-2">
            <div className="relative flex h-5 items-center overflow-hidden">
              <span
                key={statusText}
                className="truncate animate-in fade-in slide-in-from-bottom-2 duration-300"
              >
                {statusText}
              </span>
            </div>
            {hasAwaitingApproval && (
              <Badge
                className="rounded-full text-xs text-muted-foreground"
                variant="outline"
              >
                <ClockIcon className="mr-1 size-3 text-orange-500" />
                Awaiting Approval
              </Badge>
            )}
          </div>
        </div>
      </CollapsibleTrigger>

      <CollapsibleContent
        className={cn(
          "mt-3 space-y-2 overflow-hidden",
          "data-[state=open]:animate-in data-[state=closed]:animate-out",
          "data-[state=closed]:fade-out-0 data-[state=closed]:slide-out-to-top-2",
          "data-[state=open]:fade-in-0 data-[state=open]:slide-in-from-top-2",
        )}
      >
        {hasReasoning && (
          <Reasoning
            isStreaming={isStreaming}
            duration={message.reasoningDuration}
            defaultOpen={false}
          >
            <ReasoningTrigger />
            <ReasoningContent>{message.reasoning || ""}</ReasoningContent>
          </Reasoning>
        )}

        {toolCalls.map((toolCall) => (
          <ToolCallItem
            key={toolCall.id}
            toolCall={toolCall}
            settingsData={settingsData}
          />
        ))}
      </CollapsibleContent>
    </Collapsible>
  );
};
