import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { OfficeThumbnail } from "./office-thumbnail";

const { acquire, release } = vi.hoisted(() => ({
  acquire: vi.fn(),
  release: vi.fn(),
}));
vi.mock("@/lib/document-rendering/office-thumbnail", () => ({
  acquireOfficeThumbnail: acquire,
}));
let root: Root;
let container: HTMLDivElement;
let visibility: (entries: { isIntersecting: boolean }[]) => void;
let finish: (url: string) => void;

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.stubGlobal(
    "IntersectionObserver",
    class {
      constructor(callback: typeof visibility) {
        visibility = callback;
      }
      observe() {}
      disconnect() {}
    },
  );
  release.mockReset();
  acquire.mockReset().mockImplementation(() => ({
    promise: new Promise<string>((resolve) => {
      finish = resolve;
    }),
    release,
  }));
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});
async function render() {
  await act(async () =>
    root.render(
      createElement(OfficeThumbnail, {
        file: {
          id: "a",
          name: "a.docx",
          path: "a.docx",
          type: "application/octet-stream",
        },
        alt: "a.docx",
      }),
    ),
  );
}

describe("Office thumbnail visibility", () => {
  it("does no work until visible, then displays the separately stored image", async () => {
    await render();
    expect(acquire).not.toHaveBeenCalled();
    await act(async () => visibility([{ isIntersecting: true }]));
    expect(acquire).toHaveBeenCalledOnce();
    await act(async () => finish("/data/resources/thumbs/a.jpg"));
    expect(container.querySelector("img")?.getAttribute("src")).toBe(
      "/data/resources/thumbs/a.jpg",
    );
    expect(release).toHaveBeenCalledOnce();
  });

  it("releases in-flight work when it scrolls away", async () => {
    await render();
    await act(async () => visibility([{ isIntersecting: true }]));
    await act(async () => visibility([{ isIntersecting: false }]));
    expect(release).toHaveBeenCalledOnce();
    await act(async () => finish("/late.jpg"));
    expect(container.querySelector("img")).toBeNull();
  });
});
