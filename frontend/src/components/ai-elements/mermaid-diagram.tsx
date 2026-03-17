"use client";

import { cn } from "@/lib/utils";
import { CheckIcon, CopyIcon, DownloadIcon, TriangleAlertIcon } from "lucide-react";
import { useEffect, useMemo, useState, type HTMLAttributes } from "react";
import { Action } from "./actions";
import { downloadTextFile, useCopyFeedback } from "./diagram-actions";

type MermaidDiagramProps = HTMLAttributes<HTMLDivElement> & {
  code: unknown;
};

type MermaidRuntime = {
  initialize: (config: {
    startOnLoad: boolean;
    securityLevel: "strict";
    theme: "default" | "dark";
    flowchart: {
      htmlLabels: boolean;
    };
  }) => void;
  render: (id: string, code: string) => Promise<{ svg: string }>;
};

function initializeMermaid(mermaid: MermaidRuntime, isDark: boolean) {
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    theme: isDark ? "dark" : "default",
    flowchart: {
      htmlLabels: false,
    },
  });
}

export function MermaidDiagram({ code, className, ...props }: MermaidDiagramProps) {
  const [svg, setSvg] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [isDark, setIsDark] = useState(false);
  const { isCopied, isCopying, copyText } = useCopyFeedback(1800);

  const normalizedCode = useMemo(() => {
    if (typeof code === "string") {
      return code.trim();
    }

    if (Array.isArray(code)) {
      return code
        .map((part) => (typeof part === "string" ? part : ""))
        .join("")
        .trim();
    }

    return "";
  }, [code]);

  useEffect(() => {
    const syncTheme = () => {
      const root = document.documentElement;

      // App theme classes take precedence over OS preference.
      if (root.classList.contains("dark")) {
        setIsDark(true);
        return;
      }

      if (root.classList.contains("light")) {
        setIsDark(false);
        return;
      }

      setIsDark(window.matchMedia("(prefers-color-scheme: dark)").matches);
    };

    syncTheme();

    const observer = new MutationObserver(syncTheme);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });

    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    mediaQuery.addEventListener("change", syncTheme);

    return () => {
      observer.disconnect();
      mediaQuery.removeEventListener("change", syncTheme);
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const timer = setTimeout(async () => {
      if (!normalizedCode) {
        setSvg("");
        setError(null);
        return;
      }

      try {
        const mod = await import("mermaid");
        const mermaid = mod.default;
        initializeMermaid(mermaid, isDark);

        const renderId = `mermaid-${Math.random().toString(36).slice(2, 10)}`;
        const renderResult = await mermaid.render(renderId, normalizedCode);

        if (cancelled) {
          return;
        }

        setSvg(renderResult.svg);
        setError(null);
      } catch (renderError) {
        if (cancelled) {
          return;
        }

        const message =
          renderError instanceof Error ? renderError.message : "Unable to render Mermaid diagram";
        setError(message);
        setSvg("");
      }
    }, 180);

    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [normalizedCode, isDark]);

  const copyRawCode = async () => {
    if (!normalizedCode || isCopying) {
      return;
    }

    await copyText(normalizedCode);
  };

  const downloadSvg = () => {
    if (!svg) {
      return;
    }

    downloadTextFile(
      svg,
      "image/svg+xml;charset=utf-8",
      `mermaid-diagram-${Date.now()}.svg`,
    );
  };

  return (
    <div
      className={cn(
        "group relative my-4 w-full",
        className,
      )}
      {...props}
    >
      <div className="absolute right-1 top-1 z-10 flex gap-1 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100">
        <Action
          className="h-7 w-7"
          size="icon"
          onClick={copyRawCode}
          disabled={!normalizedCode || isCopying}
          label="Copy Mermaid code"
          tooltip="Copy Mermaid code"
        >
          {isCopied ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
        </Action>
        <Action
          className="h-7 w-7"
          size="icon"
          onClick={downloadSvg}
          disabled={!svg}
          label="Save Mermaid as SVG"
          tooltip="Save Mermaid as SVG"
        >
          <DownloadIcon size={14} />
        </Action>
      </div>

      {error ? (
        <div className="space-y-2">
          <div className="inline-flex items-center gap-2 text-xs text-destructive">
            <TriangleAlertIcon size={14} />
            <span>{error}</span>
          </div>
          <pre className="overflow-x-auto whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">
            {normalizedCode}
          </pre>
        </div>
      ) : (
        <div
          className="overflow-auto pt-6 [&_svg]:mx-auto [&_svg]:h-auto [&_svg]:max-w-full"
          dangerouslySetInnerHTML={{ __html: svg }}
        />
      )}
    </div>
  );
}