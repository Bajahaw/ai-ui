import { afterEach, describe, expect, it, vi } from "vitest";
import {
  getSandboxFrameOrigin,
  parseSandboxOrigin,
  resetSandboxOriginCache,
  resolveSandboxFrameOrigin,
} from "./origin";

describe("parseSandboxOrigin", () => {
  it("accepts http and https origins", () => {
    expect(parseSandboxOrigin("https://sandbox.example.com")).toBe(
      "https://sandbox.example.com",
    );
    expect(parseSandboxOrigin("http://sandbox.localhost:8080/extra")).toBe(
      "http://sandbox.localhost:8080",
    );
  });

  it("rejects empty, bad schemes, and credentials", () => {
    expect(parseSandboxOrigin("")).toBe("");
    expect(parseSandboxOrigin("ftp://sandbox.example.com")).toBe("");
    expect(parseSandboxOrigin("https://user:pass@sandbox.example.com")).toBe("");
    expect(parseSandboxOrigin("not a url")).toBe("");
  });
});

describe("resolveSandboxFrameOrigin", () => {
  it("falls back when unset or same as the page", () => {
    expect(resolveSandboxFrameOrigin("", "https://app.example.com")).toBe("");
    expect(
      resolveSandboxFrameOrigin(
        "https://app.example.com",
        "https://app.example.com",
      ),
    ).toBe("");
    expect(
      resolveSandboxFrameOrigin(
        "https://sandbox.example.com",
        "https://app.example.com",
      ),
    ).toBe("https://sandbox.example.com");
  });
});

describe("getSandboxFrameOrigin", () => {
  afterEach(() => {
    resetSandboxOriginCache();
    vi.unstubAllGlobals();
  });

  it("reads sandbox_origin from /api/version", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        ok: true,
        json: async () => ({ sandbox_origin: "https://sandbox.example.com" }),
      })),
    );
    await expect(getSandboxFrameOrigin()).resolves.toBe(
      "https://sandbox.example.com",
    );
  });
});
