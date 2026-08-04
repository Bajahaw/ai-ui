"use client";

import {
  CheckCircleIcon,
  ChevronDownIcon,
  CircleIcon,
  ClockIcon,
  XCircleIcon,
  CheckIcon,
  XIcon,
} from "lucide-react";
import { getFaviconUrl, getToolIcon } from "@/lib/toolIcons";
import {
  type ComponentProps,
  type ReactNode,
  useState,
  useEffect,
} from "react";
import { Badge } from "@/components/ui/badge.tsx";
import { Button } from "@/components/ui/button.tsx";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible.tsx";
import { cn } from "@/lib/utils.ts";
import { CodeBlock } from "./code-block.tsx";

/** Tool call lifecycle states used by the tool UI. */
export type ToolCallState =
  | "input-streaming"
  | "input-available"
  | "output-available"
  | "output-error"
  | "awaiting-approval";

export type ToolProps = ComponentProps<typeof Collapsible>;

export const Tool = ({ className, ...props }: ToolProps) => (
  <Collapsible
    className={cn("not-prose mb-2 w-full rounded-md ", className)}
    {...props}
  />
);

export type ToolHeaderProps = {
  type: string;
  state: ToolCallState;
  className?: string;
  mcpUrl?: string;
};

const getStatusBadge = (status: ToolCallState) => {
  // const labels = {
  //   'input-streaming': 'Pending',
  //   'input-available': 'Running',
  //   'output-available': 'Completed',
  //   'output-error': 'Error',
  // } as const;

  const icons: Partial<Record<ToolCallState, ReactNode>> = {
    "input-streaming": <CircleIcon className="size-4" />,
    "input-available": <ClockIcon className="size-4 animate-pulse" />,
    "output-available": <CheckCircleIcon className="size-4 text-green-600" />,
    "output-error": <XCircleIcon className="size-4 text-red-600" />,
    "awaiting-approval": <CircleIcon className="size-4 text-orange-500" />,
  };

  if (status === "awaiting-approval") {
    return (
      <Badge
        className="rounded-full text-xs text-muted-foreground"
        variant="outline"
      >
        <ClockIcon className="size-3 text-orange-500" />
      </Badge>
    );
  }

  return (
    <Badge className="rounded-full text-xs" variant="secondary">
      {icons[status] ?? <CircleIcon className="size-4" />}
      {/* {labels[status]} */}
    </Badge>
  );
};

export const ToolHeader = ({
  className,
  type,
  state,
  mcpUrl,
  ...props
}: ToolHeaderProps) => {
  const [imageError, setImageError] = useState(false);

  useEffect(() => {
    setImageError(false);
  }, [mcpUrl]);

  // Extract tool name from "tool-<name>" format for built-in icon lookup
  const toolName = type.startsWith("tool-") ? type.slice(5) : type;
  const ToolIcon = getToolIcon(toolName);

  const faviconUrl = !imageError ? getFaviconUrl(mcpUrl) : null;

  return (
    <CollapsibleTrigger
      className={cn("flex w-full items-center gap-4", className)}
      {...props}
    >
      <div className="flex items-center gap-2">
        {faviconUrl ? (
          <img
            src={faviconUrl}
            className="size-4 rounded-sm object-contain"
            onError={() => setImageError(true)}
            alt="icon"
          />
        ) : (
          <ToolIcon className="size-4 text-muted-foreground" />
        )}
        <span className="font-sm text-muted-foreground text-sm">{type}</span>
        {getStatusBadge(state)}
      </div>
      <ChevronDownIcon className="size-4 text-muted-foreground transition-transform group-data-[state=open]:rotate-180" />
    </CollapsibleTrigger>
  );
};

export type ToolApprovalProps = {
  toolCallId: string;
  onAction?: (approved: boolean) => void;
};

export const ToolApproval = ({ toolCallId, onAction }: ToolApprovalProps) => {
  const [isUpdating, setIsUpdating] = useState(false);

  const handleApproval = async (approved: boolean) => {
    if (isUpdating) return;
    setIsUpdating(true);
    try {
      await fetch(
        `/api/tools/approve?call_id=${toolCallId}&approved=${approved}`,
        {
          method: "GET",
          credentials: "include",
        },
      );
      onAction?.(approved);
    } catch (error) {
      console.error("Failed to approve/reject tool:", error);
    } finally {
      setIsUpdating(false);
    }
  };

  return (
    <div className="flex justify-between gap-3 p-4 border rounded-lg my-2">
      <div className="text-sm text-muted-foreground leading-8">
        This tool requires your approval to run.
      </div>
      <div className="flex gap-2 justify-end">
        <Button
          size="sm"
          variant="outline"
          onClick={() => handleApproval(false)}
          disabled={isUpdating}
          className="rounded-lg font-medium"
        >
          <XIcon className="pl-0 size-4" />
          Reject
        </Button>
        <Button
          size="sm"
          onClick={() => handleApproval(true)}
          disabled={isUpdating}
          className="border-black bg-primary/90 font-medium rounded-lg"
        >
          <CheckIcon className="pl-0 size-4" />
          Approve
        </Button>
      </div>
    </div>
  );
};

export type ToolContentProps = ComponentProps<typeof CollapsibleContent>;

export const ToolContent = ({ className, ...props }: ToolContentProps) => (
  <CollapsibleContent
    className={cn(
      "text-popover-foreground outline-none data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:slide-out-to-top-2 data-[state=open]:slide-in-from-top-2",
      className,
    )}
    {...props}
  />
);

export type ToolInputProps = ComponentProps<"div"> & {
  input: unknown;
};

export const ToolInput = ({ className, input, ...props }: ToolInputProps) => (
  <div className={cn("space-y-2 overflow-hidden py-4", className)} {...props}>
    <h4 className="font-medium text-muted-foreground text-xs uppercase tracking-wide">
      Parameters
    </h4>
    <div className="rounded-md bg-muted/50">
      <CodeBlock code={JSON.stringify(input, null, 2)} language="json" />
    </div>
  </div>
);

export type ToolOutputProps = ComponentProps<"div"> & {
  output: ReactNode;
  errorText?: string;
};

export const ToolOutput = ({
  className,
  output,
  errorText,
  ...props
}: ToolOutputProps) => {
  if (!(output || errorText)) {
    return null;
  }

  return (
    <div className={cn("space-y-2", className)} {...props}>
      <h4 className="font-medium text-muted-foreground text-xs uppercase tracking-wide">
        {errorText ? "Error" : "Result"}
      </h4>
      <div
        className={cn(
          "overflow-x-auto rounded-md !text-muted-foreground text-xs p-1 [&_table]:w-full",
          errorText
            ? "bg-destructive/10 text-destructive"
            : "bg-muted/50 text-foreground",
        )}
      >
        {errorText && <div>{errorText}</div>}
        {output && <div>{output}</div>}
      </div>
    </div>
  );
};
