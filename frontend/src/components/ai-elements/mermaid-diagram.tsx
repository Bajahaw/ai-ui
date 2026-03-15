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

function parseSvgDimensions(svgMarkup: string): { width: number; height: number } {
  const parser = new DOMParser();
  const doc = parser.parseFromString(svgMarkup, "image/svg+xml");
  const svg = doc.documentElement;

  const widthAttr = svg.getAttribute("width") ?? "";
  const heightAttr = svg.getAttribute("height") ?? "";
  const width = Number.parseFloat(widthAttr);
  const height = Number.parseFloat(heightAttr);

  if (Number.isFinite(width) && Number.isFinite(height) && width > 0 && height > 0) {
    return { width, height };
  }

  const viewBox = svg.getAttribute("viewBox");
  if (viewBox) {
    const parts = viewBox
      .trim()
      .split(/\s+/)
      .map((part) => Number.parseFloat(part));

    if (parts.length === 4 && Number.isFinite(parts[2]) && Number.isFinite(parts[3])) {
      return {
        width: Math.max(1, parts[2]),
        height: Math.max(1, parts[3]),
      };
    }
  }

  return { width: 1200, height: 800 };
}

export function MermaidDiagram({ code, className, ...props }: MermaidDiagramProps) {
  const [svg, setSvg] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [isCopying, setIsCopying] = useState(false);
  const [isDownloading, setIsDownloading] = useState(false);
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

  const downloadPng = async () => {
    if (!svg || isDownloading) {
      return;
    }

    setIsDownloading(true);
    try {
      const { width, height } = parseSvgDimensions(svg);
      const scale = 2;
      const canvas = document.createElement("canvas");
      canvas.width = Math.round(width * scale);
      canvas.height = Math.round(height * scale);

      const context = canvas.getContext("2d");
      if (!context) {
        throw new Error("Canvas rendering is not available");
      }

      context.setTransform(scale, 0, 0, scale, 0, 0);
      context.fillStyle = isDark ? "#0b0b0c" : "#ffffff";
      context.fillRect(0, 0, width, height);

      const svgBlob = new Blob([svg], { type: "image/svg+xml;charset=utf-8" });
      const objectUrl = URL.createObjectURL(svgBlob);

      try {
        const image = await new Promise<HTMLImageElement>((resolve, reject) => {
          const img = new Image();
          img.onload = () => resolve(img);
          img.onerror = () => reject(new Error("Failed to load rendered Mermaid SVG"));
          img.src = objectUrl;
        });

        context.drawImage(image, 0, 0, width, height);
      } finally {
        URL.revokeObjectURL(objectUrl);
      }

      const pngBlob = await new Promise<Blob>((resolve, reject) => {
        canvas.toBlob((blob) => {
          if (!blob) {
            reject(new Error("Failed to export diagram as PNG"));
            return;
          }
          resolve(blob);
        }, "image/png");
      });

      const downloadUrl = URL.createObjectURL(pngBlob);
      const anchor = document.createElement("a");
      anchor.href = downloadUrl;
      anchor.download = `mermaid-diagram-${Date.now()}.png`;
      anchor.click();
      URL.revokeObjectURL(downloadUrl);
    } catch (downloadError) {
      const message =
        downloadError instanceof Error ? downloadError.message : "Unable to save Mermaid as PNG";
      setError(message);
    } finally {
      setIsDownloading(false);
    }
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
            onClick={downloadPng}
            disabled={!svg || isDownloading}
            aria-label="Save Mermaid as PNG"
            title="Save Mermaid as PNG"
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