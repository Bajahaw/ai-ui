import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiErrorHandler } from "./api/errorHandler";
import {
  isBrowserOffline,
  isOfflineError,
  shouldSurfaceConversationsError,
} from "./offline";

describe("offline helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("detects navigator.onLine === false", () => {
    vi.stubGlobal("navigator", { onLine: false });
    expect(isBrowserOffline()).toBe(true);
    expect(isOfflineError(new Error("anything"))).toBe(true);
  });

  it("treats failed fetch as an offline/network error", () => {
    vi.stubGlobal("navigator", { onLine: true });
    expect(isOfflineError(new TypeError("Failed to fetch"))).toBe(true);
    expect(ApiErrorHandler.isNetworkError(new TypeError("Failed to fetch"))).toBe(
      true,
    );
  });

  it("hides conversation errors when local data exists or the network is down", () => {
    vi.stubGlobal("navigator", { onLine: false });
    expect(
      shouldSurfaceConversationsError(new Error("Failed to fetch"), false),
    ).toBe(false);
    vi.stubGlobal("navigator", { onLine: true });
    expect(
      shouldSurfaceConversationsError(new TypeError("Failed to fetch"), true),
    ).toBe(false);
    expect(shouldSurfaceConversationsError(new Error("boom (500)"), false)).toBe(
      true,
    );
  });
});
