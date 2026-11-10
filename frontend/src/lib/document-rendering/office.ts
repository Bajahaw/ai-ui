import { getPreviewKind } from "@/lib/document-preview";
import type { ScreenshotRenderer } from "./types";

interface Disposable {
  destroy(): void;
}

async function withDocument<T extends Disposable>(
  load: () => Promise<T>,
  signal: AbortSignal,
  render: (document: T) => Promise<Blob>,
): Promise<Blob> {
  signal.throwIfAborted();
  const document = await load();
  let destroyed = false;
  const dispose = () => {
    if (destroyed) return;
    destroyed = true;
    document.destroy();
  };
  const abort = dispose;
  signal.addEventListener("abort", abort, { once: true });
  try {
    signal.throwIfAborted();
    const blob = await render(document);
    signal.throwIfAborted();
    return blob;
  } finally {
    signal.removeEventListener("abort", abort);
    dispose();
  }
}

async function encode(bitmap: ImageBitmap): Promise<Blob> {
  try {
    if (!(bitmap.width > 0 && bitmap.height > 0) || bitmap.width > 2048 || bitmap.height > 2048)
      throw new Error("Rendered screenshot dimensions exceed limits");
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height);
    const context = canvas.getContext("2d");
    if (!context) throw new Error("Could not create screenshot canvas");
    context.fillStyle = "white";
    context.fillRect(0, 0, bitmap.width, bitmap.height);
    context.drawImage(bitmap, 0, 0);
    return await canvas.convertToBlob({ type: "image/png" });
  } finally {
    bitmap.close();
  }
}

function fittedWidth(
  width: number,
  height: number,
  naturalWidth: number,
  naturalHeight: number,
) {
  if (!(naturalWidth > 0 && naturalHeight > 0) || !Number.isFinite(naturalWidth) || !Number.isFinite(naturalHeight))
    throw new Error("Invalid document dimensions");
  const fitted = Math.floor(Math.min(width, (height * naturalWidth) / naturalHeight));
  if (fitted < 1) throw new Error("Document aspect ratio exceeds screenshot limits");
  return fitted;
}

export const renderOfficeScreenshot: ScreenshotRenderer = async (
  source,
  options,
  signal,
) => {
  if (typeof Worker === "undefined" || typeof OffscreenCanvas === "undefined")
    throw new Error(
      "Office screenshots require a browser with Worker and OffscreenCanvas support",
    );
  const loadOptions = {
    mode: "worker" as const,
    workerTimeoutMs: 20_000,
    resourceLimits: {
      maxArchiveEntryBytes: 32 * 1024 * 1024,
      maxTotalInflatedBytes: 100 * 1024 * 1024,
      maxArchiveEntries: 5000,
    },
    xlsxWorksheetLimits: { maxRows: 100_000, maxCells: 200_000 },
  };
  const pageIndex = options.page - 1;
  // The parser may transfer its input buffer to a worker. Never detach the
  // shared upload/sandbox byte cache or mutate an original document.
  const bytes = source.data.slice(0);
  switch (getPreviewKind("/screenshot", source)) {
    case "docx": {
      const { DocxDocument } = await import("@silurus/ooxml/docx");
      return withDocument(
        () => DocxDocument.load(bytes, loadOptions),
        signal,
        async (doc) => {
          if (pageIndex >= doc.pageCount)
            throw new Error("Page number is out of range");
          const size = doc.pageSize(pageIndex);
          const width = fittedWidth(
            options.width,
            options.height,
            size.widthPt,
            size.heightPt,
          );
          return encode(
            await doc.renderPageToBitmap(pageIndex, { width, dpr: 1 }),
          );
        },
      );
    }
    case "pptx": {
      const { PptxPresentation } = await import("@silurus/ooxml/pptx");
      return withDocument(
        () => PptxPresentation.load(bytes, loadOptions),
        signal,
        async (doc) => {
          if (pageIndex >= doc.slideCount)
            throw new Error("Slide number is out of range");
          const width = fittedWidth(
            options.width,
            options.height,
            doc.slideWidth,
            doc.slideHeight,
          );
          return encode(
            await doc.renderSlideToBitmap(pageIndex, { width, dpr: 1 }),
          );
        },
      );
    }
    case "xlsx": {
      const { XlsxWorkbook } = await import("@silurus/ooxml/xlsx");
      return withDocument(
        () => XlsxWorkbook.load(bytes, loadOptions),
        signal,
        async (doc) => {
          if (pageIndex >= doc.sheetCount)
            throw new Error("Worksheet number is out of range");
          return encode(
            await doc.renderViewportToBitmap(pageIndex, options.range, {
              width: options.width,
              height: options.height,
              dpr: 1,
            }),
          );
        },
      );
    }
    default:
      throw new Error("Unsupported Office screenshot format");
  }
};
