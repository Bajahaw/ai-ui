// Wails loopback patch.
//
// Inside a Wails WebView (wails.localhost) the page is served by the Go asset
// server. On Android that bridge only supports GET requests without a body,
// so the app's POST/PUT/DELETE JSON APIs cannot go through it. Instead the Go
// side runs a loopback HTTP server (127.0.0.1:8080) in the same process and
// this module transparently redirects fetch() and EventSource calls for
// /api/* and /data/* there. Outside Wails (browser, docker) nothing changes.

const LOOPBACK_BASE = "http://127.0.0.1:8080";

function isWailsHost(): boolean {
  try {
    return window.location.hostname === "wails.localhost";
  } catch {
    return false;
  }
}

function needsLoopback(url: string): boolean {
  return url.startsWith("/api/") || url.startsWith("/data/");
}

function patchFetch(): void {
  if (!isWailsHost()) return;
  if ((window as unknown as Record<string, unknown>).__aiui_fetch_patched__) {
    return;
  }
  (window as unknown as Record<string, unknown>).__aiui_fetch_patched__ = true;
  const origFetch = window.fetch.bind(window);
  window.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
    if (typeof input === "string" && needsLoopback(input)) {
      return origFetch(LOOPBACK_BASE + input, init);
    }
    return origFetch(input as Request, init);
  }) as typeof window.fetch;
}

function patchEventSource(): void {
  if (!isWailsHost()) return;
  if ((window as unknown as Record<string, unknown>).__aiui_es_patched__) {
    return;
  }
  (window as unknown as Record<string, unknown>).__aiui_es_patched__ = true;
  const OrigES = window.EventSource;
  if (!OrigES) return;
  function PatchedEventSource(url: string | URL, cfg?: EventSourceInit) {
    let u = String(url);
    if (needsLoopback(u)) u = LOOPBACK_BASE + u;
    return new OrigES(u, cfg);
  }
  PatchedEventSource.prototype = OrigES.prototype;
  window.EventSource = PatchedEventSource as unknown as typeof EventSource;
}

patchFetch();
patchEventSource();

export {};
