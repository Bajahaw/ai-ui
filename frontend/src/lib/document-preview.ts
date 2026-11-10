/** Add new renderer kinds here as more document formats become previewable. */
export type DocumentPreview = {
  href: string;
  filename: string;
  kind: "pdf";
};

export function fileLinkName(href: string): string {
  try {
    const pathname = new URL(href, "http://localhost").pathname;
    return decodeURIComponent(pathname.split("/").pop() || "Document");
  } catch {
    return "Document";
  }
}

export function getPreviewKind(href: string): DocumentPreview["kind"] | null {
  try {
    const url = new URL(href, "http://localhost");
    if (!["http:", "https:"].includes(url.protocol)) return null;
    return /\.pdf$/i.test(decodeURIComponent(url.pathname)) ? "pdf" : null;
  } catch {
    return null;
  }
}
