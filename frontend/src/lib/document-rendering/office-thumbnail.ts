import {
  cachedFileBytes,
  fileResourceUrl,
  fileThumbnailUrl,
  uploadFileThumbnail,
} from "@/lib/api/files";
import { getSessionId } from "@/lib/api/headers";
import type { File as ApiFile } from "@/lib/api/types";
import { renderDocumentScreenshot } from "./screenshot";

export type ThumbnailFile = Pick<ApiFile, "id" | "name" | "type" | "path">;

type Entry = {
  controller: AbortController;
  promise: Promise<string>;
  users: number;
  settled: boolean;
  retryAfter: number;
};
const entries = new Map<string, Entry>();
let thumbnailTail: Promise<unknown> = Promise.resolve();

async function ensureThumbnail(
  file: ThumbnailFile,
  signal: AbortSignal,
): Promise<string> {
  signal.throwIfAborted();
  const url = fileThumbnailUrl(file.path);
  if (!url) throw new Error("File has no thumbnail URL");
  const existing = await fetch(url, {
    method: "HEAD",
    credentials: "same-origin",
    signal,
  });
  if (existing.ok) return url;
  // Do not turn authentication/server/network failures into expensive renders.
  if (existing.status !== 404)
    throw new Error("Could not check stored thumbnail");
  let bytes = cachedFileBytes(file.id)?.data;
  if (!bytes) {
    const response = await fetch(fileResourceUrl(file.path), {
      credentials: "same-origin",
      signal,
    });
    if (
      !response.ok ||
      Number(response.headers.get("Content-Length")) > 50 * 1024 * 1024
    )
      throw new Error("Could not load thumbnail source");
    bytes = await response.arrayBuffer();
  }
  signal.throwIfAborted();
  const image = await renderDocumentScreenshot(
    { data: bytes, filename: file.name, mimeType: file.type },
    {
      width: 360,
      height: 360,
    },
    signal,
  );
  await uploadFileThumbnail(file.id, image, signal);
  return url;
}

/** Reference-counted jobs survive duplicate cards, but stop when all leave view. */
export function acquireOfficeThumbnail(file: ThumbnailFile) {
  const key = `${getSessionId()}:${file.id}:${file.path}`;
  let entry = entries.get(key);
  if (
    entry?.controller.signal.aborted ||
    (entry?.settled && entry.retryAfter && Date.now() >= entry.retryAfter)
  ) {
    entries.delete(key);
    entry = undefined;
  }
  if (!entry) {
    // Bound retained URLs/errors; active jobs are never evicted.
    for (const [oldKey, oldEntry] of entries) {
      if (entries.size < 256) break;
      if (oldEntry.settled && !oldEntry.users) entries.delete(oldKey);
    }
    const controller = new AbortController();
    entry = {
      controller,
      promise: Promise.resolve(""),
      users: 0,
      settled: false,
      retryAfter: 0,
    };
    const current = entry;
    current.promise = thumbnailTail
      .catch(() => undefined)
      .then(() => ensureThumbnail(file, controller.signal))
      .catch((error: unknown) => {
        current.retryAfter = Date.now() + 5 * 60_000;
        if (controller.signal.aborted && entries.get(key) === current)
          entries.delete(key);
        throw error;
      })
      .finally(() => {
        current.settled = true;
      });
    thumbnailTail = current.promise.catch(() => undefined);
    entries.set(key, current);
  }
  entry.users++;
  const current = entry;
  let released = false;
  return {
    promise: current.promise,
    release: () => {
      if (released) return;
      released = true;
      current.users--;
      if (!current.users && !current.settled) current.controller.abort();
    },
  };
}
