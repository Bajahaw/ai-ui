import { describe, expect, it } from "vitest";
import { collectConversationFileIds } from "./runner";
import { fileAliases } from "./wrap";

describe("fileAliases", () => {
  it("exposes name, basename, id and id+ext", () => {
    expect(fileAliases("dir/report.xlsx", "abc")).toEqual([
      "dir/report.xlsx",
      "report.xlsx",
      "abc",
      "abc.xlsx",
    ]);
  });

  it("does not duplicate an id that already carries the extension", () => {
    expect(fileAliases("a.txt", "id.txt")).toEqual(["a.txt", "id.txt"]);
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
