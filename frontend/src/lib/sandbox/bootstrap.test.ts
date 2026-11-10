import { afterEach, describe, expect, it } from "vitest";
import { BOOTSTRAP_SRCDOC, SANDBOX_SCRIPT_HOSTS } from "./runner";

type SandboxApi = {
  listFiles(): string[];
  readFile(name: string): Uint8Array;
  writeFile(name: string, data: unknown, mime?: string): Promise<void>;
  loadScript(url: string): Promise<void>;
  screenshot(source: string | Uint8Array, options?: {filename?: string; page?: number}): Promise<Blob>;
  done(err?: unknown): void;
  _done: boolean;
};

type Sent = { type: string; [k: string]: unknown };

type Booted = {
  sandbox: SandboxApi;
  sent: Sent[];
  frame: Window;
  doc: Document;
  init(code: string, files?: { name: string; keys?: string[]; data: ArrayBuffer }[]): void;
};

let iframe: HTMLIFrameElement | null = null;
let onMessage: ((e: MessageEvent) => void) | null = null;

// Runs the bootstrap script inside a real (jsdom) child iframe so
// window.parent is the test window, exactly like production. Messages the
// bootstrap posts to its parent are collected in `sent`.
function bootBootstrap(): Booted {
  const script = new DOMParser()
    .parseFromString(BOOTSTRAP_SRCDOC, "text/html")
    .querySelector("script")?.textContent;
  if (!script) throw new Error("bootstrap script not found");
  iframe = document.createElement("iframe");
  document.body.appendChild(iframe);
  const frame = iframe.contentWindow;
  if (!frame) throw new Error("no contentWindow");
  const sent: Sent[] = [];
  onMessage = (e: MessageEvent) => {
    if (e.data && typeof e.data.type === "string" && e.data.type.startsWith("__sandbox_")) {
      sent.push(e.data as Sent);
    }
  };
  window.addEventListener("message", onMessage);
  (frame as unknown as { eval(s: string): void }).eval(script);
  const FrameMessageEvent = (frame as unknown as { MessageEvent: typeof MessageEvent }).MessageEvent;
  return {
    sandbox: (frame as unknown as { sandbox: SandboxApi }).sandbox,
    sent,
    frame,
    doc: frame.document,
    // jsdom's postMessage leaves e.source null, which the bootstrap
    // (correctly) rejects as not from the parent; dispatch with an explicit
    // source. Use frame.parent: under vitest the global `window` is Node's
    // globalThis, not the jsdom window that is the frame's actual parent.
    init(code, files = []) {
      frame.dispatchEvent(
        new FrameMessageEvent("message", {
          data: { type: "__sandbox_init", code, files },
          source: frame.parent,
        }),
      );
    },
  };
}

async function waitFor(pred: () => boolean, ms = 2000): Promise<void> {
  const deadline = Date.now() + ms;
  while (!pred()) {
    if (Date.now() > deadline) throw new Error("timed out waiting");
    await new Promise((r) => setTimeout(r, 5));
  }
}

