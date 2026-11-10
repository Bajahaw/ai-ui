import { useEffect, useRef, useState, type RefObject } from "react";
import {
  getDocument,
  GlobalWorkerOptions,
  TextLayer,
  type PDFDocumentProxy,
  type PDFPageProxy,
  type RenderTask,
} from "pdfjs-dist";
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import {
  AlertCircleIcon,
  Loader2Icon,
  MinusIcon,
  PlusIcon,
  RotateCcwIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import type { DocumentPreview } from "@/lib/document-preview";

GlobalWorkerOptions.workerSrc = workerUrl;

function PdfPage({
  pdf,
  pageNumber,
  width,
  scrollRef,
}: {
  pdf: PDFDocumentProxy;
  pageNumber: number;
  width: number;
  scrollRef: RefObject<HTMLDivElement | null>;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasHostRef = useRef<HTMLDivElement>(null);
  const textHostRef = useRef<HTMLDivElement>(null);
  const [nearby, setNearby] = useState(pageNumber === 1);
  const [pageSize, setPageSize] = useState({ width: 612, height: 792 });
  const [error, setError] = useState(false);

  useEffect(() => {
    const observer = new IntersectionObserver(
      ([entry]) => setNearby(entry.isIntersecting),
      {
        root: scrollRef.current,
        rootMargin: "800px 0px",
      },
    );
    observer.observe(containerRef.current!);
    return () => observer.disconnect();
  }, [scrollRef]);

  useEffect(() => {
    if (!nearby || width <= 0) return;
    let cancelled = false;
    let renderTask: RenderTask | undefined;
    let textLayer: TextLayer | undefined;
    let page: PDFPageProxy | undefined;
    const canvasHost = canvasHostRef.current!;
    const textHost = textHostRef.current!;
    // Separate DOM nodes per render prevent a cancelled zoom render touching its successor.
    const canvas = window.document.createElement("canvas");
    const textContainer = window.document.createElement("div");
    textContainer.className = "pdf-text-layer";
    canvasHost.replaceChildren(canvas);
    textHost.replaceChildren(textContainer);

    void (async () => {
      try {
        page = await pdf.getPage(pageNumber);
        if (cancelled) return;
        const natural = page.getViewport({ scale: 1 });
        setPageSize({ width: natural.width, height: natural.height });
        const viewport = page.getViewport({ scale: width / natural.width });
        const ratio = Math.min(
          window.devicePixelRatio || 1,
          2,
          Math.sqrt(8_000_000 / (viewport.width * viewport.height)),
        );
        canvas.width = Math.max(1, Math.floor(viewport.width * ratio));
        canvas.height = Math.max(1, Math.floor(viewport.height * ratio));
        canvas.style.width = `${viewport.width}px`;
        canvas.style.height = `${viewport.height}px`;
        textContainer.style.setProperty(
          "--total-scale-factor",
          String(viewport.scale * viewport.userUnit),
        );
        renderTask = page.render({
          canvas,
          viewport,
          transform: [ratio, 0, 0, ratio, 0, 0],
        });
        await renderTask.promise;
        if (cancelled) return;
        textLayer = new TextLayer({
          textContentSource: page.streamTextContent(),
          container: textContainer,
          viewport,
        });
        await textLayer.render();
        if (!cancelled) setError(false);
      } catch {
        if (!cancelled) setError(true);
      }
    })();

    return () => {
      cancelled = true;
      renderTask?.cancel();
      textLayer?.cancel();
      canvas.remove();
      textContainer.remove();
      // Release large backing stores as pages leave the viewport.
      canvas.width = 0;
      canvas.height = 0;
    };
  }, [pdf, pageNumber, width, nearby]);

  return (
    <div
      ref={containerRef}
      className="pdf-document-page relative shrink-0 bg-white shadow-sm"
      style={{ width, height: (width * pageSize.height) / pageSize.width }}
      role="group"
      aria-label={`Page ${pageNumber}`}
    >
      <div ref={canvasHostRef} aria-hidden="true" />
      <div ref={textHostRef} />
      {error && (
        <p
          className="absolute inset-0 flex items-center justify-center p-6 text-center text-sm text-red-700"
          role="alert"
        >
          Couldn’t render page {pageNumber}. Download the PDF to view it.
        </p>
      )}
    </div>
  );
}

function LoadedPdf({ pdf }: { pdf: PDFDocumentProxy }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [availableWidth, setAvailableWidth] = useState(0);
  const [zoom, setZoom] = useState(1);

  useEffect(() => {
    const element = scrollRef.current!;
    const observer = new ResizeObserver(() =>
      setAvailableWidth(Math.max(0, element.clientWidth - 32)),
    );
    observer.observe(element);
    setAvailableWidth(Math.max(0, element.clientWidth - 32));
    return () => observer.disconnect();
  }, []);

  const pageWidth = availableWidth * zoom;
  const pageNumbers = Array.from(
    { length: pdf.numPages },
    (_, index) => index + 1,
  );

  return (
    <>
      <div className="flex min-h-11 shrink-0 items-center justify-between gap-2 border-b px-3 text-xs">
        <span className="text-muted-foreground">
          {pdf.numPages} {pdf.numPages === 1 ? "page" : "pages"}
        </span>
        <div
          className="flex items-center gap-1"
          role="group"
          aria-label="PDF zoom"
        >
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            aria-label="Zoom out"
            disabled={zoom <= 0.5}
            onClick={() => setZoom((value) => Math.max(0.5, value - 0.25))}
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
            disabled={zoom >= 3}
            onClick={() => setZoom((value) => Math.min(3, value + 0.25))}
          >
            <PlusIcon />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="ml-1"
            onClick={() => setZoom(1)}
            title="Fit to width"
          >
            Fit width
          </Button>
        </div>
      </div>
      <div
        ref={scrollRef}
        className="pdf-document-scroll min-h-0 flex-1 overflow-auto overscroll-contain bg-muted/60"
        tabIndex={0}
        aria-label="PDF pages"
      >
        <div className="flex min-w-full w-max flex-col items-center gap-4 p-4">
          {availableWidth > 0 &&
            pageNumbers.map((pageNumber) => (
              <PdfPage
                key={pageNumber}
                pdf={pdf}
                pageNumber={pageNumber}
                width={pageWidth}
                scrollRef={scrollRef}
              />
            ))}
        </div>
      </div>
    </>
  );
}

export default function PdfViewer({ document }: { document: DocumentPreview }) {
  const [pdf, setPdf] = useState<PDFDocumentProxy | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const assets = `${import.meta.env.BASE_URL}pdfjs/`;
    const task = getDocument({
      url: document.href,
      // Same-origin resource requests use the session cookie by default; never
      // forward credentials to external PDF hosts.
      withCredentials: false,
      cMapUrl: `${assets}cmaps/`,
      cMapPacked: true,
      standardFontDataUrl: `${assets}standard_fonts/`,
      wasmUrl: `${assets}wasm/`,
    });
    task.onPassword = () => {
      if (!cancelled)
        setError(
          "This PDF is password protected. Download it to open it with your password.",
        );
      void task.destroy();
    };
    void task.promise
      .then((loaded) => {
        if (!cancelled) setPdf(loaded);
      })
      .catch(() => {
        if (!cancelled)
          setError(
            (current) =>
              current ??
              "Couldn’t load this PDF. The file may be unavailable, invalid, or hosted on a site that blocks previews.",
          );
      });
    return () => {
      cancelled = true;
      void task.destroy();
    };
  }, [document.href, attempt]);

  if (error)
    return (
      <div
        className="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center"
        role="alert"
      >
        <AlertCircleIcon className="size-6 text-muted-foreground" />
        <p className="max-w-sm text-sm text-muted-foreground">{error}</p>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            setError(null);
            setPdf(null);
            setAttempt((value) => value + 1);
          }}
        >
          <RotateCcwIcon />
          Try again
        </Button>
      </div>
    );
  if (!pdf)
    return (
      <div
        className="flex flex-1 items-center justify-center gap-2 text-sm text-muted-foreground"
        role="status"
      >
        <Loader2Icon className="size-4 animate-spin" />
        Loading PDF…
      </div>
    );
  return <LoadedPdf pdf={pdf} />;
}
