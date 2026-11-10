import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FileCard } from "@/components/ai-elements/file-card";
import { AttachmentMessage } from "@/components/ui/file-upload";
import type { Attachment } from "@/lib/api/types";
import { DocumentPreviewProvider } from "./document-preview-provider";
import { DocumentViewer } from "./document-viewer";

vi.mock("./pdf-viewer", () => ({
  default: ({ document }: { document: { filename: string } }) =>
    createElement("div", { "data-testid": "pdf" }, document.filename),
}));

vi.mock("./office-viewer", () => ({
  default: ({ document }: { document: { filename: string; kind: string } }) =>
    createElement("div", { "data-testid": document.kind }, document.filename),
}));

let root: Root;
let container: HTMLDivElement;
let desktop: boolean;
let viewportChanged: (() => void) | undefined;

beforeEach(() => {
  desktop = true;
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => ({
      get matches() {
        return desktop;
      },
      addEventListener: (_: string, callback: () => void) => {
        viewportChanged = callback;
      },
      removeEventListener: vi.fn(),
    })),
  );
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});

const attachments: Attachment[] = [
  { name: "Photo.png", type: "image/png" },
  { name: "Uploaded.pdf", type: "application/pdf" },
  {
    name: "Notes.docx",
    type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  },
  { name: "Legacy.doc", type: "application/msword" },
  {
    name: "Budget.xlsx",
    type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  },
  {
    name: "Slides.pptx",
    type: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  },
].map(({ name, type }, index) => ({
  id: `attachment-${index}`,
  messageId: 1,
  file: {
    id: `upload-${index}`,
    name,
    type,
    size: 1024,
    path: `data/resources/upload-${index}`,
    content: "",
    createdAt: "2026-11-10T00:00:00Z",
  },
}));

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});

async function render(scopeKey = "chat-a") {
  await act(async () => {
    root.render(
      createElement(
        DocumentPreviewProvider,
        { scopeKey },
        createElement(FileCard, {
          key: "pdf",
          href: "/data/resources/report.pdf",
          filename: createElement("strong", null, "Report"),
        }),
        createElement(FileCard, {
          key: "pdf2",
          href: "/data/resources/second.pdf",
          filename: "Second",
        }),
        createElement(FileCard, {
          key: "word",
          href: "/data/resources/report.docx",
          filename: "Word document",
        }),
        createElement(AttachmentMessage, { key: "uploads", attachments }),
        createElement(DocumentViewer, { key: "viewer" }),
      ),
    );
  });
}

async function click(selector: string) {
  const element = document.querySelector<HTMLElement>(selector)!;
  expect(element).not.toBeNull();
  let event: MouseEvent;
  await act(async () => {
    event = new MouseEvent("click", { bubbles: true, cancelable: true });
    element.dispatchEvent(event);
  });
  return event!;
}

