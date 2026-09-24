import { getHeaders } from "@/lib/api/headers";
import {
  getFileBytes,
  resetFileBytesCache,
  retainFileBytes,
} from "@/lib/api/files";
import type { FrontendMessage, ToolCall } from "@/lib/api/types";
import { fileAliases } from "./wrap";

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
const MAX_RESULT_CHARS = 16_000;
const MAX_NAME_CHARS = 255;
// Must match the iframe CSP script-src below.
export const SANDBOX_SCRIPT_HOSTS = [
  "cdn.jsdelivr.net",
  "cdnjs.cloudflare.com",
] as const;

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
  result: string;
  files: SandboxFile[];
};

const running = new Set<string>();

let hostIframe: HTMLIFrameElement | null = null;
let hostBusy = false;

// Exported for tests only: the bootstrap runs inside the sandbox iframe.
export const BOOTSTRAP_SRCDOC = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="script-src 'unsafe-inline' 'unsafe-eval' ${SANDBOX_SCRIPT_HOSTS.map((h) => "https://" + h).join(" ")} blob:; object-src 'none'; base-uri 'none';">
</head>
<body>
<script>
(function(){
  var ALLOWED_HOSTS = ${JSON.stringify(SANDBOX_SCRIPT_HOSTS)};
  var LOAD_SCRIPT_TIMEOUT_MS = 30000;
  var MAX_RESULT_CHARS = ${MAX_RESULT_CHARS};
  var __files = {};
  var __fileList = [];
  var __loaded = {};
  var __started = false;
  function send(msg, transfer){ window.parent.postMessage(msg, '*', transfer || []); }
  function toBuf(data){
    if (data instanceof ArrayBuffer) return data;
    if (ArrayBuffer.isView(data)) return data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength);
    if (typeof data === 'string') return new TextEncoder().encode(data).buffer;
    throw new Error('writeFile: unsupported data type');
  }
  function describe(v){
    if (v === undefined) return '';
    if (typeof v === 'string') return v;
    if (v instanceof ArrayBuffer) return '[ArrayBuffer ' + v.byteLength + ' bytes]';
    if (ArrayBuffer.isView(v)) return '[' + (v.constructor && v.constructor.name || 'TypedArray') + ' ' + v.byteLength + ' bytes]';
    var s;
    try { s = JSON.stringify(v); } catch (e) { s = undefined; }
    if (s === undefined) s = String(v);
    return s.length > MAX_RESULT_CHARS ? s.slice(0, MAX_RESULT_CHARS) + '…(result truncated)' : s;
  }
  window.sandbox = {
    _done: false,
    listFiles: function(){
      return __fileList.slice();
    },
    loadScript: function(url){
      var u;
      try { u = new URL(String(url)); } catch (e) { return Promise.reject(new Error('loadScript: invalid URL: ' + url)); }
      if (u.protocol !== 'https:' || ALLOWED_HOSTS.indexOf(u.hostname) < 0) {
        return Promise.reject(new Error('loadScript: host "' + u.hostname + '" is blocked by the sandbox CSP. Allowed hosts: ' + ALLOWED_HOSTS.join(', ')));
      }
      if (__loaded[u.href]) return __loaded[u.href];
      __loaded[u.href] = new Promise(function(resolve, reject){
        var s = document.createElement('script');
        var timer = setTimeout(function(){
          delete __loaded[u.href];
          reject(new Error('loadScript: timed out after ' + (LOAD_SCRIPT_TIMEOUT_MS / 1000) + 's loading ' + u.href));
        }, LOAD_SCRIPT_TIMEOUT_MS);
        s.onload = function(){ clearTimeout(timer); resolve(); };
        s.onerror = function(){
          clearTimeout(timer);
          delete __loaded[u.href];
          reject(new Error('loadScript: failed to load ' + u.href + ' (wrong URL, version, or path?)'));
        };
        s.src = u.href;
        document.head.appendChild(s);
      });
      return __loaded[u.href];
    },
    readFile: function(name){
      var key = String(name == null ? '' : name);
      if (__files[key]) return __files[key];
      var avail = __fileList.join(', ');
      throw new Error('file not found: ' + key + (avail ? ' (available files: ' + avail.slice(0, 1000) + '; use sandbox.listFiles())' : ' (no files available; use sandbox.listFiles())'));
    },
    writeFile: function(name, data, mime){
      var p = (typeof Blob !== 'undefined' && data instanceof Blob)
        ? data.arrayBuffer()
        : Promise.resolve(toBuf(data));
      return p.then(function(buf){
        send({ type: '__sandbox_file', name: name, mime: mime || '', data: buf }, [buf]);
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
  document.addEventListener('securitypolicyviolation', function(e){
    send({ type: '__sandbox_log', level: 'error', message: 'CSP blocked ' + (e.blockedURI || 'a resource') + ' (' + e.violatedDirective + '). Allowed script hosts: ' + ALLOWED_HOSTS.join(', ') });
  });
  function mountFiles(files){
    __files = {};
    __fileList = [];
    for (var i = 0; i < files.length; i++) {
      var f = files[i];
      var u8 = new Uint8Array(f.data);
      __fileList.push(f.name);
      var keys = f.keys && f.keys.length ? f.keys : [f.name];
      for (var k = 0; k < keys.length; k++) __files[keys[k]] = u8;
    }
  }
  function runJs(code){
    (async function(){
      var value;
      try {
        var run = new Function('return (async()=>{\\n' + code + '\\n})()');
        value = await run();
        await Promise.resolve();
      } catch (e) {
        sandbox.done(String(e && e.message ? e.message : e));
        return;
      }
      var text = describe(value);
      if (text) send({ type: '__sandbox_result', value: text });
      if (!sandbox._done) sandbox.done();
    })();
  }
  window.addEventListener('message', function(e){
    if (e.source !== window.parent) return;
    var data = e.data;
    if (!data || data.type !== '__sandbox_init' || __started) return;
    __started = true;
    mountFiles(data.files || []);
    runJs(data.code || '');
  });
  send({ type: '__sandbox_ready' });
})();
</script>
</body>
</html>`;

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
    const code = args.code || "";
    if (!code.trim()) {
      await postResultWithRetry(
        toolCall.id,
        { ok: false, error: "code is required", logs: [], result: "", files: [] },
        signal,
      );
      return;
    }
    if (new TextEncoder().encode(code).length > MAX_CODE_BYTES) {
      await postResultWithRetry(
        toolCall.id,
        { ok: false, error: "code exceeds 500KB limit", logs: [], result: "", files: [] },
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
        result: "",
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
  file_ids?: string[];
} {
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw) as {
      code?: unknown;
      file_ids?: unknown;
    };
    return {
      code: typeof parsed.code === "string" ? parsed.code : undefined,
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
  iframe.setAttribute("sandbox", "allow-scripts");
  iframe.setAttribute("title", "browser sandbox");
  iframe.style.cssText =
    "position:fixed;left:-9999px;top:0;width:1px;height:1px;opacity:0;pointer-events:none;border:0";
  return iframe;
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
  iframe.srcdoc = "";
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
    const { iframe, shared } = acquireIframe();
    const logs: string[] = [];
    const outFiles: SandboxFile[] = [];
    let result = "";
    let settled = false;
    let started = false;

    const finish = (error = "") => {
      if (settled) return;
      settled = true;
      window.removeEventListener("message", onMessage);
      clearTimeout(timer);
      signal?.removeEventListener("abort", onAbort);
      releaseIframe(iframe, shared);
      resolve({
        ok: !error,
        error: truncate(error),
        logs,
        result: truncate(result, MAX_RESULT_CHARS),
        files: outFiles.slice(0, MAX_FILES),
      });
    };

    const startRun = () => {
      if (settled || started) return;
      const win = iframe.contentWindow;
      if (!win) {
        finish("sandbox failed to start");
        return;
      }
      started = true;
      const copies = files.map((f) => f.data.slice(0));
      win.postMessage(
        {
          type: "__sandbox_init",
          code: code.trim(),
          files: files.map((f, i) => ({
            name: f.name,
            keys: fileAliases(f.name, f.id),
            data: copies[i],
          })),
        },
        "*",
        copies,
      );
    };

    const onAbort = () => finish("cancelled");
    const onMessage = (event: MessageEvent) => {
      if (event.source !== iframe.contentWindow) return;
      const data = event.data;
      if (!data || typeof data !== "object") return;
      if (data.type === "__sandbox_ready") {
        startRun();
        return;
      }
      if (data.type === "__sandbox_log" && typeof data.message === "string") {
        if (logs.length < MAX_LOG_LINES) {
          logs.push(data.message.slice(0, MAX_LOG_LINE_CHARS));
        }
        return;
      }
      if (data.type === "__sandbox_result" && typeof data.value === "string") {
        result = data.value;
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
    iframe.srcdoc = BOOTSTRAP_SRCDOC;
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
    result: truncate(result.result, MAX_RESULT_CHARS),
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
