import { cn } from "@/lib/utils";
import {
  CheckIcon,
  CopyIcon,
  DownloadIcon,
  ExternalLinkIcon,
  TriangleAlertIcon,
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type HTMLAttributes,
} from "react";
import { Action } from "./actions";
import { downloadTextFile, useCopyFeedback } from "./diagram-actions";

type WidgetRendererProps = HTMLAttributes<HTMLDivElement> & {
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

const THEME_VARS = [
  "background",
  "foreground",
  "card",
  "card-foreground",
  "popover",
  "popover-foreground",
  "primary",
  "primary-foreground",
  "secondary",
  "secondary-foreground",
  "muted",
  "muted-foreground",
  "accent",
  "accent-foreground",
  "destructive",
  "destructive-foreground",
  "border",
  "input",
  "ring",
  "chart-1",
  "chart-2",
  "chart-3",
  "chart-4",
  "chart-5",
] as const;

function readThemeVars(): Record<string, string> {
  const style = getComputedStyle(document.documentElement);
  const vars: Record<string, string> = {};
  for (const name of THEME_VARS) {
    const value = style.getPropertyValue(`--${name}`).trim();
    if (value) {
      vars[name] = value;
    }
  }
  return vars;
}

function hslString(raw: string): string {
  return `hsl(${raw})`;
}

function buildSrcdoc(code: string, isDark: boolean): string {
  const vars = readThemeVars();

  const cssVarBlock = Object.entries(vars)
    .map(([k, v]) => `  --${k}: hsl(${v});`)
    .join("\n");

  const themeJs = Object.entries(vars)
    .map(([k, v]) => `  '${k}': '${hslString(v)}'`)
    .join(",\n");

  return `<!DOCTYPE html>
<html class="${isDark ? "dark" : ""}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
:root {
${cssVarBlock}
}
*, *::before, *::after { box-sizing: border-box; }
body {
  margin: 0;
  padding: 16px;
  background: var(--background);
  color: var(--foreground);
  font-family: ui-sans-serif, -apple-system, system-ui, 'Segoe UI', Helvetica, Arial, sans-serif;
  font-size: 14px;
  font-weight: 400;
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
  scrollbar-width: none;
  -ms-overflow-style: none;
}
::-webkit-scrollbar { display: none; }
</style>
</head>
<body>
<script>
const __theme = {
  isDark: ${isDark},
${themeJs}
};
</script>
${code}
<script>
(function() {
  var lastH = 0;
  function sendHeight() {
    var h = Math.max(
      document.body.scrollHeight,
      document.body.offsetHeight,
      document.documentElement.scrollHeight
    );
    if (h !== lastH && h > 0) {
      lastH = h;
      window.parent.postMessage({ type: '__widget_resize', height: h }, '*');
    }
  }

  // ResizeObserver is the most reliable way to detect layout changes
  if (typeof ResizeObserver !== 'undefined') {
    new ResizeObserver(sendHeight).observe(document.body);
  }

  // MutationObserver for DOM changes
  new MutationObserver(sendHeight).observe(document.body, {
    childList: true, subtree: true, attributes: true
  });

  // Initial measurement
  sendHeight();
  window.addEventListener('load', sendHeight);

  // Capture console for error reporting
  var origError = console.error;
  console.error = function() {
    origError.apply(console, arguments);
    window.parent.postMessage({
      type: '__widget_error',
      message: Array.from(arguments).join(' ')
    }, '*');
  };

  // Catch unhandled errors
  window.addEventListener('error', function(e) {
    window.parent.postMessage({
      type: '__widget_error',
      message: e.message || 'Runtime error'
    }, '*');
  });
})();
</script>
</body>
</html>`;
}

export function WidgetRenderer({
  code,
  className,
  ...props
}: WidgetRendererProps) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const lastHeight = useRef(200);
  const [iframeHeight, setIframeHeight] = useState(200);
  const [error, setError] = useState<string | null>(null);
  const [isDark, setIsDark] = useState(false);
  const { isCopied, isCopying, copyText } = useCopyFeedback(1800);

  const normalizedCode = useMemo(() => normalizeCode(code), [code]);

  // Sync theme from parent document
  useEffect(() => {
    const syncTheme = () => {
      const root = document.documentElement;
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

  // Listen for messages from iframe
  useEffect(() => {
    const handler = (e: MessageEvent) => {
      if (!e.data || typeof e.data !== "object") return;

      if (
        e.data.type === "__widget_resize" &&
        typeof e.data.height === "number"
      ) {
        const h = Math.min(Math.max(e.data.height, 60), 2000);
        lastHeight.current = h;
        setIframeHeight(h);
      }

      if (
        e.data.type === "__widget_error" &&
        typeof e.data.message === "string"
      ) {
        setError(e.data.message);
      }
    };

    window.addEventListener("message", handler);
    return () => window.removeEventListener("message", handler);
  }, []);

  const [srcdoc, setSrcdoc] = useState("");

  useEffect(() => {
    if (!normalizedCode) {
      setSrcdoc("");
      return;
    }

    const timer = setTimeout(() => {
      setError(null);
      setSrcdoc(buildSrcdoc(normalizedCode, isDark));
    }, 300);

    return () => clearTimeout(timer);
  }, [normalizedCode, isDark]);

  const copyRawCode = useCallback(async () => {
    if (!normalizedCode || isCopying) return;
    await copyText(normalizedCode);
  }, [normalizedCode, isCopying, copyText]);

  const downloadHtml = useCallback(() => {
    if (!srcdoc) return;
    downloadTextFile(
      srcdoc,
      "text/html;charset=utf-8",
      `widget-${Date.now()}.html`,
    );
  }, [srcdoc]);

  const openInNewTab = useCallback(() => {
    if (!srcdoc) return;
    const blob = new Blob([srcdoc], { type: "text/html" });
    const url = URL.createObjectURL(blob);
    window.open(url, "_blank");
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }, [srcdoc]);

  if (!normalizedCode) return null;

  return (
    <div className={cn("group relative my-4 w-full", className)} {...props}>
      <div className="absolute right-1 top-1 z-10 flex gap-1 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100">
        <Action
          className="h-7 w-7"
          size="icon"
          onClick={copyRawCode}
          disabled={!normalizedCode || isCopying}
          label="Copy widget code"
          tooltip="Copy code"
        >
          {isCopied ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
        </Action>
        <Action
          className="h-7 w-7"
          size="icon"
          onClick={downloadHtml}
          disabled={!srcdoc}
          label="Download as HTML"
          tooltip="Download HTML"
        >
          <DownloadIcon size={14} />
        </Action>
        <Action
          className="h-7 w-7"
          size="icon"
          onClick={openInNewTab}
          disabled={!srcdoc}
          label="Open in new tab"
          tooltip="Open in new tab"
        >
          <ExternalLinkIcon size={14} />
        </Action>
      </div>

      {error && (
        <div className="mb-2 inline-flex items-center gap-2 text-xs text-destructive">
          <TriangleAlertIcon size={14} />
          <span>{error}</span>
        </div>
      )}

      {srcdoc ? (
        <iframe
          ref={iframeRef}
          srcDoc={srcdoc}
          sandbox="allow-scripts"
          className="w-full rounded-xl border border-border"
          style={{
            height: `${iframeHeight}px`,
            overflow: "hidden",
            border: "none",
          }}
          title="Widget"
        />
      ) : (
        <div
          className="w-full rounded-xl border border-border bg-background"
          style={{ height: `${iframeHeight}px` }}
        />
      )}
    </div>
  );
}
