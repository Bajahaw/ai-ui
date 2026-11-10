import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FileCard } from "@/components/ai-elements/file-card";
import { DocumentPreviewProvider } from "./document-preview-provider";
import { DocumentViewer } from "./document-viewer";

vi.mock("./pdf-viewer", () => ({
  default: ({ document }: { document: { filename: string } }) =>
    createElement("div", { "data-testid": "pdf" }, document.filename),
}));

let root: Root;
let container: HTMLDivElement;
let desktop: boolean;
let viewportChanged: (() => void) | undefined;

beforeEach(() => {
  desktop = true;
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
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
