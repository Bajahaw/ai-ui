import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ImageViewer } from "./image-viewer";

let root: Root;
let container: HTMLDivElement;
let resize: () => void;
let width: number;

beforeEach(async () => {
  width = 600;
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockImplementation(
    () => width,
  );
  vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(500);
  vi.stubGlobal(
    "ResizeObserver",
    class {
      constructor(callback: () => void) {
        resize = callback;
      }
      observe() {}
      disconnect() {}
    },
  );
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
  await act(async () =>
    root.render(
      createElement(ImageViewer, {
        document: {
          href: "/data/resources/photo.png",
          filename: "Photo.png",
          kind: "image",
        },
      }),
    ),
  );
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

async function loadImage(width = 1200, height = 800) {
  const image = container.querySelector("img")!;
  Object.defineProperties(image, {
    naturalWidth: { value: width },
    naturalHeight: { value: height },
  });
  await act(async () => image.dispatchEvent(new Event("load")));
  return image;
}

describe("image renderer", () => {
  it("shows loading, fits large images, and enables zoom after load", async () => {
    expect(container.querySelector('[role="status"]')?.textContent).toContain(
      "Loading image",
    );
    expect(
      container.querySelector<HTMLButtonElement>('[aria-label="Zoom in"]')
        ?.disabled,
    ).toBe(true);
    const image = await loadImage();
    expect(container.querySelector('[role="status"]')).toBeNull();
    expect(parseFloat(image.style.width)).toBeCloseTo(568);
    expect(parseFloat(image.style.height)).toBeCloseTo((568 * 800) / 1200);
    await act(async () =>
      container
        .querySelector<HTMLButtonElement>('[aria-label="Zoom in"]')!
        .click(),
    );
    expect(parseFloat(image.style.width)).toBeCloseTo(710);
    expect(container.textContent).toContain("125%");
  });

  it("refits on resize and does not upscale small images", async () => {
    const image = await loadImage(300, 200);
    expect(parseFloat(image.style.width)).toBe(300);
    await act(async () => {
      width = 200;
      resize();
    });
    expect(parseFloat(image.style.width)).toBeCloseTo(168);
  });

  it("offers retry for failed images and recovers after load", async () => {
    await act(async () =>
      container.querySelector("img")!.dispatchEvent(new Event("error")),
    );
    expect(container.querySelector('[role="alert"]')?.textContent).toContain(
      "Couldn’t load this image",
    );
    const oldImage = container.querySelector("img");
    await act(async () =>
      container
        .querySelector<HTMLButtonElement>('[role="alert"] button')!
        .click(),
    );
    expect(container.querySelector("img")).not.toBe(oldImage);
    expect(container.querySelector('[role="status"]')).not.toBeNull();
    await loadImage();
    expect(container.querySelector('[role="alert"]')).toBeNull();
  });
});
