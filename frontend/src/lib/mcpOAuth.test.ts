import { afterEach, describe, expect, it, vi } from "vitest";
import { waitForMCPOAuth } from "./mcpOAuth";

const post = (data: unknown, origin = window.location.origin) =>
  window.dispatchEvent(new MessageEvent("message", { data, origin }));

describe("waitForMCPOAuth", () => {
  afterEach(() => vi.useRealTimers());

  it("resolves on a success message for the same server", async () => {
    const p = waitForMCPOAuth("s1");
    post({ type: "mcp-oauth", serverId: "other", error: "" });
    post({ type: "mcp-oauth", serverId: "s1", error: "" });
    await expect(p).resolves.toBeUndefined();
  });

  it("rejects with the error reported by the callback page", async () => {
    const p = waitForMCPOAuth("s1");
    post({ type: "mcp-oauth", serverId: "s1", error: "access_denied" });
    await expect(p).rejects.toThrow("access_denied");
  });

  it("ignores messages from other origins", async () => {
    vi.useFakeTimers();
    const p = waitForMCPOAuth("s1", 1000);
    post({ type: "mcp-oauth", serverId: "s1", error: "" }, "https://evil.example");
    vi.advanceTimersByTime(1000);
    await expect(p).rejects.toThrow("Timed out");
  });
});