describe("document viewer integration", () => {
  it("opens uploaded images and PDFs using metadata rather than resource extensions", async () => {
    await render();
    const event = await click('[aria-label="Preview Photo.png"]');
    expect(event.defaultPrevented).toBe(true);
    const image = document.querySelector<HTMLImageElement>(
      'aside img[alt="Photo.png"]',
    );
    expect(image?.getAttribute("src")).toBe("/data/resources/upload-0");
    expect(document.querySelector("aside header")?.textContent).toContain(
      "PNG",
    );
    await click('[aria-label="Preview Uploaded.pdf"]');
    expect(document.querySelector('[data-testid="pdf"]')?.textContent).toBe(
      "Uploaded.pdf",
    );
    await click('[aria-label="Close document preview"]');
    expect(document.activeElement?.getAttribute("aria-label")).toBe(
      "Preview Uploaded.pdf",
    );
  });

  it("opens unsupported uploads with a short note and direct download", async () => {
    await render();
    await click('[aria-label="Preview Legacy.doc"]');
    expect(document.querySelector("aside")?.textContent).toContain(
      "Preview isn’t available for this file type yet.",
    );
    expect(document.querySelector("aside header")?.textContent).toContain(
      "DOC",
    );
    const download = document.querySelector<HTMLAnchorElement>(
      "aside a[download]:not([aria-label])",
    );
    expect(download?.textContent).toContain("Download file");
    expect(download?.getAttribute("href")).toBe("/data/resources/upload-3");
    expect(download?.getAttribute("download")).toBe("Legacy.doc");
    expect(
      document.querySelector("aside iframe, aside object, aside embed"),
    ).toBeNull();
  });

  it.each([
    ["Notes.docx", "docx"],
    ["Budget.xlsx", "xlsx"],
    ["Slides.pptx", "pptx"],
  ])("opens uploaded %s in the Office renderer", async (name, kind) => {
    await render();
    await click(`[aria-label="Preview ${name}"]`);
    expect(
      document.querySelector(`aside [data-testid="${kind}"]`)?.textContent,
    ).toBe(name);
    expect(document.querySelector("aside header")?.textContent).toContain(
      kind.toUpperCase(),
    );
    expect(
      document.querySelector(`aside a[download="${name}"]`),
    ).not.toBeNull();
  });

  it("previews generated Office links even when the display label has no extension", async () => {
    await render();
    await click('[aria-label="Preview Word document"]');
    expect(
      document.querySelector('aside [data-testid="docx"]')?.textContent,
    ).toBe("Word document");
    expect(document.querySelector("aside header")?.textContent).toContain(
      "DOCX",
    );
  });

  it("supports legacy image attachments and keeps native links outside the viewer provider", async () => {
    await act(async () =>
      root.render(
        createElement(
          DocumentPreviewProvider,
          { scopeKey: "legacy" },
          createElement(AttachmentMessage, {
            attachment: "data/resources/old.jpg",
            filename: "Old photo.jpg",
          }),
          createElement(DocumentViewer),
        ),
      ),
    );
    await click('[aria-label="Preview Old photo.jpg"]');
    expect(
      document
        .querySelector('aside img[alt="Old photo.jpg"]')
        ?.getAttribute("src"),
    ).toBe("/data/resources/old.jpg");
    await act(async () =>
      root.render(createElement(AttachmentMessage, { attachments })),
    );
    expect(
      container
        .querySelector('a[aria-label="Photo.png"]')
        ?.getAttribute("target"),
    ).toBe("_blank");
    expect(document.querySelector("aside")).toBeNull();
  });

  it("opens image attachments in the mobile bottom sheet", async () => {
    desktop = false;
    await render();
    await click('[aria-label="Preview Photo.png"]');
    expect(
      document.querySelector('[role="dialog"] img[alt="Photo.png"]'),
    ).not.toBeNull();
    await click('[aria-label="Close document preview"]');
    await vi.waitFor(() =>
      expect(document.activeElement?.getAttribute("aria-label")).toBe(
        "Preview Photo.png",
      ),
    );
  });

  it("opens PDF cards beside chat, switches files, and restores focus on close", async () => {
    await render();
    const event = await click('[aria-label="Preview Report"]');
    expect(event.defaultPrevented).toBe(true);
    expect(document.querySelector("aside")?.textContent).toContain("Report");
    expect(document.querySelectorAll('[data-testid="pdf"]')).toHaveLength(1);
    await click('[aria-label="Preview Second"]');
    expect(document.querySelector("aside")?.textContent).toContain("Second");
    expect(document.querySelectorAll("aside")).toHaveLength(1);
    await click('[aria-label="Close document preview"]');
    expect(document.querySelector("aside")).toBeNull();
    expect(document.activeElement?.getAttribute("aria-label")).toBe(
      "Preview Second",
    );
  });

  it("keeps downloads and modified clicks as native links", async () => {
    await render();
    expect(
      document.querySelector('a[href$=".docx"]')?.hasAttribute("download"),
    ).toBe(true);
    expect(
      document
        .querySelector('a[aria-label="Download Report"]')
        ?.hasAttribute("download"),
    ).toBe(true);
    const event = new MouseEvent("click", {
      bubbles: true,
      cancelable: true,
      ctrlKey: true,
    });
    let intercepted = true;
    document.addEventListener(
      "click",
      (nativeEvent) => {
        intercepted = nativeEvent.defaultPrevented;
        nativeEvent.preventDefault(); // Prevent jsdom trying to navigate after the assertion.
      },
      { once: true },
    );
    await act(async () => {
      document
        .querySelector('[aria-label="Preview Report"]')!
        .dispatchEvent(event);
    });
    expect(intercepted).toBe(false);
    expect(document.querySelector("aside")).toBeNull();
  });

  it("closes on Escape and on conversation navigation, without reopening on return", async () => {
    await render();
    await click('[aria-label="Preview Report"]');
    await act(async () => {
      document.dispatchEvent(
        new KeyboardEvent("keydown", { key: "Escape", bubbles: true }),
      );
    });
    expect(document.querySelector("aside")).toBeNull();
    await click('[aria-label="Preview Report"]');
    await render("chat-b");
    expect(document.querySelector("aside")).toBeNull();
    await render("chat-a");
    expect(document.querySelector("aside")).toBeNull();
  });

  it("uses a modal bottom sheet on mobile and adapts to viewport changes", async () => {
    await render();
    await click('[aria-label="Preview Report"]');
    await act(async () => {
      desktop = false;
      viewportChanged?.();
    });
    const dialog = document.querySelector('[role="dialog"]');
    expect(dialog?.classList.contains("document-viewer-sheet")).toBe(true);
    expect(dialog?.getAttribute("aria-labelledby")).toBeTruthy();
    expect(document.querySelector("aside")).toBeNull();
    await click('[aria-label="Close document preview"]');
    expect(document.querySelector('[role="dialog"]')).toBeNull();
    // Radix restores focus after its focus-scope cleanup has finished.
    await vi.waitFor(() =>
      expect(document.activeElement?.getAttribute("aria-label")).toBe(
        "Preview Report",
      ),
    );
  });
});
