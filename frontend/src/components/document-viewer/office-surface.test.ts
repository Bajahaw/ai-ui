import { afterEach, describe, expect, it, vi } from "vitest";
import { createOfficeSurface } from "./office-surface";

const { construct } = vi.hoisted(() => ({ construct: vi.fn() }));
vi.mock("@silurus/ooxml/docx", () => ({
  DocxScrollViewer: class {
    constructor(host: HTMLElement, options: unknown) {
      construct("docx", host, options);
    }
  },
}));
vi.mock("@silurus/ooxml/xlsx", () => ({
  XlsxViewer: class {
    constructor(host: HTMLElement, options: unknown) {
      construct("xlsx", host, options);
    }
  },
}));
vi.mock("@silurus/ooxml/pptx", () => ({
  PptxScrollViewer: class {
    constructor(host: HTMLElement, options: unknown) {
      construct("pptx", host, options);
    }
  },
}));

afterEach(() => {
  construct.mockClear();
  vi.unstubAllGlobals();
});

describe("Office surface adapter", () => {
  it.each(["docx", "xlsx", "pptx"] as const)(
    "selects the %s renderer with safe defaults",
    async (kind) => {
      vi.stubGlobal("Worker", class {});
      vi.stubGlobal("OffscreenCanvas", class {});
      const host = document.createElement("div");
      const onError = vi.fn();
      const onScaleChange = vi.fn();
      await createOfficeSurface(host, kind, onError, onScaleChange);
      expect(construct).toHaveBeenCalledWith(
        kind,
        host,
        expect.objectContaining({
          mode: "worker",
          enableHyperlinks: false,
          onError,
          onScaleChange,
          ...(kind === "xlsx"
            ? { showZoomSlider: false }
            : { enableTextSelection: true, gap: 16 }),
        }),
      );
    },
  );

  it("falls back to main-thread rendering without OffscreenCanvas", async () => {
    vi.stubGlobal("OffscreenCanvas", undefined);
    await createOfficeSurface(
      document.createElement("div"),
      "docx",
      vi.fn(),
      vi.fn(),
    );
    expect(construct).toHaveBeenCalledWith(
      "docx",
      expect.any(HTMLElement),
      expect.objectContaining({ mode: "main" }),
    );
  });
});
