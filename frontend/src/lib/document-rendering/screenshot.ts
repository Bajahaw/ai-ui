import { getPreviewKind } from "@/lib/document-preview";
import type {
  ScreenshotOptions,
  ScreenshotRenderer,
  ScreenshotSource,
} from "./types";

// Format-neutral dispatch: a future PDF adapter belongs here, not in the
// sandbox bridge or thumbnail components. No renderer loads until requested.
const renderers: Partial<Record<string, () => Promise<ScreenshotRenderer>>> = {
  docx: () => import("./office").then((m) => m.renderOfficeScreenshot),
  xlsx: () => import("./office").then((m) => m.renderOfficeScreenshot),
  pptx: () => import("./office").then((m) => m.renderOfficeScreenshot),
};

export function screenshotKind(
  source: Pick<ScreenshotSource, "filename" | "mimeType">,
) {
  return getPreviewKind("/screenshot", source);
}

function integer(
  value: unknown,
  fallback: number,
  max: number,
  name: string,
): number {
  const result = value === undefined ? fallback : value;
  if (
    typeof result !== "number" ||
    !Number.isInteger(result) ||
    result < 1 ||
    result > max
  )
    throw new Error(`${name} must be an integer between 1 and ${max}`);
  return result;
}

// A single lane is shared by thumbnails and sandbox screenshots. Parsing many
// large documents concurrently is much costlier than downloading the package.
let tail: Promise<unknown> = Promise.resolve();

export function renderDocumentScreenshot(
  source: ScreenshotSource,
  options: ScreenshotOptions = {},
  signal?: AbortSignal,
): Promise<Blob> {
  const task = tail
    .catch(() => undefined)
    .then(async () => {
      signal?.throwIfAborted();
      if (
        !(source.data instanceof ArrayBuffer) ||
        !source.data.byteLength ||
        source.data.byteLength > 50 * 1024 * 1024
      )
        throw new Error(
          "Screenshot input must contain between 1 byte and 50MB",
        );
      const kind = screenshotKind(source);
      const loader = kind && renderers[kind];
      if (!loader)
        throw new Error(
          "Screenshots currently support DOCX, XLSX, and PPTX only",
        );
      const range = options.range;
      if (range && kind !== "xlsx")
        throw new Error("Cell ranges are only supported for XLSX screenshots");
      const resolved = {
        page: integer(options.page, 1, 1_048_576, "page"),
        width: integer(options.width, 1200, 2048, "width"),
        height: integer(options.height, 1600, 2048, "height"),
        range: {
          row: integer(range?.row, 1, 1_048_576, "range.row"),
          col: integer(range?.col, 1, 16_384, "range.col"),
          rows: integer(range?.rows, 20, 200, "range.rows"),
          cols: integer(range?.cols, 8, 100, "range.cols"),
        },
      };
      if (
        resolved.range.rows * resolved.range.cols > 10_000 ||
        resolved.range.row + resolved.range.rows - 1 > 1_048_576 ||
        resolved.range.col + resolved.range.cols - 1 > 16_384
      )
        throw new Error("Screenshot cell range exceeds limits");

      const controller = new AbortController();
      const abort = () => controller.abort(signal?.reason);
      signal?.addEventListener("abort", abort, { once: true });
      const timeout = setTimeout(
        () => controller.abort(new Error("Screenshot timed out")),
        30_000,
      );
      try {
        const render = await loader();
        controller.signal.throwIfAborted();
        return await render(source, resolved, controller.signal);
      } finally {
        clearTimeout(timeout);
        signal?.removeEventListener("abort", abort);
      }
    });
  // Keep the lane occupied until renderer teardown actually completes, even if
  // a caller stops waiting. This prevents overlapping cancelled parsers.
  tail = task.catch(() => undefined);
  return task;
}
