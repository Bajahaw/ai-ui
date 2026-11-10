import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderDocumentScreenshot } from "./screenshot";

const { render } = vi.hoisted(() => ({ render: vi.fn() }));
vi.mock("./office", () => ({ renderOfficeScreenshot: render }));
const source = (filename = "test.docx") => ({
  data: new ArrayBuffer(8),
  filename,
});

beforeEach(() => {
  render
    .mockReset()
    .mockResolvedValue(new Blob(["png"], { type: "image/png" }));
});
afterEach(() => vi.restoreAllMocks());

describe("document screenshot dispatch", () => {
  it.each(["docx", "xlsx", "pptx"])(
    "lazily dispatches %s with format-neutral defaults",
    async (format) => {
      const image = await renderDocumentScreenshot(source(`test.${format}`));
      expect(image.type).toBe("image/png");
      expect(render).toHaveBeenCalledWith(
        expect.objectContaining({ filename: `test.${format}` }),
        {
          page: 1,
          width: 1200,
          height: 1600,
          range: { row: 1, col: 1, rows: 20, cols: 8 },
        },
        expect.any(AbortSignal),
      );
    },
  );

  it.each([
    ["test.pdf", {}, "DOCX, XLSX, and PPTX"],
    ["test.docx", { width: 4096 }, "width"],
    ["test.docx", { page: 0 }, "page"],
    ["test.docx", { height: 1.5 }, "height"],
    ["test.docx", { range: { row: 1, col: 1, rows: 1, cols: 1 } }, "XLSX"],
    [
      "test.xlsx",
      { range: { row: 1, col: 1, rows: 200, cols: 100 } },
      "range exceeds",
    ],
    [
      "test.xlsx",
      { range: { row: 1_048_576, col: 1, rows: 2, cols: 1 } },
      "range exceeds",
    ],
  ])(
    "rejects invalid requests without loading a renderer: %s %j",
    async (filename, options, error) => {
      await expect(
        renderDocumentScreenshot(source(filename), options),
      ).rejects.toThrow(error);
      expect(render).not.toHaveBeenCalled();
    },
  );

  it("rejects empty and oversized sources", async () => {
    await expect(
      renderDocumentScreenshot({ ...source(), data: new ArrayBuffer(0) }),
    ).rejects.toThrow("50MB");
    await expect(
      renderDocumentScreenshot({
        ...source(),
        data: new ArrayBuffer(50 * 1024 * 1024 + 1),
      }),
    ).rejects.toThrow("50MB");
    expect(render).not.toHaveBeenCalled();
  });

  it("serializes work and does not run cancelled queued jobs", async () => {
    let finish!: (blob: Blob) => void;
    render.mockImplementationOnce(
      () =>
        new Promise<Blob>((resolve) => {
          finish = resolve;
        }),
    );
    const first = renderDocumentScreenshot(source());
    await vi.waitFor(() => expect(render).toHaveBeenCalledOnce());
    const controller = new AbortController();
    const second = renderDocumentScreenshot(
      source("second.xlsx"),
      {},
      controller.signal,
    );
    const secondResult = expect(second).rejects.toThrow();
    controller.abort();
    expect(render).toHaveBeenCalledOnce();
    finish(new Blob(["png"]));
    await first;
    await secondResult;
    expect(render).toHaveBeenCalledOnce();
    await renderDocumentScreenshot(source("third.pptx"));
    expect(render).toHaveBeenCalledTimes(2);
  });
});
