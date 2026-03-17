"use client";

import { cn } from "@/lib/utils";
import DOMPurify from "dompurify";
import { CheckIcon, CopyIcon, DownloadIcon, TriangleAlertIcon } from "lucide-react";
import { useEffect, useMemo, useState, type HTMLAttributes } from "react";
import { Action } from "./actions";
import { downloadTextFile, useCopyFeedback } from "./diagram-actions";

type SvgRendererProps = HTMLAttributes<HTMLDivElement> & {
  code: unknown;
};

function normalizeCode(code: unknown): string {
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
}

function sanitizeSvg(svgCode: string): string {
  const sanitized = DOMPurify.sanitize(svgCode, {
    USE_PROFILES: { svg: true, svgFilters: true },
    FORBID_TAGS: ["script", "foreignObject", "iframe", "object", "embed"],
    FORBID_ATTR: [
      "onload",
      "onerror",
      "onclick",
      "onmouseover",
      "onfocus",
      "onanimationstart",
      "onbegin",
      "onend",
      "href",
      "xlink:href",
    ],
  });

  if (!sanitized || typeof sanitized !== "string") {
    throw new Error("SVG sanitization failed");
  }

  const parser = new DOMParser();
  const doc = parser.parseFromString(sanitized, "image/svg+xml");
  const parserError = doc.querySelector("parsererror");
  if (parserError) {
    throw new Error("Invalid SVG markup");
  }

  const svgElement = doc.documentElement;
  if (!svgElement || svgElement.nodeName.toLowerCase() !== "svg") {
    throw new Error("SVG root element is missing");
  }

  return new XMLSerializer().serializeToString(svgElement);
}

export function SvgRenderer({ code, className, ...props }: SvgRendererProps) {
  const [svg, setSvg] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const { isCopied, isCopying, copyText } = useCopyFeedback(1800);

  const normalizedCode = useMemo(() => normalizeCode(code), [code]);

  useEffect(() => {
    let cancelled = false;

    const timer = setTimeout(() => {
      if (!normalizedCode) {
        setSvg("");
        setError(null);
        return;
      }

      try {
        const sanitizedSvg = sanitizeSvg(normalizedCode);

        if (cancelled) {
          return;
        }

        setSvg(sanitizedSvg);
        setError(null);
      } catch (sanitizeError) {
        if (cancelled) {
          return;
        }

        const message =
          sanitizeError instanceof Error ? sanitizeError.message : "Unable to render SVG";
        setError(message);
        setSvg("");
      }
    }, 180);

    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [normalizedCode]);

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

    downloadTextFile(svg, "image/svg+xml;charset=utf-8", `svg-diagram-${Date.now()}.svg`);
  };

  return (
    <div className={cn("group relative my-4 w-full", className)} {...props}>
      <div className="absolute right-1 top-1 z-10 flex gap-1 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100">
        <Action
          className="h-7 w-7"
          size="icon"
          onClick={copyRawCode}
          disabled={!normalizedCode || isCopying}
          label="Copy SVG code"
          tooltip="Copy SVG code"
        >
          {isCopied ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
        </Action>
        <Action
          className="h-7 w-7"
          size="icon"
          onClick={downloadSvg}
          disabled={!svg}
          label="Save SVG"
          tooltip="Save SVG"
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
