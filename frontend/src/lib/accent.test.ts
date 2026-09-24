import { describe, expect, it } from "vitest";
import {
  ACCENTS,
  DEFAULT_ACCENT,
  applyAccentToDocument,
  isAccentId,
  resolveAccent,
} from "./accent";

describe("resolveAccent", () => {
  it("falls back to the default when unset or unknown", () => {
    expect(resolveAccent(undefined)).toBe(DEFAULT_ACCENT);
    expect(resolveAccent("")).toBe(DEFAULT_ACCENT);
    expect(resolveAccent("hotpink")).toBe(DEFAULT_ACCENT);
    expect(resolveAccent(42)).toBe(DEFAULT_ACCENT);
  });

  it("accepts every known preset", () => {
    for (const { id } of ACCENTS) {
      expect(isAccentId(id)).toBe(true);
      expect(resolveAccent(id)).toBe(id);
    }
  });
});

describe("applyAccentToDocument", () => {
  const fakeRoot = () => ({ dataset: {} as DOMStringMap }) as HTMLElement;

  it("sets data-accent for coloured presets", () => {
    const root = fakeRoot();
    applyAccentToDocument("blue", root);
    expect(root.dataset.accent).toBe("blue");
  });

  it("clears data-accent for the neutral default", () => {
    const root = fakeRoot();
    applyAccentToDocument("pink", root);
    applyAccentToDocument(DEFAULT_ACCENT, root);
    expect(root.dataset.accent).toBeUndefined();
  });
});
