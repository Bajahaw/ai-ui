import { describe, expect, it } from "vitest";
import { filesObjectLiteral, wrapSandboxCode } from "./wrap";

describe("wrapSandboxCode", () => {
  it("returns empty for blank input", () => {
    expect(wrapSandboxCode("  ")).toBe("");
  });

  it("passes HTML through", () => {
    const html = "<div>hi</div><script>sandbox.done()</script>";
    expect(wrapSandboxCode(html)).toBe(html);
  });

  it("wraps bare JS in an async IIFE", () => {
    const out = wrapSandboxCode("console.log(1)");
    expect(out.startsWith("<script>")).toBe(true);
    expect(out).toContain("console.log(1)");
    expect(out).toContain("sandbox.done()");
  });
});

describe("filesObjectLiteral", () => {
  it("builds an empty object", () => {
    expect(filesObjectLiteral([])).toBe("{}");
  });

  it("embeds named base64 payloads", () => {
    const out = filesObjectLiteral([{ name: "a.txt", data: "YQ==" }]);
    expect(out).toContain('"a.txt"');
    expect(out).toContain('"YQ=="');
  });
});
