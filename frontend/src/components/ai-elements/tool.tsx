'use client';

import {
  CheckCircleIcon,
  ChevronDownIcon,
  CircleIcon,
  ClockIcon,
  WrenchIcon,
  XCircleIcon,
} from 'lucide-react';
import { type ComponentProps, type ReactNode, useState, useEffect } from 'react';
import { Badge } from '@/components/ui/badge.tsx';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible.tsx';
import { cn } from '@/lib/utils.ts';
import type { ToolUIPart } from 'ai';
import { CodeBlock } from './code-block.tsx';

export type ToolProps = ComponentProps<typeof Collapsible>;

export const Tool = ({ className, ...props }: ToolProps) => (
  <Collapsible
    className={cn('not-prose mb-2 w-full rounded-md ', className)}
    {...props}
  />
);

export type ToolHeaderProps = {
  type: ToolUIPart['type'];
  state: ToolUIPart['state'];
  className?: string;
  mcpUrl?: string;
};

const getStatusBadge = (status: ToolUIPart['state']) => {
  // const labels = {
  //   'input-streaming': 'Pending',
  //   'input-available': 'Running',
  //   'output-available': 'Completed',
  //   'output-error': 'Error',
  // } as const;

  const icons = {
    'input-streaming': <CircleIcon className="size-4" />,
    'input-available': <ClockIcon className="size-4 animate-pulse" />,
    'output-available': <CheckCircleIcon className="size-4 text-green-600" />,
    'output-error': <XCircleIcon className="size-4 text-red-600" />,
  } as const;

  return (
    <Badge className="rounded-full text-xs" variant="secondary">
      {icons[status]}
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

  let faviconUrl: string | null = null;
  if (mcpUrl && !imageError) {
    try {
      const url = new URL(mcpUrl);
      const hostname = url.hostname;
      // Simple heuristic: if there are more than 2 parts and the first part is 'api' or 'www', remove it.
      // This is a basic implementation of "trim subdomains".
      const parts = hostname.split('.');
      let domain = hostname;
      if (parts.length > 2) {
        domain = parts.slice(-2).join('.');
      }
      
      faviconUrl = `https://www.google.com/s2/favicons?domain=https://${domain}&sz=32`;
    } catch (e) {
      console.warn('Invalid MCP URL for favicon:', mcpUrl);
      // invalid url, ignore
    }
  }

  return (
    <CollapsibleTrigger
      className={cn(
        'flex w-full items-center gap-4',
        className,
      )}
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
          <WrenchIcon className="size-4 text-muted-foreground" />
        )}
        <span className="font-sm text-muted-foreground text-sm">{type}</span>
        {getStatusBadge(state)}
      </div>
      <ChevronDownIcon className="size-4 text-muted-foreground transition-transform group-data-[state=open]:rotate-180" />
    </CollapsibleTrigger>
  );
};

export type ToolContentProps = ComponentProps<typeof CollapsibleContent>;

export const ToolContent = ({ className, ...props }: ToolContentProps) => (
  <CollapsibleContent
    className={cn(
      'text-popover-foreground outline-none data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:slide-out-to-top-2 data-[state=open]:slide-in-from-top-2',
      className,
    )}
    {...props}
  />
);

export type ToolInputProps = ComponentProps<'div'> & {
  input: ToolUIPart['input'];
};

export const ToolInput = ({ className, input, ...props }: ToolInputProps) => (
  <div className={cn('space-y-2 overflow-hidden py-4', className)} {...props}>
    <h4 className="font-medium text-muted-foreground text-xs uppercase tracking-wide">
      Parameters
    </h4>
    <div className="rounded-md bg-muted/50">
      <CodeBlock code={JSON.stringify(input, null, 2)} language="json" />
    </div>
  </div>
);

export type ToolOutputProps = ComponentProps<'div'> & {
  output: ReactNode;
  errorText: ToolUIPart['errorText'];
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
    <div className={cn('space-y-2', className)} {...props}>
      <h4 className="font-medium text-muted-foreground text-xs uppercase tracking-wide">
        {errorText ? 'Error' : 'Result'}
      </h4>
      <div
        className={cn(
          'overflow-x-auto rounded-md !text-muted-foreground text-xs p-1 [&_table]:w-full',
          errorText
            ? 'bg-destructive/10 text-destructive'
            : 'bg-muted/50 text-foreground',
        )}
      >
        {errorText && <div>{errorText}</div>}
        {output && <div>{output}</div>}
      </div>
    </div>
  );
};