describe("sandbox bootstrap", () => {
  afterEach(() => {
    if (onMessage) window.removeEventListener("message", onMessage);
    iframe?.remove();
    iframe = null;
    onMessage = null;
  });

  it("rejects loadScript for hosts outside the CSP allowlist without touching the DOM", async () => {
    const { sandbox, doc } = bootBootstrap();
    await expect(sandbox.loadScript("https://unpkg.com/x.js")).rejects.toThrow(/blocked by the sandbox CSP/);
    await expect(sandbox.loadScript("https://unpkg.com/x.js")).rejects.toThrow(SANDBOX_SCRIPT_HOSTS[0]);
    await expect(sandbox.loadScript("http://cdn.jsdelivr.net/x.js")).rejects.toThrow(/blocked/);
    await expect(sandbox.loadScript("not a url")).rejects.toThrow(/invalid URL/);
    expect(doc.head.querySelectorAll("script")).toHaveLength(0);
  });

  it("rejects loadScript fast when the script element errors (bad URL/path)", async () => {
    const { sandbox, doc, frame } = bootBootstrap();
    const p = sandbox.loadScript("https://cdn.jsdelivr.net/npm/does-not-exist@0.0.0/x.js");
    const el = doc.head.querySelector("script");
    expect(el?.src).toBe("https://cdn.jsdelivr.net/npm/does-not-exist@0.0.0/x.js");
    el?.dispatchEvent(new (frame as unknown as { Event: typeof Event }).Event("error"));
    await expect(p).rejects.toThrow(/failed to load .*does-not-exist/);
  });

  it("resolves loadScript on load and dedupes the same URL", async () => {
    const { sandbox, doc, frame } = bootBootstrap();
    const url = "https://cdnjs.cloudflare.com/ajax/libs/x/1.0.0/x.min.js";
    const p1 = sandbox.loadScript(url);
    const p2 = sandbox.loadScript(url);
    expect(doc.head.querySelectorAll("script")).toHaveLength(1);
    doc.head.querySelector("script")?.dispatchEvent(new (frame as unknown as { Event: typeof Event }).Event("load"));
    await expect(p1).resolves.toBeUndefined();
    await expect(p2).resolves.toBeUndefined();
  });

  it("mounts files, reports the return value, and auto-calls done", async () => {
    const { sent, init } = bootBootstrap();
    await waitFor(() => sent.some((m) => m.type === "__sandbox_ready"));
    init("return sandbox.listFiles()", [
      { name: "report.xlsx", keys: ["report.xlsx", "id-1"], data: new Uint8Array([1, 2, 3]).buffer },
    ]);
    await waitFor(() => sent.some((m) => m.type === "__sandbox_done"));
    const result = sent.find((m) => m.type === "__sandbox_result");
    expect(result?.value).toBe('["report.xlsx"]');
    const done = sent.find((m) => m.type === "__sandbox_done");
    expect(done?.error).toBe("");
  });

  it("reports thrown errors through done and does not send a result", async () => {
    const { sent, init } = bootBootstrap();
    await waitFor(() => sent.some((m) => m.type === "__sandbox_ready"));
    init("sandbox.readFile('missing.bin')");
    await waitFor(() => sent.some((m) => m.type === "__sandbox_done"));
    const done = sent.find((m) => m.type === "__sandbox_done");
    expect(String(done?.error)).toMatch(/file not found: missing\.bin/);
    expect(sent.find((m) => m.type === "__sandbox_result")).toBeUndefined();
  });

  it("describes binary return values instead of dumping bytes", async () => {
    const { sent, init } = bootBootstrap();
    await waitFor(() => sent.some((m) => m.type === "__sandbox_ready"));
    init("return sandbox.readFile('a.bin')", [
      { name: "a.bin", keys: ["a.bin"], data: new Uint8Array(5).buffer },
    ]);
    await waitFor(() => sent.some((m) => m.type === "__sandbox_done"));
    const result = sent.find((m) => m.type === "__sandbox_result");
    expect(result?.value).toBe("[Uint8Array 5 bytes]");
  });

  it("requests screenshots for mounted aliases without detaching input bytes", async () => {
    const { sent, init, frame, sandbox } = bootBootstrap();
    const FrameMessageEvent = (frame as unknown as {MessageEvent: typeof MessageEvent}).MessageEvent;
    init("const image = await sandbox.screenshot('file-id', {page: 2}); return image.type", [{name: "report.docx", keys: ["report.docx", "file-id"], data: new Uint8Array([1, 2, 3]).buffer}]);
    await waitFor(() => sent.some((m) => m.type === "__sandbox_screenshot"));
    const message = sent.find((m) => m.type === "__sandbox_screenshot")!;
    expect(message.filename).toBe("report.docx");
    expect(message.options).toMatchObject({page: 2});
    expect(sandbox.readFile("report.docx").byteLength).toBe(3);
    const FrameArrayBuffer = (frame as unknown as {ArrayBuffer: typeof ArrayBuffer}).ArrayBuffer;
    frame.dispatchEvent(new FrameMessageEvent("message", {source: frame.parent, data: {type: "__sandbox_screenshot_result", requestId: message.requestId, data: new FrameArrayBuffer(4)}}));
    await waitFor(() => sent.some((m) => m.type === "__sandbox_done"));
    expect(sent.find((m) => m.type === "__sandbox_result")?.value).toBe("image/png");
  });

  it("reports screenshot failures to calling sandbox code", async () => {
    const { sent, init, frame } = bootBootstrap();
    const FrameMessageEvent = (frame as unknown as {MessageEvent: typeof MessageEvent}).MessageEvent;
    init("await sandbox.screenshot('report.docx')", [{name: "report.docx", data: new Uint8Array([1, 2, 3]).buffer}]);
    await waitFor(() => sent.some((m) => m.type === "__sandbox_screenshot"));
    const message = sent.find((m) => m.type === "__sandbox_screenshot")!;
    frame.dispatchEvent(new FrameMessageEvent("message", {source: frame.parent, data: {type: "__sandbox_screenshot_result", requestId: message.requestId, error: "Page number is out of range"}}));
    await waitFor(() => sent.some((m) => m.type === "__sandbox_done"));
    expect(sent.find((m) => m.type === "__sandbox_done")?.error).toBe("Page number is out of range");
  });

  it("assigns distinct request IDs to concurrent screenshots", async () => {
    const { sent, init, frame } = bootBootstrap();
    const FrameMessageEvent = (frame as unknown as {MessageEvent: typeof MessageEvent}).MessageEvent;
    init("await Promise.all([sandbox.screenshot('report.docx'), sandbox.screenshot('report.docx')])", [{name: "report.docx", data: new Uint8Array([1, 2, 3]).buffer}]);
    await waitFor(() => sent.filter((m) => m.type === "__sandbox_screenshot").length === 2);
    const requests = sent.filter((m) => m.type === "__sandbox_screenshot");
    expect(requests.map((m) => m.requestId)).toEqual([1, 2]);
    for (const message of requests) frame.dispatchEvent(new FrameMessageEvent("message", {source: frame.parent, data: {type: "__sandbox_screenshot_result", requestId: message.requestId, error: "test complete"}}));
    await waitFor(() => sent.some((m) => m.type === "__sandbox_done"));
  });
});
