import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderOfficeScreenshot } from "./office";

const { load } = vi.hoisted(() => ({ load: vi.fn() }));
vi.mock("@silurus/ooxml/docx", () => ({ DocxDocument: { load } }));
vi.mock("@silurus/ooxml/pptx", () => ({ PptxPresentation: { load } }));
vi.mock("@silurus/ooxml/xlsx", () => ({ XlsxWorkbook: { load } }));
const options = {
  page: 1,
  width: 360,
  height: 360,
  range: { row: 1, col: 1, rows: 20, cols: 8 },
};
let engine: Record<string, ReturnType<typeof vi.fn> | number>;
let bitmap: { width: number; height: number; close: ReturnType<typeof vi.fn> };
let encode: ReturnType<typeof vi.fn>;

beforeEach(() => {
  bitmap = { width: 270, height: 360, close: vi.fn() };
  engine = {
    pageCount: 2,
    slideCount: 2,
    sheetCount: 2,
    slideWidth: 800,
    slideHeight: 600,
    destroy: vi.fn(),
    pageSize: vi.fn(() => ({ widthPt: 600, heightPt: 800 })),
    renderPageToBitmap: vi.fn(async () => bitmap),
    renderSlideToBitmap: vi.fn(async () => bitmap),
    renderViewportToBitmap: vi.fn(async () => bitmap),
  };
  load.mockReset().mockResolvedValue(engine);
  encode = vi.fn(async () => new Blob(["png"], { type: "image/png" }));
  vi.stubGlobal("Worker", class {});
  vi.stubGlobal(
    "OffscreenCanvas",
    class {
      getContext() {
        return { fillRect: vi.fn(), drawImage: vi.fn() };
      }
      convertToBlob = encode;
    },
  );
});
afterEach(() => vi.unstubAllGlobals());

describe("Office headless screenshot adapter", () => {
  it.each([
    ["docx", "renderPageToBitmap"],
    ["pptx", "renderSlideToBitmap"],
    ["xlsx", "renderViewportToBitmap"],
  ])(
    "renders %s, copies input, and releases all resources",
    async (kind, method) => {
      const bytes = new ArrayBuffer(8);
      const result = await renderOfficeScreenshot(
        { data: bytes, filename: `report.${kind}` },
        options,
        new AbortController().signal,
      );
      expect(result.type).toBe("image/png");
      expect(load.mock.calls[0][0]).not.toBe(bytes);
      expect(bytes.byteLength).toBe(8);
      expect(load.mock.calls[0][1]).toMatchObject({
        mode: "worker",
        workerTimeoutMs: 20_000,
      });
      expect(engine[method]).toHaveBeenCalled();
      expect(engine.destroy).toHaveBeenCalledOnce();
      expect(bitmap.close).toHaveBeenCalledOnce();
      if (kind === "docx")
        expect(engine[method]).toHaveBeenCalledWith(0, { width: 270, dpr: 1 });
      if (kind === "pptx")
        expect(engine[method]).toHaveBeenCalledWith(0, { width: 360, dpr: 1 });
      if (kind === "xlsx")
        expect(engine[method]).toHaveBeenCalledWith(0, options.range, {
          width: 360,
          height: 360,
          dpr: 1,
        });
    },
  );

  it("destroys the engine after an out-of-range request", async () => {
    await expect(
      renderOfficeScreenshot(
        { data: new ArrayBuffer(8), filename: "a.docx" },
        { ...options, page: 3 },
        new AbortController().signal,
      ),
    ).rejects.toThrow("out of range");
    expect(engine.destroy).toHaveBeenCalledOnce();
  });

  it("closes bitmaps and destroys engines after encoding failure", async () => {
    encode.mockRejectedValue(new Error("Encoding failed"));
    await expect(
      renderOfficeScreenshot(
        { data: new ArrayBuffer(8), filename: "a.docx" },
        options,
        new AbortController().signal,
      ),
    ).rejects.toThrow("Encoding failed");
    expect(bitmap.close).toHaveBeenCalledOnce();
    expect(engine.destroy).toHaveBeenCalledOnce();
  });

  it("disposes late loads after cancellation without rendering", async () => {
    const controller = new AbortController();
    let finish!: (value: unknown) => void;
    load.mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const pending = renderOfficeScreenshot(
      { data: new ArrayBuffer(8), filename: "a.docx" },
      options,
      controller.signal,
    );
    const rejected = expect(pending).rejects.toThrow();
    await vi.waitFor(() => expect(load).toHaveBeenCalled());
    controller.abort();
    finish(engine);
    await rejected;
    expect(engine.destroy).toHaveBeenCalledOnce();
    expect(engine.renderPageToBitmap).not.toHaveBeenCalled();
  });
});
