"use client";

import {Loader2Icon, SendIcon, SquareIcon, XIcon} from "lucide-react";
import type {ComponentProps, HTMLAttributes, KeyboardEventHandler,} from "react";
import {Children, forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState} from "react";
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
      "w-full divide-y overflow-hidden rounded-3xl border bg-secondary/50 shadow-sm",
      className,
    )}
    {...props}
  />
);

export type PromptInputTextareaHandle = {
  focus(options?: FocusOptions): void;
  clear(): void;
  readonly value: string;
};

export type PromptInputTextareaProps = ComponentProps<typeof Textarea> & {
  minHeight?: number;
  maxHeight?: number;
  onFilesPasted?: (files: File[]) => void;
};

export const PromptInputTextarea = forwardRef<PromptInputTextareaHandle, PromptInputTextareaProps>(({
  onChange,
  onFilesPasted,
  className,
  placeholder = "What would you like to know?",
  value,
  minHeight = 24,
  maxHeight = 200,
  ...props
}, ref) => {
  const isControlled = value !== undefined;
  const [internalValue, setInternalValue] = useState(value as string || "");
  const inputValue = (isControlled ? value : internalValue) as string;
  const { settings } = useSettings();
  const internalRef = useRef<HTMLTextAreaElement>(null);

  const adjustHeight = useCallback(() => {
    const el = internalRef.current;
    if (!el) return;

    // Record the current rendered height so we can animate pixel → pixel.
    const currentHeight = el.getBoundingClientRect().height;

    // Temporarily disable transitions and collapse to "auto" to measure content.
    el.style.transition = "none";
    el.style.height = "auto";
    const scrollH = el.scrollHeight;
    const next = Math.min(Math.max(scrollH, minHeight), maxHeight);

    // Restore the previous pixel height (no visible change yet).
    el.style.height = `${currentHeight}px`;

    // Force a reflow so the browser registers the restored height.
    void el.offsetHeight;

    // Re-enable the CSS transition, then set the target height to animate.
    el.style.transition = "";
    el.style.height = `${next}px`;
    el.style.overflowY = scrollH > maxHeight ? "auto" : "hidden";
  }, [minHeight, maxHeight]);

  useEffect(() => {
    adjustHeight();
  }, [inputValue, adjustHeight]);

  useImperativeHandle(ref, () => ({
    focus: (options?: FocusOptions) => internalRef.current?.focus(options),
    clear: () => setInternalValue(""),
    get value() { return internalRef.current?.value ?? ""; },
  }), []);

  const setRefs = useCallback((el: HTMLTextAreaElement | null) => {
    (internalRef as React.MutableRefObject<HTMLTextAreaElement | null>).current = el;
  }, []);

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
    if (!isControlled) {
      setInternalValue(e.target.value);
    }
    onChange?.(e);
  };

  return (
    <Textarea
      rows={1}
      ref={setRefs}
      className={cn(
        "w-full resize-none rounded-none border-none py-3 px-5 shadow-none outline-none ring-0",
        "bg-transparent dark:bg-transparent overflow-hidden transition-[height] duration-150 ease-in-out",
        "focus-visible:ring-0",
        getTextDirection(inputValue),
        className,
      )}
      name="message"
      onChange={handleChange}
      onKeyDown={handleKeyDown}
      onPaste={handlePaste}
      placeholder={placeholder}
      value={inputValue}
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
    className={cn("flex items-center justify-between border-none py-1.5 px-1.5", className)}
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
        "shrink-0 gap-1.5 rounded-full",
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
  onStop?: () => void;
};

export const PromptInputSubmit = ({
  className,
  variant = "default",
  size = "icon",
  status,
  children,
  onStop,
  ...props
}: PromptInputSubmitProps) => {
  let Icon = <SendIcon className="size-4" />;

  if (status === "submitted") {
    Icon = <Loader2Icon className="size-4 animate-spin" />;
  } else if (status === "streaming") {
    Icon = <SquareIcon className="size-4 fill-current" />;
  } else if (status === "error") {
    Icon = <XIcon className="size-4" />;
  }

  if (status === "streaming" && onStop) {
    return (
      <Button
        className={cn("gap-1.5 rounded-full", className)}
        size={size}
        type="button"
        variant={variant}
        onClick={onStop}
        {...props}
      >
        {children ?? Icon}
      </Button>
    );
  }

  return (
    <Button
      className={cn("gap-1.5 rounded-full", className)}
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
