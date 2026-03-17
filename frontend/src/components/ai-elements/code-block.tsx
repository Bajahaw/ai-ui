"use client";

import { CheckIcon, CopyIcon } from "lucide-react";
import type { ComponentProps, HTMLAttributes, ReactNode } from "react";
import { createContext, useContext, useEffect, useMemo, useState } from "react";
import PrismAsyncLight from "react-syntax-highlighter/dist/esm/prism-async-light";
import {
  oneDark,
  oneLight,
} from "react-syntax-highlighter/dist/esm/styles/prism";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const ENABLE_HIGHLIGHTING = true; // Disable highlighting in dev for performance

type PrismAsyncLightWithSupported = typeof PrismAsyncLight & {
  supportedLanguages?: string[];
};

const prismSupportedLanguages =
  (PrismAsyncLight as PrismAsyncLightWithSupported).supportedLanguages ?? [];

const toKebabCase = (value: string): string =>
  value
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .replace(/[_\s]+/g, "-")
    .toLowerCase();

const toCompactKey = (value: string): string =>
  toKebabCase(value).replace(/[^a-z0-9]/g, "");

const prismLanguageLookup = (() => {
  const lookup = new Map<string, string>();

  for (const language of prismSupportedLanguages) {
    lookup.set(toKebabCase(language), language);
    lookup.set(toCompactKey(language), language);
  }

  return lookup;
})();

const normalizeLanguage = (language: string): string => {
  const normalized = language.trim().replace(/^language-/, "");
  if (!normalized) {
    return "text";
  }

  return toKebabCase(normalized);
};

const resolvePrismLanguage = (language: string): string | null => {
  const normalized = normalizeLanguage(language);
  return prismLanguageLookup.get(normalized) ?? prismLanguageLookup.get(toCompactKey(normalized)) ?? null;
};

type CodeBlockContextType = {
  code: string;
};

const CodeBlockContext = createContext<CodeBlockContextType>({
  code: "",
});

export type CodeBlockProps = HTMLAttributes<HTMLDivElement> & {
  code: string;
  language: string;
  showLineNumbers?: boolean;
  children?: ReactNode;
};

export const CodeBlock = ({
  code,
  language,
  showLineNumbers = false,
  className,
  children,
  ...props
}: CodeBlockProps) => {
  const [showHighlight, setShowHighlight] = useState(false);
  const [resolvedLanguage, setResolvedLanguage] = useState("text");

  const normalizedLanguage = useMemo(() => normalizeLanguage(language), [language]);

  useEffect(() => {
    // Small delay to ensure raw code is rendered and visible first,
    // especially for large code blocks that might block the UI during highlighting.
    const timer = setTimeout(() => {
      setShowHighlight(true);
    }, 10);
    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    const nextLanguage = resolvePrismLanguage(normalizedLanguage);
    setResolvedLanguage(nextLanguage ?? "text");
  }, [normalizedLanguage]);

  const canHighlight = ENABLE_HIGHLIGHTING && showHighlight;

  return (
    <CodeBlockContext.Provider value={{ code }}>
      <div
        className={cn(
          "relative w-full max-w-full overflow-hidden rounded-xl border bg-[hsl(var(--code-background))] text-foreground",
          className,
        )}
        {...props}
      >
        <div className="flex items-center justify-between p-1 pb-0">
          <span className="text-xs font-medium text-muted-foreground px-3">
            {language}
          </span>
          <div className="flex items-center gap-2">{children}</div>
        </div>
        <div className="relative">
          {canHighlight ? (
            <>
              <PrismAsyncLight
                language={resolvedLanguage}
                style={oneLight}
                customStyle={{
                  margin: 0,
                  padding: "0 1rem 1rem",
                  fontSize: "1rem",
                  background: "hsl(var(--code-background))",
                  color: "hsl(var(--foreground))",
                  overflowX: "auto",
                  maxWidth: "100%",
                }}
                showLineNumbers={showLineNumbers}
                lineNumberStyle={{
                  color: "hsl(var(--muted-foreground))",
                  paddingRight: "1rem",
                  minWidth: "2.5rem",
                }}
                codeTagProps={{
                  className: "font-mono text-base",
                }}
                className="dark:hidden overflow-x-auto"
                wrapLongLines={false}
              >
                {code}
              </PrismAsyncLight>
              <PrismAsyncLight
                language={resolvedLanguage}
                style={oneDark}
                customStyle={{
                  margin: 0,
                  padding: "0 1rem 1rem",
                  fontSize: "1rem",
                  background: "hsl(var(--code-background))",
                  color: "hsl(var(--foreground))",
                  overflowX: "auto",
                  maxWidth: "100%",
                }}
                showLineNumbers={showLineNumbers}
                lineNumberStyle={{
                  color: "hsl(var(--muted-foreground))",
                  paddingRight: "1rem",
                  minWidth: "2.5rem",
                }}
                codeTagProps={{
                  className: "font-mono text-base",
                }}
                className="hidden dark:block overflow-x-auto"
                wrapLongLines={false}
              >
                {code}
              </PrismAsyncLight>
            </>
          ) : (
            <pre className="overflow-x-auto p-4 pt-0 font-mono text-base">
              {code}
            </pre>
          )}
        </div>
      </div>
    </CodeBlockContext.Provider>
  );
};

export type CodeBlockCopyButtonProps = ComponentProps<typeof Button> & {
  onCopy?: () => void;
  onError?: (error: Error) => void;
  timeout?: number;
};

export const CodeBlockCopyButton = ({
  onCopy,
  onError,
  timeout = 2000,
  children,
  className,
  ...props
}: CodeBlockCopyButtonProps) => {
  const [isCopied, setIsCopied] = useState(false);
  const { code } = useContext(CodeBlockContext);

  const copyToClipboard = async () => {
    if (typeof window === "undefined" || !navigator.clipboard.writeText) {
      onError?.(new Error("Clipboard API not available"));
      return;
    }

    try {
      await navigator.clipboard.writeText(code);
      setIsCopied(true);
      onCopy?.();
      setTimeout(() => setIsCopied(false), timeout);
    } catch (error) {
      onError?.(error as Error);
    }
  };

  const Icon = isCopied ? CheckIcon : CopyIcon;

  return (
    <Button
      className={cn("shrink-0", className)}
      onClick={copyToClipboard}
      size="icon"
      variant="ghost"
      {...props}
    >
      {children ?? <Icon size={14} />}
    </Button>
  );
};
