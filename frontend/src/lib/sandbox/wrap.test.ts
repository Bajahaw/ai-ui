import { describe, expect, it } from "vitest";
import { collectConversationFileIds } from "./runner";
import { filesObjectLiteral, isSandboxHtml, wrapSandboxCode } from "./wrap";

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

  it("wraps JS starting with `<` instead of hanging without done()", () => {
    expect(isSandboxHtml("if (a < b) { console.log(1) }")).toBe(false);
    expect(wrapSandboxCode("if (a < b) { console.log(1) }")).toContain(
      "sandbox.done()",
    );
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

describe("collectConversationFileIds", () => {
  it("collects attachment and tool file ids, deduped", () => {
    const msgs = [
      {
        attachments: [{ file: { id: " f1 " } }],
        toolCalls: [{ file_id: "f2" }],
      },
      {
        attachments: [{ file: { id: "f1" } }],
        toolCalls: [{ file_id: "f2" }, { file_id: "" }],
      },
    ] as unknown as Parameters<typeof collectConversationFileIds>[0];
    expect(collectConversationFileIds(msgs)).toEqual(["f1", "f2"]);
  });

  it("returns empty for no files", () => {
    const msgs = [
      { attachments: [], toolCalls: [] },
    ] as unknown as Parameters<typeof collectConversationFileIds>[0];
    expect(collectConversationFileIds(msgs)).toEqual([]);
  });
});
