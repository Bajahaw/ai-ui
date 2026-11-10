import { useEffect, useRef, useState } from "react";
import {
  AlertCircleIcon,
  Loader2Icon,
  MinusIcon,
  PlusIcon,
  RotateCcwIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { DocumentPreview } from "@/lib/document-preview";

export function ImageViewer({ document }: { document: DocumentPreview }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [bounds, setBounds] = useState({ width: 0, height: 0 });
  const [size, setSize] = useState<{ width: number; height: number } | null>(
    null,
  );
  const [zoom, setZoom] = useState(1);
  const [error, setError] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const element = scrollRef.current!;
    const measure = () =>
      setBounds({
        width: Math.max(0, element.clientWidth - 32),
        height: Math.max(0, element.clientHeight - 32),
      });
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    measure();
    return () => observer.disconnect();
  }, []);

  const fit = size
    ? Math.min(1, bounds.width / size.width, bounds.height / size.height)
    : 1;
  const ready = !!size && !error;

  return (
    <>
      <div className="flex min-h-11 shrink-0 items-center justify-between gap-2 border-b px-3 text-xs">
        <span className="min-w-0 truncate text-muted-foreground tabular-nums">
          {size ? `${size.width} × ${size.height}` : "Image"}
        </span>
        <div
          className="flex items-center gap-1"
          role="group"
          aria-label="Image zoom"
        >
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            aria-label="Zoom out"
            disabled={!ready || zoom <= 0.25}
            onClick={() => setZoom((value) => Math.max(0.25, value - 0.25))}
          >
            <MinusIcon />
          </Button>
          <span className="w-10 text-center tabular-nums" aria-live="polite">
            {Math.round(zoom * 100)}%
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            aria-label="Zoom in"
            disabled={!ready || zoom >= 4}
            onClick={() => setZoom((value) => Math.min(4, value + 0.25))}
          >
            <PlusIcon />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="ml-1"
            disabled={!ready}
            onClick={() => {
              setZoom(1);
              scrollRef.current?.scrollTo({ top: 0, left: 0 });
            }}
            title="Fit image to panel"
          >
            Fit image
          </Button>
        </div>
      </div>
      <div
        ref={scrollRef}
        className="relative min-h-0 flex-1 overflow-auto overscroll-contain bg-muted/60"
        tabIndex={0}
        aria-label="Image preview"
      >
        <div className="flex min-h-full min-w-full w-max items-center justify-center p-4">
          <img
            key={attempt}
            src={document.href}
            alt={document.filename}
            className={cn(
              "block max-w-none shrink-0",
              !ready && "invisible absolute",
            )}
            style={
              size
                ? {
                    width: size.width * fit * zoom,
                    height: size.height * fit * zoom,
                  }
                : undefined
            }
            onLoad={(event) => {
              const image = event.currentTarget;
              if (!image.naturalWidth || !image.naturalHeight) {
                setError(true);
                return;
              }
              setSize({
                width: image.naturalWidth,
                height: image.naturalHeight,
              });
              setError(false);
            }}
            onError={() => setError(true)}
          />
        </div>
        {!size && !error && (
          <div
            className="absolute inset-0 flex items-center justify-center gap-2 text-sm text-muted-foreground"
            role="status"
          >
            <Loader2Icon className="size-4 animate-spin" />
            Loading image…
          </div>
        )}
        {error && (
          <div
            className="absolute inset-0 flex flex-col items-center justify-center gap-3 p-8 text-center"
            role="alert"
          >
            <AlertCircleIcon
              className="size-6 text-muted-foreground"
              aria-hidden="true"
            />
            <p className="max-w-sm text-sm text-muted-foreground">
              Couldn’t load this image. Try again or download it to view it.
            </p>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setError(false);
                setSize(null);
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
