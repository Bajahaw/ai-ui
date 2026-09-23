import { describe, expect, it } from "vitest";
import { resolveEnterAction } from "./enterBehavior";

describe("resolveEnterAction", () => {
  it("defaults to send when unset or unknown", () => {
    expect(resolveEnterAction(undefined, true)).toBe("send");
    expect(resolveEnterAction(undefined, false)).toBe("send");
    expect(resolveEnterAction("", false)).toBe("send");
    expect(resolveEnterAction("bogus", false)).toBe("send");
  });

  it("honours explicit send and newline regardless of screen size", () => {
    expect(resolveEnterAction("send", false)).toBe("send");
    expect(resolveEnterAction("send", true)).toBe("send");
    expect(resolveEnterAction("newline", false)).toBe("newline");
    expect(resolveEnterAction("newline", true)).toBe("newline");
  });

  it("dynamic sends on large screens and inserts a newline on small ones", () => {
    expect(resolveEnterAction("dynamic", true)).toBe("send");
    expect(resolveEnterAction("dynamic", false)).toBe("newline");
  });
});
