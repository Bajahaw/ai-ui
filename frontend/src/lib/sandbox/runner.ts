import { getHeaders } from "@/lib/api/headers";
import {
  getFileBytes,
  resetFileBytesCache,
  retainFileBytes,
} from "@/lib/api/files";
import type { FrontendMessage, ToolCall } from "@/lib/api/types";
import { getSandboxFrameOrigin, resetSandboxOriginCache } from "./origin";
import { fileAliases, isSandboxHtml, wrapSandboxCode } from "./wrap";
import BOOTSTRAP_SRCDOC from "../../../public/sandbox.html?raw";

const SANDBOX_TIMEOUT_MS = 90_000;
const RETRY_BUDGET_MS = 30_000;
const MAX_FILES = 3;
const MAX_FILE_BYTES = 50 * 1024 * 1024;
const MAX_CODE_BYTES = 500 * 1024;
const MAX_INPUT_FILES = 10;
const MAX_TOTAL_INPUT_BYTES = 100 * 1024 * 1024;
const MAX_LOG_LINES = 200;
const MAX_LOG_LINE_CHARS = 2000;
const MAX_ERROR_CHARS = 4000;
const MAX_NAME_CHARS = 255;

type MountedInputFile = {
  name: string;
  id: string;
  data: ArrayBuffer;
};

type SandboxFile = {
  name: string;
  mime: string;
  data: ArrayBuffer;
};

type SandboxRunResult = {
  ok: boolean;
  error: string;
  logs: string[];
  files: SandboxFile[];
};

const running = new Set<string>();

let hostIframe: HTMLIFrameElement | null = null;
let hostBusy = false;

export function collectConversationFileIds(
  messages: Pick<FrontendMessage, "attachments" | "toolCalls">[],
): string[] {
  const ids: string[] = [];
  const seen = new Set<string>();
  const push = (id: string | undefined) => {
    const trimmed = (id || "").trim();
    if (!trimmed || seen.has(trimmed)) return;
    seen.add(trimmed);
    ids.push(trimmed);
  };
  for (const m of messages) {
    for (const a of m.attachments ?? []) push(a?.file?.id);
    for (const t of m.toolCalls ?? []) push(t?.file_id);
  }
  return ids.slice(0, MAX_INPUT_FILES);
}

export function resetSandboxFileCache(): void {
  resetFileBytesCache();
  resetSandboxOriginCache();
  hostBusy = false;
  if (hostIframe) {
    hostIframe.remove();
    hostIframe = null;
  }
}

export async function loadSandboxInputFiles(
  ids: string[],
  signal?: AbortSignal,
): Promise<MountedInputFile[]> {
  const out: MountedInputFile[] = [];
  const seen = new Set<string>();
  const requested = new Set<string>();
  let totalBytes = 0;
  for (const rawId of ids.slice(0, MAX_INPUT_FILES)) {
    const id = (rawId || "").trim();
    if (!id || seen.has(id)) continue;
    seen.add(id);
    requested.add(id);
    if (signal?.aborted) break;
    try {
      const file = await getFileBytes(id, signal);
      if (!file.data.byteLength || file.data.byteLength > MAX_FILE_BYTES) {
        continue;
      }
      if (totalBytes + file.data.byteLength > MAX_TOTAL_INPUT_BYTES) continue;
      totalBytes += file.data.byteLength;
      out.push({
        name: file.name.slice(0, MAX_NAME_CHARS),
        id: id.slice(0, MAX_NAME_CHARS),
        data: file.data,
      });
    } catch {
      continue;
    }
  }
  if (!signal?.aborted) retainFileBytes(requested);
  return out;
}

export async function runBrowserSandbox(
  toolCall: ToolCall,
  signal?: AbortSignal,
  conversationFileIds?: string[],
): Promise<void> {
  if (toolCall.name !== "browser_sandbox" || toolCall.tool_output) return;
  if (!toolCall.id || running.has(toolCall.id)) return;
  running.add(toolCall.id);
  try {
    const args = parseArgs(toolCall.args);
    const code = args.code || args.html || "";
    if (!code.trim()) {
      await postResultWithRetry(
        toolCall.id,
        { ok: false, error: "code is required", logs: [], files: [] },
        signal,
      );
      return;
    }
    if (new TextEncoder().encode(code).length > MAX_CODE_BYTES) {
      await postResultWithRetry(
        toolCall.id,
        { ok: false, error: "code exceeds 500KB limit", logs: [], files: [] },
        signal,
      );
      return;
    }
    const inputs = await loadSandboxInputFiles(
      conversationFileIds ?? args.file_ids ?? [],
      signal,
    );
    const result = await executeInIframe(code, inputs, signal);
    await postResultWithRetry(toolCall.id, result, signal);
  } catch (err) {
    if (signal?.aborted) return;
    await postResultWithRetry(
      toolCall.id,
      {
        ok: false,
        error: truncate(err instanceof Error ? err.message : String(err)),
        logs: [],
        files: [],
      },
      signal,
    ).catch(() => undefined);
  } finally {
    running.delete(toolCall.id);
  }
}

