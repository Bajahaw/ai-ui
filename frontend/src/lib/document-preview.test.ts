import { describe, expect, it } from "vitest";
import {
  fileLinkName,
  getPreviewKind,
  getPreviewLabel,
} from "./document-preview";

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

  it.each([
    "png",
    "jpg",
    "jpeg",
    "gif",
    "webp",
    "avif",
    "bmp",
    "svg",
    "ico",
    "apng",
  ])("recognizes browser image formats: %s", (extension) => {
    expect(
      getPreviewKind(`/data/resources/photo.${extension}?download=1`),
    ).toBe("image");
  });

  it("uses upload MIME types and original filenames for opaque resource URLs", () => {
    expect(
      getPreviewKind("/data/resources/upload-123", {
        mimeType: "IMAGE/PNG; charset=binary",
      }),
    ).toBe("image");
    expect(
      getPreviewKind("/data/resources/upload-123", {
        mimeType: "application/pdf",
      }),
    ).toBe("pdf");
    expect(
      getPreviewKind("/data/resources/upload-123", {
        filename: "photo.JPG",
        mimeType: "application/octet-stream",
      }),
    ).toBe("image");
    expect(
      getPreviewKind("/data/resources/upload-123", { filename: "report.pdf" }),
    ).toBe("pdf");
  });

  it("does not attempt unsupported images or execute active file types", () => {
    expect(
      getPreviewKind("/data/resources/photo.heic", { mimeType: "image/heic" }),
    ).toBeNull();
    expect(
      getPreviewKind("/data/resources/photo.tiff", { mimeType: "image/tiff" }),
    ).toBeNull();
    expect(
      getPreviewKind("/data/resources/misleading.png", {
        mimeType: "text/html",
      }),
    ).toBeNull();
    expect(
      getPreviewKind("javascript:alert(1)", { mimeType: "image/png" }),
    ).toBeNull();
  });

  it("labels every renderer without calling non-PDF attachments PDFs", () => {
    expect(
      getPreviewLabel({
        href: "/opaque",
        filename: "Photo.png",
        kind: "image",
      }),
    ).toBe("PNG");
    expect(
      getPreviewLabel({ href: "/opaque", filename: "Photo", kind: "image" }),
    ).toBe("Image");
    expect(
      getPreviewLabel({
        href: "/opaque",
        filename: "Notes.docx",
        kind: "unsupported",
      }),
    ).toBe("DOCX");
    expect(
      getPreviewLabel({ href: "/opaque", filename: "Report", kind: "pdf" }),
    ).toBe("PDF");
  });
});
