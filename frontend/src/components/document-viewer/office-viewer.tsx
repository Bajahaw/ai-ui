import { useEffect, useRef, useState } from "react";
import {
  AlertCircleIcon,
  Loader2Icon,
  MinusIcon,
  PlusIcon,
  RotateCcwIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import type { DocumentPreview } from "@/lib/document-preview";
import { createOfficeSurface, type OfficeSurface } from "./office-surface";

function loadError(error: unknown): string {
  const code = (error as { code?: string } | null)?.code;
  if (
    ["encrypted", "invalid-password", "unsupported-encryption"].includes(
      code ?? "",
    )
  )
    return "This file is password protected. Download it to open it in Office.";
  if (code === "ooxml-resource-limit" || code === "ooxml-decoded-image-limit")
    return "This file is too complex to preview safely. Download it to view it in Office.";
  return "Couldn’t load this Office file. It may be unavailable, invalid, or hosted on a site that blocks previews. Download it to view it in Office.";
}

export default function OfficeViewer({
  document,
}: {
  document: DocumentPreview;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  const surfaceRef = useRef<OfficeSurface | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [error, setError] = useState("");
  const [scale, setScale] = useState(1);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    let surface: OfficeSurface | undefined;
    const controller = new AbortController();
    // A distinct mount keeps late async work off a successor's DOM (StrictMode,
    // retries, or switching attachments while a parser is loading).
    const mount = window.document.createElement("div");
    mount.className = "h-full w-full";
    hostRef.current!.replaceChildren(mount);
    const dispose = () => {
      if (surfaceRef.current === surface) surfaceRef.current = null;
      surface?.destroy();
      surface = undefined;
      mount.remove();
    };
    const fail = (cause: unknown) => {
      if (cancelled || controller.signal.aborted) return;
      setError(loadError(cause));
      setStatus("error");
      controller.abort();
      dispose();
    };

    void (async () => {
      try {
        surface = await createOfficeSurface(
          mount,
          document.kind,
          fail,
          (value) => {
            if (!cancelled) setScale(value);
          },
        );
        if (cancelled || controller.signal.aborted) {
          dispose();
          return;
        }
        surfaceRef.current = surface;
        const response = await fetch(document.href, {
          // Send session cookies only to our own resource endpoint.
          credentials: "same-origin",
          signal: controller.signal,
        });
        if (!response.ok)
          throw new Error(`File request failed: ${response.status}`);
        const bytes = await response.arrayBuffer();
        if (cancelled) return;
        await surface.load(bytes);
        if (!cancelled && !controller.signal.aborted) setStatus("ready");
      } catch (cause) {
        fail(cause);
      }
    })();

    return () => {
      cancelled = true;
      controller.abort();
      dispose();
    };
  }, [document.href, document.kind, attempt]);

  return (
    <>
      <div className="flex min-h-11 shrink-0 items-center justify-between gap-2 border-b px-3 text-xs">
        <span className="text-muted-foreground">Read-only preview</span>
        <div
          className="flex items-center gap-1"
          role="group"
          aria-label="Office zoom"
        >
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            aria-label="Zoom out"
            disabled={status !== "ready"}
            onClick={() => surfaceRef.current?.zoomOut()}
          >
            <MinusIcon />
          </Button>
          <span className="w-10 text-center tabular-nums" aria-live="polite">
            {Math.round(scale * 100)}%
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            aria-label="Zoom in"
            disabled={status !== "ready"}
            onClick={() => surfaceRef.current?.zoomIn()}
          >
            <PlusIcon />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="ml-1"
            disabled={status !== "ready"}
            onClick={() => surfaceRef.current?.fitWidth()}
          >
            Fit width
          </Button>
        </div>
      </div>
      <div className="relative min-h-0 flex-1 overflow-hidden bg-muted/60">
        <div
          ref={hostRef}
          className="office-document-surface absolute inset-0 text-black"
          role="region"
          aria-label={`${document.kind.toUpperCase()} preview`}
          aria-busy={status === "loading"}
        />
        {status === "loading" && (
          <div
            className="absolute inset-0 flex items-center justify-center gap-2 bg-background/90 text-sm text-muted-foreground"
            role="status"
          >
            <Loader2Icon className="size-4 animate-spin" />
            Loading Office file…
          </div>
        )}
        {status === "error" && (
          <div
            className="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-background p-8 text-center"
            role="alert"
          >
            <AlertCircleIcon className="size-6 text-muted-foreground" />
            <p className="max-w-sm text-sm text-muted-foreground">{error}</p>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setStatus("loading");
                setAttempt((value) => value + 1);
              }}
            >
              <RotateCcwIcon />
              Try again
            </Button>
          </div>
        )}
      </div>
    </>
  );
}
