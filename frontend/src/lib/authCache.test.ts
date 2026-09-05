import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { clearAuthCache, readAuthCache, writeAuthCache } from "./authCache";

function stubLocalStorage() {
  const store = new Map<string, string>();
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => {
      store.set(key, value);
    },
    removeItem: (key: string) => {
      store.delete(key);
    },
    clear: () => store.clear(),
  });
}

describe("authCache", () => {
  beforeEach(() => {
    stubLocalStorage();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("round-trips authenticated state and keeps userId across writes", () => {
    expect(readAuthCache()).toBeNull();
    writeAuthCache({ authenticated: true, userId: "user-1" });
    expect(readAuthCache()).toEqual({ authenticated: true, userId: "user-1" });
    writeAuthCache({ authenticated: true });
    expect(readAuthCache()).toEqual({ authenticated: true, userId: "user-1" });
  });

  it("clears stored auth", () => {
    writeAuthCache({ authenticated: true, userId: "user-1" });
    clearAuthCache();
    expect(readAuthCache()).toBeNull();
  });

  it("ignores corrupt storage", () => {
    localStorage.setItem("ai-ui-auth-cache", "{not-json");
    expect(readAuthCache()).toBeNull();
  });
});