function parseArgs(raw?: string): {
  code?: string;
  html?: string;
  file_ids?: string[];
} {
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw) as {
      code?: unknown;
      html?: unknown;
      file_ids?: unknown;
    };
    return {
      code: typeof parsed.code === "string" ? parsed.code : undefined,
      html: typeof parsed.html === "string" ? parsed.html : undefined,
      file_ids: Array.isArray(parsed.file_ids)
        ? parsed.file_ids
            .filter((id): id is string => typeof id === "string")
            .slice(0, MAX_INPUT_FILES)
        : undefined,
    };
  } catch {
    return {};
  }
}

function createIframe(): HTMLIFrameElement {
  const iframe = document.createElement("iframe");
  iframe.setAttribute("title", "browser sandbox");
  iframe.referrerPolicy = "no-referrer";
  iframe.style.cssText =
    "position:fixed;left:-9999px;top:0;width:1px;height:1px;opacity:0;pointer-events:none;border:0";
  return iframe;
}

function mountFrame(iframe: HTMLIFrameElement, frameOrigin: string): void {
  if (frameOrigin) {
    iframe.setAttribute("sandbox", "allow-scripts allow-same-origin");
    iframe.src = `${frameOrigin}/sandbox.html?parent=${encodeURIComponent(window.location.origin)}`;
    return;
  }
  iframe.setAttribute("sandbox", "allow-scripts");
  iframe.srcdoc = BOOTSTRAP_SRCDOC;
}

function acquireIframe(): { iframe: HTMLIFrameElement; shared: boolean } {
  if (hostIframe && !hostBusy) {
    if (!hostIframe.isConnected) document.body.appendChild(hostIframe);
    hostBusy = true;
    return { iframe: hostIframe, shared: true };
  }
  const iframe = createIframe();
  document.body.appendChild(iframe);
  if (!hostIframe) {
    hostIframe = iframe;
    hostBusy = true;
    return { iframe, shared: true };
  }
  return { iframe, shared: false };
}

function releaseIframe(iframe: HTMLIFrameElement, shared: boolean): void {
  iframe.removeAttribute("srcdoc");
  iframe.src = "about:blank";
  if (shared) {
    hostBusy = false;
    return;
  }
  iframe.remove();
}

