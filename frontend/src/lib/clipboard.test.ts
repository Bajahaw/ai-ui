import { afterEach, describe, expect, it, vi } from "vitest";
import {
  filesFromClipboard,
  filesFromImageElements,
  readClipboardImageFiles,
} from "./clipboard";

function item(
  kind: string,
  file: File | null,
  type = file?.type ?? "",
): DataTransferItem {
  return {
    kind,
    type,
    getAsFile: () => file,
  } as DataTransferItem;
}

function clipboard(partial: {
  files?: File[] | null;
  items?: DataTransferItem[] | null;
}): DataTransfer {
  const files = partial.files;
  const items = partial.items;
  return {
    files: files
      ? Object.assign(files.slice(), {
          item: (i: number) => files[i] ?? null,
        })
      : null,
    items: items ? Object.assign(items.slice(), {}) : null,
  } as unknown as DataTransfer;
}

describe("filesFromClipboard", () => {
  it("returns files when DataTransfer.files is populated", () => {
    const file = new File(["img"], "shot.png", { type: "image/png" });
    expect(filesFromClipboard(clipboard({ files: [file] }))).toEqual([file]);
  });

  it("falls back to items when files is empty", () => {
    const file = new File(["img"], "shot.png", { type: "image/png" });
    expect(
      filesFromClipboard(
        clipboard({ files: [], items: [item("file", file)] }),
      ),
    ).toEqual([file]);
  });

  it("falls back to items when files is null", () => {
    const file = new File(["img"], "shot.png", { type: "image/png" });
    expect(
      filesFromClipboard(
        clipboard({ files: null, items: [item("file", file)] }),
      ),
    ).toEqual([file]);
  });

  it("accepts image items even when kind is not file", () => {
    const file = new File(["img"], "shot.png", { type: "image/png" });
    expect(
      filesFromClipboard(
        clipboard({ items: [item("string", file, "image/png")] }),
      ),
    ).toEqual([file]);
  });

  it("ignores string items and empty input", () => {
    expect(filesFromClipboard(null)).toEqual([]);
    expect(
      filesFromClipboard(clipboard({ items: [item("string", null)] })),
    ).toEqual([]);
  });
});

describe("readClipboardImageFiles", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("reads image types from the async clipboard API", async () => {
    const blob = new Blob(["img"], { type: "image/png" });
    vi.stubGlobal("navigator", {
      clipboard: {
        read: async () => [
          {
            types: ["text/plain", "image/png"],
            getType: async (type: string) => {
              if (type !== "image/png") throw new Error("unexpected");
              return blob;
            },
          },
        ],
      },
    });

    const files = await readClipboardImageFiles();
    expect(files).toHaveLength(1);
    expect(files[0].type).toBe("image/png");
    expect(files[0].name).toBe("pasted-image.png");
  });

  it("returns empty when clipboard read is denied", async () => {
    vi.stubGlobal("navigator", {
      clipboard: {
        read: async () => {
          throw new Error("NotAllowedError");
        },
      },
    });
    expect(await readClipboardImageFiles()).toEqual([]);
  });
});

describe("filesFromImageElements", () => {
  it("converts data-url images and skips remote urls", async () => {
    const local = document.createElement("img");
    local.src = "data:image/png;base64,aW1n";
    const remote = document.createElement("img");
    remote.src = "https://example.com/a.png";

    const files = await filesFromImageElements([local, remote]);
    expect(files).toHaveLength(1);
    expect(files[0].name).toBe("pasted-image.png");
  });
});
