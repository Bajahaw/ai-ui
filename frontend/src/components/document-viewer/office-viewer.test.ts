import { act, createElement, StrictMode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { OfficePreviewKind } from "@/lib/document-preview";
import OfficeViewer from "./office-viewer";

const { construct } = vi.hoisted(() => ({ construct: vi.fn() }));
vi.mock("./office-surface", () => ({
  createOfficeSurface: (...args: unknown[]) =>
    Promise.resolve(construct(...args)),
}));

type Surface = {
  host: HTMLElement;
  options: {
    onError: (error: unknown) => void;
    onScaleChange: (scale: number) => void;
  };
  load: ReturnType<typeof vi.fn>;
  destroy: ReturnType<typeof vi.fn>;
  zoomIn: ReturnType<typeof vi.fn>;
  zoomOut: ReturnType<typeof vi.fn>;
  fitWidth: ReturnType<typeof vi.fn>;
  resolve: () => void;
  reject: (error: unknown) => void;
};

let root: Root;
let container: HTMLDivElement;
let surfaces: Surface[];
let fetchFile: ReturnType<typeof vi.fn>;
const bytes = new ArrayBuffer(8);

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  surfaces = [];
  construct
    .mockReset()
    .mockImplementation((host, _, onError, onScaleChange) => {
      let resolve!: Surface["resolve"];
      let reject!: Surface["reject"];
      const promise = new Promise<void>((res, rej) => {
        resolve = res;
        reject = rej;
      });
      const surface: Surface = {
        host,
        options: { onError, onScaleChange },
        resolve,
        reject,
        load: vi.fn(() => promise),
        destroy: vi.fn(),
        zoomIn: vi.fn(),
        zoomOut: vi.fn(),
        fitWidth: vi.fn(),
      };
      host.append(document.createElement("canvas"));
      surfaces.push(surface);
      return surface;
    });
  fetchFile = vi.fn(async () => ({
    ok: true,
    arrayBuffer: async () => bytes,
  }));
  vi.stubGlobal("fetch", fetchFile);
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});

async function render(
  kind: OfficePreviewKind = "docx",
  strict = false,
  href = `/data/resources/test.${kind}`,
) {
  const viewer = createElement(OfficeViewer, {
    document: { href, filename: "Test", kind },
  });
  await act(async () => {
    root.render(strict ? createElement(StrictMode, null, viewer) : viewer);
  });
}

async function click(selector: string) {
  await act(async () =>
    container.querySelector<HTMLButtonElement>(selector)!.click(),
  );
}

describe("Office renderer lifecycle", () => {
  it.each(["docx", "xlsx", "pptx"] as const)(
    "loads %s locally with isolated credentials",
    async (kind) => {
      await render(kind);
      expect(construct).toHaveBeenCalledWith(
        expect.any(HTMLElement),
        kind,
        expect.any(Function),
        expect.any(Function),
      );
      expect(fetchFile).toHaveBeenCalledWith(
        `/data/resources/test.${kind}`,
        expect.objectContaining({
          credentials: "same-origin",
          signal: expect.any(AbortSignal),
        }),
      );
      expect(surfaces[0].load).toHaveBeenCalledWith(bytes);
      expect(container.textContent).toContain("Loading Office file");
      expect(
        container
          .querySelector('[aria-label="Zoom in"]')
          ?.hasAttribute("disabled"),
      ).toBe(true);
      await act(async () => surfaces[0].resolve());
      expect(container.querySelector('[role="status"]')).toBeNull();
      expect(
        container.querySelector('[role="region"]')?.getAttribute("aria-busy"),
      ).toBe("false");
    },
  );

  it("uses same-origin credentials mode for external URLs too", async () => {
    await render("docx", false, "https://example.com/report.docx");
    expect(fetchFile.mock.calls[0][1].credentials).toBe("same-origin");
  });

  it("connects zoom controls and reflects library scale changes", async () => {
    await render();
    await act(async () => surfaces[0].resolve());
    await click('[aria-label="Zoom in"]');
    await click('[aria-label="Zoom out"]');
    await click("button:last-child");
    expect(surfaces[0].zoomIn).toHaveBeenCalledOnce();
    expect(surfaces[0].zoomOut).toHaveBeenCalledOnce();
    expect(surfaces[0].fitWidth).toHaveBeenCalledOnce();
    await act(async () => surfaces[0].options.onScaleChange(1.25));
    expect(container.textContent).toContain("125%");
  });

  it("destroys failed parsers and provides retry", async () => {
    await render();
    await act(async () => surfaces[0].reject(new Error("Invalid archive")));
    expect(container.querySelector('[role="alert"]')?.textContent).toContain(
      "Couldn’t load this Office file",
    );
    expect(surfaces[0].destroy).toHaveBeenCalledOnce();
    await click('[role="alert"] button');
    expect(surfaces).toHaveLength(2);
    await act(async () => surfaces[1].resolve());
    expect(container.querySelector('[role="alert"]')).toBeNull();
    expect(surfaces[0].destroy).toHaveBeenCalledOnce();
  });

  it.each([
    ["encrypted", "password protected"],
    ["ooxml-resource-limit", "too complex to preview safely"],
  ])("explains %s errors", async (code, message) => {
    await render();
    await act(async () => surfaces[0].reject({ code }));
    expect(container.querySelector('[role="alert"]')?.textContent).toContain(
      message,
    );
  });

  it("reports HTTP failures without parsing the error response", async () => {
    fetchFile.mockResolvedValue({ ok: false, status: 404 });
    await render();
    expect(container.querySelector('[role="alert"]')).not.toBeNull();
    expect(surfaces[0].load).not.toHaveBeenCalled();
    expect(surfaces[0].destroy).toHaveBeenCalledOnce();
  });

  it("routes asynchronous rendering errors to the fallback", async () => {
    await render();
    await act(async () => surfaces[0].resolve());
    await act(async () =>
      surfaces[0].options.onError(new Error("Render failed")),
    );
    expect(container.querySelector('[role="alert"]')).not.toBeNull();
    expect(surfaces[0].destroy).toHaveBeenCalledOnce();
  });

  it("isolates StrictMode mounts and aborts the active load on close", async () => {
    await render("docx", true);
    expect(surfaces).toHaveLength(2);
    expect(surfaces[0].destroy).toHaveBeenCalledOnce();
    expect(surfaces[0].host.isConnected).toBe(false);
    expect(surfaces[1].host.isConnected).toBe(true);
    expect(surfaces[0].load).not.toHaveBeenCalled();
    const signal = fetchFile.mock.calls[0][1].signal as AbortSignal;
    await act(async () => root.unmount());
    expect(signal.aborted).toBe(true);
    expect(surfaces[1].destroy).toHaveBeenCalledOnce();
    await act(async () => surfaces[1].resolve());
    expect(container.textContent).toBe("");
    root = createRoot(container);
  });
});
