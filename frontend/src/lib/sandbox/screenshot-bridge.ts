import { renderDocumentScreenshot } from "@/lib/document-rendering/screenshot";
import type { ScreenshotOptions } from "@/lib/document-rendering/types";

type Reply = (
  message: Record<string, unknown>,
  transfer?: Transferable[],
) => void;

/** Narrow capability: accepts bytes, never a URL, file ID, or authenticated API request. */
export function createScreenshotBridge(reply: Reply, signal: AbortSignal) {
  let requests = 0;
  let inputBytes = 0;
  const seen = new Set<number>();
  return (message: Record<string, unknown>): boolean => {
    if (message.type !== "__sandbox_screenshot") return false;
    const requestId = message.requestId;
    if (
      signal.aborted ||
      typeof requestId !== "number" ||
      !Number.isSafeInteger(requestId) ||
      requestId < 1 ||
      seen.has(requestId)
    )
      return true;
    const respond = (
      data: Record<string, unknown>,
      transfer?: Transferable[],
    ) => {
      if (!signal.aborted)
        reply(
          { type: "__sandbox_screenshot_result", requestId, ...data },
          transfer,
        );
    };
    if (++requests > 6) {
      respond({ error: "Maximum 6 screenshots per sandbox run" });
      return true;
    }
    seen.add(requestId);
    const { data, filename, options } = message;
    if (
      !(data instanceof ArrayBuffer) ||
      !data.byteLength ||
      data.byteLength > 50 * 1024 * 1024 ||
      typeof filename !== "string" ||
      !filename ||
      filename.length > 255 ||
      (options !== undefined &&
        (!options || typeof options !== "object" || Array.isArray(options)))
    ) {
      respond({ error: "Invalid screenshot request" });
      return true;
    }
    inputBytes += data.byteLength;
    if (inputBytes > 100 * 1024 * 1024) {
      respond({ error: "Screenshot inputs exceed 100MB per run" });
      return true;
    }
    void renderDocumentScreenshot(
      { data, filename },
      options as ScreenshotOptions | undefined,
      signal,
    )
      .then(async (image) => {
        const bytes = await image.arrayBuffer();
        respond({ data: bytes }, [bytes]);
      })
      .catch((error: unknown) => {
        respond({
          error: (error instanceof Error
            ? error.message
            : "Screenshot failed"
          ).slice(0, 1000),
        });
      });
    return true;
  };
}
