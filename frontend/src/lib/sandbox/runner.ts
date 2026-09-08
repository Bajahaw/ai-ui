import { getHeaders } from "@/lib/api/headers";
import { fileResourceUrl, getFile } from "@/lib/api/files";
import type { Tool, ToolCall } from "@/lib/api/types";
import { filesObjectLiteral, wrapSandboxCode } from "./wrap";

const SANDBOX_TIMEOUT_MS = 90_000;
const RETRY_BUDGET_MS = 30_000;
const MAX_FILES = 3;
const MAX_FILE_BYTES = 15 * 1024 * 1024;
const APPROVAL_CACHE_TTL_MS = 30_000;

let sandboxApprovalCache: { value: boolean; ts: number } | null = null;

async function sandboxRequiresApproval(
  signal?: AbortSignal,
): Promise<boolean> {
  const now = Date.now();
  if (
    sandboxApprovalCache &&
    now - sandboxApprovalCache.ts < APPROVAL_CACHE_TTL_MS
  ) {
    return sandboxApprovalCache.value;
  }
  try {
    const res = await fetch("/api/tools/all", {
      method: "GET",
      credentials: "include",
      headers: getHeaders(),
      signal,
    });
    if (!res.ok) return false;
    const data = (await res.json()) as { tools?: Tool[] };
    const value =
      data.tools?.some(
        (t) => t.name === "browser_sandbox" && t.require_approval,
      ) ?? false;
    sandboxApprovalCache = { value, ts: now };
    return value;
  } catch {
    return false;
  }
}

/**
 * Streaming entry point: runs immediately, unless the sandbox is
 * approval-gated — then the ToolApproval UI triggers runBrowserSandbox
 * on click, after the backend has registered the pending call.
 */
export async function maybeAutoRunBrowserSandbox(
  toolCall: ToolCall,
  signal?: AbortSignal,
): Promise<void> {
  if (toolCall.name !== "browser_sandbox" || toolCall.tool_output) return;
  if (!toolCall.id || running.has(toolCall.id)) return;
  if (await sandboxRequiresApproval(signal)) return;
  await runBrowserSandbox(toolCall, signal);
}

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
    const inputs = await loadInputFiles(args.file_ids ?? [], signal);
    const result = await executeInIframe(code, inputs, signal);
    await postResultWithRetry(toolCall.id, result, signal);
  } catch (err) {
    if (signal?.aborted) return;
    await postResultWithRetry(
      toolCall.id,
      {
        ok: false,
        error: err instanceof Error ? err.message : String(err),
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
        ? parsed.file_ids.filter((id): id is string => typeof id === "string")
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
  for (const id of ids) {
    const meta = await getFile(id);
    const url = fileResourceUrl(meta.path);
    if (!url) continue;
    const res = await fetch(url, { credentials: "include", signal });
    if (!res.ok) continue;
    const buf = await res.arrayBuffer();
    out.push({ name: meta.name || id, data: arrayBufferToBase64(buf) });
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
        error,
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
        logs.push(data.message);
        return;
      }
      if (data.type === "__sandbox_file" && typeof data.name === "string") {
        const raw = toArrayBuffer(data.data);
        if (!raw || raw.byteLength > MAX_FILE_BYTES) return;
        outFiles.push({
          name: data.name,
          mime: typeof data.mime === "string" ? data.mime : "",
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

async function postResultWithRetry(
  callId: string,
  result: SandboxRunResult,
  signal?: AbortSignal,
): Promise<void> {
  const body = JSON.stringify({
    call_id: callId,
    ok: result.ok,
    logs: result.logs.slice(0, 200),
    error: result.error || "",
    files: result.files.map((f) => ({
      name: f.name,
      mime: f.mime,
      data: arrayBufferToBase64(f.data),
    })),
  });
  const deadline = Date.now() + RETRY_BUDGET_MS;
  let delay = 200;
  while (!signal?.aborted) {
    const res = await fetch("/api/tools/sandbox-result", {
      method: "POST",
      credentials: "include",
      headers: getHeaders({ "Content-Type": "application/json" }),
      body,
      signal,
    });
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
