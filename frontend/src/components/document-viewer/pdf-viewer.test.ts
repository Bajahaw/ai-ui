import { act, createElement, StrictMode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import PdfViewer from "./pdf-viewer";

const { getDocument, cancelRender, cancelText, renderPage } = vi.hoisted(
  () => ({
    getDocument: vi.fn(),
    cancelRender: vi.fn(),
    cancelText: vi.fn(),
    renderPage: vi.fn(),
  }),
);

vi.mock("pdfjs-dist", () => ({
  getDocument,
  GlobalWorkerOptions: {},
  TextLayer: class {
    render = () => Promise.resolve();
    cancel = cancelText;
  },
}));

type LoadingTask = {
  promise: Promise<unknown>;
  resolve: (value: unknown) => void;
  reject: (error: Error) => void;
  destroy: ReturnType<typeof vi.fn>;
  onPassword?: () => void;
};

let root: Root;
let container: HTMLDivElement;
let tasks: LoadingTask[];

function loadedPdf() {
  return {
    numPages: 2,
    getPage: vi.fn(async () => ({
      getViewport: ({ scale }: { scale: number }) => ({
        width: 612 * scale,
        height: 792 * scale,
        scale,
        userUnit: 1,
      }),
      streamTextContent: vi.fn(),
      render: renderPage,
    })),
  };
}

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(600);
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  vi.stubGlobal(
    "IntersectionObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  tasks = [];
  renderPage
    .mockReset()
    .mockImplementation(() => ({
      promise: Promise.resolve(),
      cancel: cancelRender,
    }));
  cancelRender.mockReset();
  cancelText.mockReset();
  getDocument.mockReset().mockImplementation(() => {
    let resolve!: LoadingTask["resolve"];
    let reject!: LoadingTask["reject"];
    const promise = new Promise((res, rej) => {
      resolve = res;
      reject = rej;
    });
    const task = { promise, resolve, reject, destroy: vi.fn(async () => {}) };
    tasks.push(task);
    return task;
  });
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

async function render(strict = false) {
  const viewer = createElement(PdfViewer, {
    document: {
      href: "/data/resources/test.pdf",
      filename: "Test",
      kind: "pdf",
    },
  });
  await act(async () => {
    root.render(strict ? createElement(StrictMode, null, viewer) : viewer);
  });
}

describe("PDF renderer lifecycle", () => {
  it("loads local assets without forwarding credentials and renders nearby pages only", async () => {
    await render();
    expect(container.textContent).toContain("Loading PDF");
    expect(getDocument).toHaveBeenCalledWith(
      expect.objectContaining({
        url: "/data/resources/test.pdf",
        withCredentials: false,
        cMapUrl: "/pdfjs/cmaps/",
        standardFontDataUrl: "/pdfjs/standard_fonts/",
        wasmUrl: "/pdfjs/wasm/",
      }),
    );
    const pdf = loadedPdf();
    await act(async () => tasks[0].resolve(pdf));
    expect(container.textContent).toContain("2 pages");
    expect(container.querySelectorAll(".pdf-document-page")).toHaveLength(2);
    expect(pdf.getPage).toHaveBeenCalledTimes(1);
    expect(container.querySelector("canvas")?.width).toBe(568);
  });

  it("cancels old page renders during zoom and destroys the PDF on unmount", async () => {
    await render();
    await act(async () => tasks[0].resolve(loadedPdf()));
    const oldCanvas = container.querySelector("canvas")!;
    await act(async () =>
      container
        .querySelector<HTMLButtonElement>('[aria-label="Zoom in"]')!
        .click(),
    );
    expect(container.textContent).toContain("125%");
    expect(cancelRender).toHaveBeenCalled();
    expect(cancelText).toHaveBeenCalled();
    expect(oldCanvas.width).toBe(0);
    expect(container.querySelector("canvas")?.width).toBe(710);
    await act(async () => root.unmount());
    expect(tasks[0].destroy).toHaveBeenCalledOnce();
    root = createRoot(container);
  });

  it("provides retry after a failed load", async () => {
    await render();
    await act(async () => tasks[0].reject(new Error("Network failure")));
    expect(container.querySelector('[role="alert"]')?.textContent).toContain(
      "Couldn’t load this PDF",
    );
    await act(async () => container.querySelector("button")!.click());
    expect(tasks[0].destroy).toHaveBeenCalledOnce();
    expect(tasks).toHaveLength(2);
    await act(async () => tasks[1].resolve(loadedPdf()));
    expect(container.querySelector('[role="alert"]')).toBeNull();
    expect(container.textContent).toContain("2 pages");
  });

  it("explains password-protected documents instead of leaving a spinner", async () => {
    await render();
    await act(async () => tasks[0].onPassword?.());
    expect(container.querySelector('[role="alert"]')?.textContent).toContain(
      "password protected",
    );
    expect(tasks[0].destroy).toHaveBeenCalled();
  });

  it("ignores stale completions under StrictMode", async () => {
    await render(true);
    expect(tasks).toHaveLength(2);
    expect(tasks[0].destroy).toHaveBeenCalledOnce();
    await act(async () => tasks[0].resolve(loadedPdf()));
    expect(container.textContent).toContain("Loading PDF");
    await act(async () => tasks[1].resolve(loadedPdf()));
    expect(container.textContent).toContain("2 pages");
  });
});
