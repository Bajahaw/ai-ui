"use client";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { CheckIcon, CopyIcon, DownloadIcon, TriangleAlertIcon } from "lucide-react";
import { useEffect, useMemo, useState, type HTMLAttributes } from "react";

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
  const [isCopying, setIsCopying] = useState(false);
  const [isCopied, setIsCopied] = useState(false);
  const [isDark, setIsDark] = useState(false);

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

    setIsCopying(true);
    try {
      await navigator.clipboard.writeText(normalizedCode);
      setIsCopied(true);
      setTimeout(() => setIsCopied(false), 1800);
    } finally {
      setIsCopying(false);
    }
  };

  const downloadSvg = () => {
    if (!svg) {
      return;
    }

    const svgBlob = new Blob([svg], { type: "image/svg+xml;charset=utf-8" });
    const downloadUrl = URL.createObjectURL(svgBlob);
    const anchor = document.createElement("a");
    anchor.href = downloadUrl;
    anchor.download = `mermaid-diagram-${Date.now()}.svg`;
    anchor.click();
    URL.revokeObjectURL(downloadUrl);
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
          <Button
            className="h-7 w-7"
            size="icon"
            variant="ghost"
            onClick={copyRawCode}
            disabled={!normalizedCode || isCopying}
            aria-label="Copy Mermaid code"
            title="Copy Mermaid code"
          >
            {isCopied ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
          </Button>
          <Button
            className="h-7 w-7"
            size="icon"
            variant="ghost"
            onClick={downloadSvg}
            disabled={!svg}
            aria-label="Save Mermaid as SVG"
            title="Save Mermaid as SVG"
          >
            <DownloadIcon size={14} />
          </Button>
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