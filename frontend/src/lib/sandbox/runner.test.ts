import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { getFileBytes, retainFileBytes } from "@/lib/api/files";
import {
  BOOTSTRAP_SRCDOC,
  loadSandboxInputFiles,
  resetSandboxFileCache,
  runBrowserSandbox,
} from "./runner";

vi.mock("@/lib/api/files", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/files")>();
  const bytes = new Map<string, ArrayBuffer>();
  return {
    ...actual,
    getFileBytes: vi.fn(async (id: string) => {
      let data = bytes.get(id);
      if (!data) {
        const raw = new Uint8Array(id.length);
        for (let i = 0; i < id.length; i++) raw[i] = id.charCodeAt(i);
        data = raw.buffer;
        bytes.set(id, data);
      }
      return { name: `${id}.txt`, data };
    }),
    retainFileBytes: vi.fn((ids: Iterable<string>) => {
      const keep = new Set(ids);
      for (const key of [...bytes.keys()]) {
        if (!keep.has(key)) bytes.delete(key);
      }
    }),
    resetFileBytesCache: vi.fn(() => bytes.clear()),
  };
});

const getFileBytesMock = vi.mocked(getFileBytes);
const retainFileBytesMock = vi.mocked(retainFileBytes);

describe("loadSandboxInputFiles", () => {
  beforeEach(() => {
    resetSandboxFileCache();
    getFileBytesMock.mockClear();
    retainFileBytesMock.mockClear();
  });

  afterEach(() => {
    resetSandboxFileCache();
  });

  it("loads conversation files from the files cache", async () => {
    const first = await loadSandboxInputFiles(["a"]);
    const second = await loadSandboxInputFiles(["a"]);
    expect(first).toHaveLength(1);
    expect(first[0]?.data.byteLength).toBeGreaterThan(0);
    expect(second[0]?.data).toBe(first[0]?.data);
    expect(getFileBytesMock).toHaveBeenCalledTimes(2);
    expect(retainFileBytesMock).toHaveBeenCalled();
  });

  it("remounts the full conversation set including prior outputs", async () => {
    const first = await loadSandboxInputFiles(["orig", "out1"]);
    const second = await loadSandboxInputFiles(["orig", "out1"]);
    expect(first.map((f) => f.id)).toEqual(["orig", "out1"]);
    expect(second.map((f) => f.id)).toEqual(["orig", "out1"]);
    expect(getFileBytesMock).toHaveBeenCalledTimes(4);
  });

  it("prunes files that left the conversation set", async () => {
    await loadSandboxInputFiles(["a"]);
    await loadSandboxInputFiles(["b"]);
    expect(retainFileBytesMock).toHaveBeenLastCalledWith(new Set(["b"]));
  });
});

describe("runBrowserSandbox iframe lifecycle", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(null, { status: 200 })),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    document.querySelectorAll("iframe").forEach((f) => f.remove());
  });

  // Re-navigating a live iframe adds joint session history entries, which
  // makes history.back() (New Chat, Android back) step through the iframe.
  async function runOnce(id: string) {
    const srcdocAtInsert: (string | null)[] = [];
    const observer = new MutationObserver((records) => {
      for (const r of records) {
        r.addedNodes.forEach((n) => {
          if (n instanceof HTMLIFrameElement) {
            srcdocAtInsert.push(n.getAttribute("srcdoc"));
          }
        });
      }
    });
    observer.observe(document.body, { childList: true });

    const done = runBrowserSandbox({
      id,
      name: "browser_sandbox",
      args: JSON.stringify({ code: "1" }),
    } as never);

    await vi.waitFor(() => {
      if (!document.querySelector("iframe")) throw new Error("no iframe yet");
    });
    const iframe = document.querySelector("iframe")!;
    const srcdocSetter = vi.spyOn(iframe, "srcdoc", "set");
    window.dispatchEvent(
      new MessageEvent("message", {
        data: { type: "__sandbox_done" },
        source: iframe.contentWindow,
      }),
    );
    await done;
    observer.disconnect();
    return { iframe, srcdocAtInsert, srcdocSetter };
  }

  it("loads each run in a fresh iframe and removes it without re-navigating", async () => {
    const first = await runOnce("call-1");
    const second = await runOnce("call-2");

    for (const run of [first, second]) {
      expect(run.srcdocAtInsert).toEqual([BOOTSTRAP_SRCDOC]);
      expect(run.srcdocSetter).not.toHaveBeenCalled();
      expect(run.iframe.isConnected).toBe(false);
    }
    expect(second.iframe).not.toBe(first.iframe);
    expect(document.querySelectorAll("iframe")).toHaveLength(0);
  });
});
