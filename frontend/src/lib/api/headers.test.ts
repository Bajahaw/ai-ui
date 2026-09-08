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

  it("falls back when crypto.randomUUID is missing", () => {
    const original = crypto.randomUUID;
    // @ts-expect-error insecure HTTP (LAN IP) has no randomUUID
    delete crypto.randomUUID;
    try {
      const id = getSessionId();
      expect(id).toMatch(
        /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
      );
    } finally {
      crypto.randomUUID = original;
    }
  });

  it("rotateSessionId issues a new id so recovered SSE is not the stream source", () => {
    const original = getSessionId();
    const rotated = rotateSessionId();

    expect(rotated).not.toBe(original);
    expect(getSessionId()).toBe(rotated);
    expect(sessionStorage.getItem(SESSION_ID_KEY)).toBe(rotated);
  });
});