function executeInIframe(
  code: string,
  files: MountedInputFile[],
  signal?: AbortSignal,
): Promise<SandboxRunResult> {
  return new Promise((resolve) => {
    const logs: string[] = [];
    const outFiles: SandboxFile[] = [];
    let settled = false;
    let started = false;
    let iframe: HTMLIFrameElement | null = null;
    let shared = false;
    let timer = 0;

    const finish = (error = "") => {
      if (settled) return;
      settled = true;
      window.removeEventListener("message", onMessage);
      clearTimeout(timer);
      signal?.removeEventListener("abort", onAbort);
      if (iframe) {
        iframe.removeEventListener("load", onFrameLoad);
        releaseIframe(iframe, shared);
      }
      resolve({
        ok: !error,
        error: truncate(error),
        logs,
        files: outFiles.slice(0, MAX_FILES),
      });
    };

    const onAbort = () => finish("cancelled");
    const onFrameLoad = () => {
      if (!iframe?.dataset.sandboxOrigin) return;
      try {
        const win = iframe.contentWindow;
        if (!win) return;
        void win.document;
        if (win.location.origin === window.location.origin) {
          finish("sandbox origin mismatch");
        }
      } catch {
        // Cross-origin frame: expected when SANDBOX_ORIGIN is set.
      }
    };

    const startRun = (frameOrigin: string) => {
      if (settled || started || !iframe) return;
      const win = iframe.contentWindow;
      if (!win) {
        finish("sandbox failed to start");
        return;
      }
      started = true;
      const copies = files.map((f) => f.data.slice(0));
      const html = isSandboxHtml(code) ? wrapSandboxCode(code) : "";
      win.postMessage(
        {
          type: "__sandbox_init",
          html,
          code: html ? "" : code.trim(),
          files: files.map((f, i) => ({
            name: f.name,
            keys: fileAliases(f.name, f.id),
            data: copies[i],
          })),
        },
        frameOrigin || "*",
        copies,
      );
    };

    const onMessage = (event: MessageEvent) => {
      if (!iframe || event.source !== iframe.contentWindow) return;
      const data = event.data;
      if (!data || typeof data !== "object") return;
      const frameOrigin = iframe.dataset.sandboxOrigin || "";
      if (frameOrigin && event.origin !== frameOrigin) return;
      if (data.type === "__sandbox_ready") {
        startRun(frameOrigin);
        return;
      }
      if (data.type === "__sandbox_log" && typeof data.message === "string") {
        if (logs.length < MAX_LOG_LINES) {
          logs.push(data.message.slice(0, MAX_LOG_LINE_CHARS));
        }
        return;
      }
      if (data.type === "__sandbox_file" && typeof data.name === "string") {
        if (outFiles.length >= MAX_FILES) return;
        const raw = toArrayBuffer(data.data);
        if (!raw || !raw.byteLength || raw.byteLength > MAX_FILE_BYTES) return;
        outFiles.push({
          name: String(data.name).slice(0, MAX_NAME_CHARS),
          mime:
            typeof data.mime === "string"
              ? data.mime.slice(0, MAX_NAME_CHARS)
              : "",
          data: raw,
        });
        return;
      }
      if (data.type === "__sandbox_done") {
        finish(typeof data.error === "string" ? data.error : "");
      }
    };

    window.addEventListener("message", onMessage);
    signal?.addEventListener("abort", onAbort);
    timer = window.setTimeout(
      () => finish("sandbox timed out"),
      SANDBOX_TIMEOUT_MS,
    );
    if (signal?.aborted) {
      finish("cancelled");
      return;
    }

    void getSandboxFrameOrigin().then((frameOrigin) => {
      if (settled) return;
      const acquired = acquireIframe();
      iframe = acquired.iframe;
      shared = acquired.shared;
      if (frameOrigin) iframe.dataset.sandboxOrigin = frameOrigin;
      else delete iframe.dataset.sandboxOrigin;
      iframe.addEventListener("load", onFrameLoad);
      mountFrame(iframe, frameOrigin);
    });
  });
}

function truncate(s: string, max = MAX_ERROR_CHARS): string {
  return s.length > max ? s.slice(0, max) : s;
}

async function postResultWithRetry(
  callId: string,
  result: SandboxRunResult,
  signal?: AbortSignal,
): Promise<void> {
  const body = JSON.stringify({
    call_id: callId,
    ok: result.ok,
    logs: result.logs.slice(0, MAX_LOG_LINES),
    error: truncate(result.error),
    files: result.files.slice(0, MAX_FILES).map((f) => ({
      name: f.name.slice(0, MAX_NAME_CHARS),
      mime: f.mime.slice(0, MAX_NAME_CHARS),
      data: arrayBufferToBase64(f.data),
    })),
  });
  const deadline = Date.now() + RETRY_BUDGET_MS;
  let delay = 200;
  while (!signal?.aborted) {
    let res: Response;
    try {
      res = await fetch("/api/tools/sandbox-result", {
        method: "POST",
        credentials: "include",
        headers: getHeaders({ "Content-Type": "application/json" }),
        body,
        signal,
      });
    } catch (err) {
      if (signal?.aborted) return;
      if (Date.now() >= deadline) throw err;
      await sleep(delay, signal);
      delay = Math.min(delay * 2, 2000);
      continue;
    }
    if (res.ok) return;
    if (res.status !== 404 || Date.now() >= deadline) {
      throw new Error((await res.text()) || res.statusText);
    }
    await sleep(delay, signal);
    delay = Math.min(delay * 2, 2000);
  }
}

function toArrayBuffer(value: unknown): ArrayBuffer | null {
  if (value instanceof ArrayBuffer) return value;
  if (ArrayBuffer.isView(value)) {
    return value.buffer.slice(
      value.byteOffset,
      value.byteOffset + value.byteLength,
    ) as ArrayBuffer;
  }
  return null;
}

function arrayBufferToBase64(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let binary = "";
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException("Aborted", "AbortError"));
      return;
    }
    const id = window.setTimeout(resolve, ms);
    signal?.addEventListener(
      "abort",
      () => {
        clearTimeout(id);
        reject(new DOMException("Aborted", "AbortError"));
      },
      { once: true },
    );
  });
}
