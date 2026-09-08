import { getHeaders } from "@/lib/api/headers";
import { fileResourceUrl, getFile } from "@/lib/api/files";
import type { ToolCall } from "@/lib/api/types";
import { filesObjectLiteral, wrapSandboxCode } from "./wrap";

const SANDBOX_TIMEOUT_MS = 90_000;
const RETRY_BUDGET_MS = 30_000;
const MAX_FILES = 3;
const MAX_FILE_BYTES = 15 * 1024 * 1024;
const MAX_CODE_BYTES = 500 * 1024;
const MAX_INPUT_FILES = 3;
const MAX_LOG_LINES = 200;
const MAX_LOG_LINE_CHARS = 2000;
const MAX_ERROR_CHARS = 4000;
const MAX_NAME_CHARS = 255;

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

/** Guards against duplicate runs from streaming updates + approval clicks. */
const running = new Set<string>();

export async function runBrowserSandbox(
  toolCall: ToolCall,
  signal?: AbortSignal,
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
    const inputs = await loadInputFiles(args.file_ids ?? [], signal);
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

async function loadInputFiles(
  ids: string[],
  signal?: AbortSignal,
): Promise<{ name: string; data: string }[]> {
  const out: { name: string; data: string }[] = [];
  for (const id of ids.slice(0, MAX_INPUT_FILES)) {
    if (signal?.aborted) break;
    try {
      const meta = await getFile(id);
      const url = fileResourceUrl(meta.path);
      if (!url) continue;
      const res = await fetch(url, { credentials: "include", signal });
      if (!res.ok) continue;
      const buf = await res.arrayBuffer();
      if (!buf.byteLength || buf.byteLength > MAX_FILE_BYTES) continue;
      out.push({
        name: (meta.name || id).slice(0, MAX_NAME_CHARS),
        data: arrayBufferToBase64(buf),
      });
    } catch {
      // Skip unreadable inputs so one bad id doesn't fail the whole run.
      continue;
    }
  }
  return out;
}

function executeInIframe(
  code: string,
  files: { name: string; data: string }[],
  signal?: AbortSignal,
): Promise<SandboxRunResult> {
  return new Promise((resolve) => {
    const iframe = document.createElement("iframe");
    iframe.setAttribute("sandbox", "allow-scripts");
    iframe.setAttribute("title", "browser sandbox");
    iframe.style.cssText =
      "position:fixed;left:-9999px;top:0;width:1px;height:1px;opacity:0;pointer-events:none;border:0";

    const logs: string[] = [];
    const outFiles: SandboxFile[] = [];
    let settled = false;

    const finish = (error = "") => {
      if (settled) return;
      settled = true;
      window.removeEventListener("message", onMessage);
      clearTimeout(timer);
      signal?.removeEventListener("abort", onAbort);
      iframe.remove();
      resolve({
        ok: !error,
        error: truncate(error),
        logs,
        files: outFiles.slice(0, MAX_FILES),
      });
    };

    const onAbort = () => finish("cancelled");
    const onMessage = (event: MessageEvent) => {
      if (event.source !== iframe.contentWindow) return;
      const data = event.data;
      if (!data || typeof data !== "object") return;
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
    const timer = window.setTimeout(
      () => finish("sandbox timed out"),
      SANDBOX_TIMEOUT_MS,
    );
    if (signal?.aborted) {
      finish("cancelled");
      return;
    }
    iframe.srcdoc = buildSrcdoc(code, files);
    document.body.appendChild(iframe);
  });
}

function buildSrcdoc(
  code: string,
  files: { name: string; data: string }[],
): string {
  return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="script-src 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net https://cdnjs.cloudflare.com blob:; object-src 'none'; base-uri 'none';">
</head>
<body>
<script>
(function(){
  var __files = ${filesObjectLiteral(files)};
  function send(msg){ window.parent.postMessage(msg,'*'); }
  function toBuf(data){
    if (data instanceof ArrayBuffer) return data;
    if (ArrayBuffer.isView(data)) return data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength);
    if (typeof data === 'string') return new TextEncoder().encode(data).buffer;
    throw new Error('writeFile: unsupported data type');
  }
  window.sandbox = {
    _done: false,
    readFile: function(name){
      if (!__files[name]) throw new Error('file not found: ' + name);
      return __files[name];
    },
    writeFile: function(name, data, mime){
      var p = (typeof Blob !== 'undefined' && data instanceof Blob)
        ? data.arrayBuffer()
        : Promise.resolve(toBuf(data));
      return p.then(function(buf){
        send({ type: '__sandbox_file', name: name, mime: mime || '', data: buf });
      });
    },
    done: function(err){
      if (window.sandbox._done) return;
      window.sandbox._done = true;
      send({ type: '__sandbox_done', error: err ? String(err) : '' });
    }
  };
  function hook(fn, level){
    return function(){
      var msg = Array.prototype.map.call(arguments, function(a){
        try { return typeof a === 'string' ? a : JSON.stringify(a); }
        catch (e) { return String(a); }
      }).join(' ');
      send({ type: '__sandbox_log', level: level, message: msg });
      return fn.apply(console, arguments);
    };
  }
  console.log = hook(console.log, 'log');
  console.info = hook(console.info, 'info');
  console.warn = hook(console.warn, 'warn');
  console.error = hook(console.error, 'error');
  window.addEventListener('error', function(e){
    send({ type: '__sandbox_log', level: 'error', message: e.message || 'Runtime error' });
  });
  window.addEventListener('unhandledrejection', function(e){
    send({ type: '__sandbox_log', level: 'error', message: String(e.reason) });
  });
})();
</script>
${wrapSandboxCode(code)}
</body>
</html>`;
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
  // Retry while the backend hasn't registered the pending call yet (404)
  // or the network flaked; any other status is fatal.
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
