"use client";

import {Loader2Icon, SendIcon, SquareIcon, XIcon} from "lucide-react";
import type {ComponentProps, HTMLAttributes, KeyboardEventHandler,} from "react";
import {Children, forwardRef, useCallback, useState} from "react";
import {ModelSelect,} from "@/components/ai-elements/model-select.tsx";
import {Button} from "@/components/ui/button.tsx";
import {Textarea} from "@/components/ui/textarea.tsx";
import {cn} from "@/lib/utils.ts";
import {getTextDirection} from "@/lib/rtl-utils.ts";
import type {ChatStatus} from "ai";
import {useSettings} from "@/hooks/useSettings";

export type PromptInputProps = HTMLAttributes<HTMLFormElement>;

export const PromptInput = ({ className, ...props }: PromptInputProps) => (
  <form
    className={cn(
      "w-full divide-y overflow-hidden rounded-xl border bg-background shadow-sm",
      className,
    )}
    {...props}
  />
);

export type PromptInputTextareaProps = ComponentProps<typeof Textarea> & {
  minHeight?: number;
  maxHeight?: number;
  onFilesPasted?: (files: File[]) => void;
};

export const PromptInputTextarea = forwardRef<HTMLTextAreaElement, PromptInputTextareaProps>(({
  onChange,
  onFilesPasted,
  className,
  placeholder = "What would you like to know?",
  value,
  ...props
}, ref) => {
  const [input, setInput] = useState(value as string || "");
  const { settings } = useSettings();

  const handleKeyDown: KeyboardEventHandler<HTMLTextAreaElement> = useCallback((e) => {
    if (e.key === "Enter") {
      // Get the enter behavior setting (default: "send")
      const enterBehavior = settings?.enterBehavior || "send";
      
      if (e.shiftKey) {
        // Always allow newline with Shift+Enter
        return;
      }

      if (enterBehavior === "newline") {
        // Allow newline on plain Enter when setting is "newline"
        return;
      }

      // Submit on Enter (without Shift) when setting is "send" (default)
      e.preventDefault();
      const form = e.currentTarget.form;
      if (form) {
        form.requestSubmit();
      }
    }
  }, [settings]);

  const handlePaste = (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    const clipboardData = e.clipboardData;
    if (!clipboardData || !onFilesPasted) return;

    const files = Array.from(clipboardData.files);
    if (files.length > 0) {
      e.preventDefault();
      onFilesPasted(files);
    }
  };

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newValue = e.target.value;
    setInput(newValue);
    onChange?.(e);
  };

  return (
    <Textarea
      ref={ref}
      className={cn(
        "w-full resize-none rounded-none border-none py-3 px-5 shadow-none outline-none ring-0",
        "bg-transparent dark:bg-transparent field-sizing-content max-h-[6lh]",
        "focus-visible:ring-0",
        getTextDirection(input),
        className,
      )}
      name="message"
      onChange={handleChange}
      onKeyDown={handleKeyDown}
      onPaste={handlePaste}
      placeholder={placeholder}
      value={value ?? input}
      {...props}
    />
  );
});
PromptInputTextarea.displayName = "PromptInputTextarea";

export type PromptInputToolbarProps = HTMLAttributes<HTMLDivElement>;

export const PromptInputToolbar = ({
  className,
  ...props
}: PromptInputToolbarProps) => (
  <div
    className={cn("flex items-center justify-between py-1 px-1.5", className)}
    {...props}
  />
);

export type PromptInputToolsProps = HTMLAttributes<HTMLDivElement>;

export const PromptInputTools = ({
  className,
  ...props
}: PromptInputToolsProps) => (
  <div
    className={cn(
      "flex items-center gap-1",
      "[&_button:first-child]:rounded-bl-xl",
      className,
    )}
    {...props}
  />
);

export type PromptInputButtonProps = ComponentProps<typeof Button>;

export const PromptInputButton = ({
  variant = "ghost",
  className,
  size,
  ...props
}: PromptInputButtonProps) => {
  const newSize =
    (size ?? Children.count(props.children) > 1) ? "default" : "icon";

  return (
    <Button
      className={cn(
        "shrink-0 gap-1.5 rounded-lg",
        variant === "ghost" && "text-muted-foreground",
        newSize === "default" && "px-3",
        className,
      )}
      size={newSize}
      type="button"
      variant={variant}
      {...props}
    />
  );
};

export type PromptInputSubmitProps = ComponentProps<typeof Button> & {
  status?: ChatStatus;
};

export const PromptInputSubmit = ({
  className,
  variant = "default",
  size = "icon",
  status,
  children,
  ...props
}: PromptInputSubmitProps) => {
  let Icon = <SendIcon className="size-4" />;

  if (status === "submitted") {
    Icon = <Loader2Icon className="size-4 animate-spin" />;
  } else if (status === "streaming") {
    Icon = <SquareIcon className="size-4" />;
  } else if (status === "error") {
    Icon = <XIcon className="size-4" />;
  }

  return (
    <Button
      className={cn("gap-1.5 rounded-lg", className)}
      size={size}
      type="submit"
      variant={variant}
      {...props}
    >
      {children ?? Icon}
    </Button>
  );
};
export const PromptInputModelSelect = ModelSelect;
