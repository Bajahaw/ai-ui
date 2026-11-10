import { describe, expect, it } from "vitest";
import { fileLinkName, getPreviewKind } from "./document-preview";

describe("document preview detection", () => {
  it.each([
    "/data/resources/report.pdf",
    "/data/resources/REPORT.PDF?download=true#page=2",
    "https://example.com/report%2Epdf?signature=example",
  ])("recognizes PDF paths: %s", (href) => {
    expect(getPreviewKind(href)).toBe("pdf");
  });

  it.each([
    "/data/resources/report.docx",
    "/download?name=report.pdf",
    "https://example.com/report.pdf.exe",
    "javascript:report.pdf",
    "data:application/pdf,report.pdf",
    "https://example.com/%invalid.pdf",
  ])("leaves unsupported or unsafe links alone: %s", (href) => {
    expect(getPreviewKind(href)).toBeNull();
  });

  it("extracts a decoded filename without the query or hash", () => {
    expect(
      fileLinkName("/data/resources/Annual%20report.pdf?download=1#page=1"),
    ).toBe("Annual report.pdf");
  });
});
