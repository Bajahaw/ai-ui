import { beforeEach, describe, expect, it } from "vitest";
import { getSessionId, rotateSessionId, SESSION_ID_KEY } from "./headers";

describe("session id", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it("reuses the stored session id", () => {
    const first = getSessionId();
    const second = getSessionId();
    expect(first).toBe(second);
    expect(sessionStorage.getItem(SESSION_ID_KEY)).toBe(first);
  });

  it("rotateSessionId issues a new id so recovered SSE is not the stream source", () => {
    const original = getSessionId();
    const rotated = rotateSessionId();

    expect(rotated).not.toBe(original);
    expect(getSessionId()).toBe(rotated);
    expect(sessionStorage.getItem(SESSION_ID_KEY)).toBe(rotated);
  });
});
