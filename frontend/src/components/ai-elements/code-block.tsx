"use client";

import { CheckIcon, CopyIcon } from "lucide-react";
import type { ComponentProps, HTMLAttributes, ReactNode } from "react";
import { createContext, useContext, useState, lazy, Suspense, useEffect } from "react";
// import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import {
  oneDark,
  oneLight,
} from "react-syntax-highlighter/dist/cjs/styles/prism";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const ENABLE_HIGHLIGHTING = true; // Disable highlighting in dev for performance

const SyntaxHighlighter = lazy(() =>
  import("react-syntax-highlighter").then((mod) => ({ default: mod.PrismAsyncLight })),
);

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

  useEffect(() => {
    // Small delay to ensure raw code is rendered and visible first,
    // especially for large code blocks that might block the UI during highlighting.
    const timer = setTimeout(() => {
      setShowHighlight(true);
    }, 10);
    return () => clearTimeout(timer);
  }, []);

  return (
    <CodeBlockContext.Provider value={{ code }}>
      <div
        className={cn(
          "relative w-full overflow-hidden rounded-xl border bg-[hsl(var(--code-background))] text-foreground",
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
          {ENABLE_HIGHLIGHTING && showHighlight ? (
            <Suspense
              fallback={
                <pre className="overflow-x-auto p-4 pt-0 font-mono text-base">
                  {code}
                </pre>
              }
            >
              <SyntaxHighlighter
                language={language}
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
              </SyntaxHighlighter>
              <SyntaxHighlighter
                language={language}
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
              </SyntaxHighlighter>
            </Suspense>
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
