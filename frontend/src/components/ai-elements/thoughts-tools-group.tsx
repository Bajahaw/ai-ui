import { createElement, useEffect, useMemo, useState } from "react";
import { BrainIcon, ClockIcon } from "lucide-react";
import {
  getFaviconUrl,
  getToolCallIconSourceUrl,
  getToolIcon,
} from "@/lib/toolIcons";
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
import { runBrowserSandbox } from "@/lib/sandbox/runner";
import { Badge } from "../ui/badge";

type SettingsDataLike = {
  tools: ToolDefinition[];
  mcpServers: MCPServerResponse[];
};

type ThoughtsToolsGroupProps = {
  message: FrontendMessage;
  settingsData: SettingsDataLike;
  className?: string;
  conversationFileIds?: string[];
};

const MAX_VISIBLE_TOOL_ICONS = 3;
const NO_TOOL_CALLS: ToolCall[] = [];
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
  conversationFileIds,
}: {
  toolCall: ToolCall;
  settingsData: SettingsDataLike;
  conversationFileIds?: string[];
}) => {
  const tool = settingsData.tools.find(
    (candidate) => candidate.name === toolCall.name,
  );
  const initialState = getToolCallDisplayState(toolCall, settingsData.tools);

  const [localState, setLocalState] =
    useState<ToolCallDisplayState>(initialState);
  const [prevInitialState, setPrevInitialState] = useState(initialState);

  if (initialState !== prevInitialState) {
    setPrevInitialState(initialState);
    setLocalState(initialState);
  }

  return (
    <Tool key={toolCall.id} defaultOpen={false}>
      <ToolHeader
        type={`tool-${toolCall.name}` as `tool-${string}`}
        state={localState}
        mcpUrl={getToolCallIconSourceUrl(
          toolCall.name,
          toolCall.args,
          tool?.mcp_server_id
            ? settingsData.mcpServers.find(
                (mcpServer) => mcpServer.id === tool.mcp_server_id,
              )?.endpoint
            : undefined,
        )}
      />
      <ToolContent>
        <ToolInput input={safeParseJSON(toolCall.args)} />
        {localState === "awaiting-approval" && (
          <ToolApproval
            toolCallId={toolCall.id}
            onAction={(approved) => {
              if (approved) {
                setLocalState("input-available");
                // browser_sandbox executes client-side: approval is the
                // trigger to run, now that the backend is waiting for us.
                // Conversation files are auto-mounted; the model only sees a filesystem.
                if (toolCall.name === "browser_sandbox") {
                  void runBrowserSandbox(toolCall, undefined, conversationFileIds);
                }
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

const getIconSourceUrlForToolCall = (
  toolCall: ToolCall,
  settingsData: SettingsDataLike,
) => {
  const tool = settingsData.tools.find(
    (candidate) => candidate.name === toolCall.name,
  );
  const mcpEndpoint = tool?.mcp_server_id
    ? settingsData.mcpServers.find(
        (mcpServer) => mcpServer.id === tool.mcp_server_id,
      )?.endpoint
    : undefined;

  return getToolCallIconSourceUrl(toolCall.name, toolCall.args, mcpEndpoint);
};

const ToolSummaryIcon = ({
  toolCall,
  settingsData,
}: {
  toolCall: ToolCall;
  settingsData: SettingsDataLike;
}) => {
  // Remember which URL failed so a new URL gets a fresh attempt
  const [failedUrl, setFailedUrl] = useState<string | undefined>();
  const iconSourceUrl = useMemo(
    () => getIconSourceUrlForToolCall(toolCall, settingsData),
    [toolCall, settingsData],
  );
  const faviconUrl = useMemo(
    () => (failedUrl !== iconSourceUrl ? getFaviconUrl(iconSourceUrl) : null),
    [iconSourceUrl, failedUrl],
  );

  if (!faviconUrl) {
    return (
      <span className={ICON_CHIP_CLASS} aria-hidden="true">
        {createElement(getToolIcon(toolCall.name), {
          className: "size-3 text-muted-foreground",
        })}
      </span>
    );
  }

  return (
    <span className={ICON_CHIP_CLASS} aria-hidden="true">
      <img
        src={faviconUrl}
        className="size-full object-cover"
        onError={() => setFailedUrl(iconSourceUrl)}
        alt={`${toolCall.name} icon`}
      />
    </span>
  );
};

export const ThoughtsToolsGroup = ({
  message,
  settingsData,
  className,
  conversationFileIds,
}: ThoughtsToolsGroupProps) => {
  const toolCalls = message.toolCalls ?? NO_TOOL_CALLS;
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
  const [prevAwaitingApproval, setPrevAwaitingApproval] =
    useState(hasAwaitingApproval);

  if (hasAwaitingApproval !== prevAwaitingApproval) {
    setPrevAwaitingApproval(hasAwaitingApproval);
    if (hasAwaitingApproval) {
      setIsOpen(true);
    }
  }

  // Force tool call text to be visible for a brief moment even if it completes instantly.
  // Holds the tool count whose 1.5s window has elapsed; a new call reopens the window.
  const toolCallCount = toolCalls.length;
  const [expiredToolCallCount, setExpiredToolCallCount] = useState<
    number | null
  >(null);

  useEffect(() => {
    if (toolCallCount === 0) return;
    const timer = setTimeout(() => {
      setExpiredToolCallCount(toolCallCount);
    }, 1500); // 1.5s delay
    return () => clearTimeout(timer);
  }, [toolCallCount]);

  const briefToolName =
    toolCallCount > 0 && expiredToolCallCount !== toolCallCount
      ? toolCalls[toolCallCount - 1].name
      : null;

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
            conversationFileIds={conversationFileIds}
          />
        ))}
      </CollapsibleContent>
    </Collapsible>
  );
};
