export type OfficePreviewKind = "docx" | "xlsx" | "pptx";

/** Add new renderer kinds here as more document formats become previewable. */
export type DocumentPreview = {
  href: string;
  filename: string;
  /** Original upload name, separate from a generated link's display label. */
  downloadFilename?: string;
  kind: "pdf" | "image" | OfficePreviewKind | "unsupported";
};

const imageExtensions = /\.(png|jpe?g|gif|webp|avif|bmp|svg|ico|apng)$/i;
const officeMimeTypes: Record<string, OfficePreviewKind> = {
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
    "docx",
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": "xlsx",
  "application/vnd.openxmlformats-officedocument.presentationml.presentation":
    "pptx",
};
const imageMimeTypes = new Set([
  "image/png",
  "image/jpeg",
  "image/jpg",
  "image/pjpeg",
  "image/gif",
  "image/webp",
  "image/avif",
  "image/bmp",
  "image/x-ms-bmp",
  "image/svg+xml",
  "image/x-icon",
  "image/vnd.microsoft.icon",
  "image/apng",
]);

export function fileLinkName(href: string): string {
  try {
    const pathname = new URL(href, "http://localhost").pathname;
    return decodeURIComponent(pathname.split("/").pop() || "Document");
  } catch {
    return "Document";
  }
}

export function getPreviewKind(
  href: string,
  metadata: { filename?: string; mimeType?: string } = {},
): Exclude<DocumentPreview["kind"], "unsupported"> | null {
  try {
    const url = new URL(href, "http://localhost");
    if (!["http:", "https:"].includes(url.protocol)) return null;
    const mimeType = metadata.mimeType?.split(";")[0].trim().toLowerCase();
    if (mimeType === "application/pdf") return "pdf";
    if (mimeType && imageMimeTypes.has(mimeType)) return "image";
    if (mimeType && Object.hasOwn(officeMimeTypes, mimeType))
      return officeMimeTypes[mimeType];
    // Specific MIME types take precedence over misleading extensions. Generic
    // upload MIME types still allow detection using the original filename.
    if (
      mimeType &&
      ![
        "application/octet-stream",
        "binary/octet-stream",
        "image/unknown",
      ].includes(mimeType)
    )
      return null;
    const name = metadata.filename || decodeURIComponent(url.pathname);
    if (/\.pdf$/i.test(name)) return "pdf";
    const officeExtension = name.match(/\.(docx|xlsx|pptx)$/i)?.[1];
    if (officeExtension)
      return officeExtension.toLowerCase() as OfficePreviewKind;
    return imageExtensions.test(name) ? "image" : null;
  } catch {
    return null;
  }
}

export function getPreviewLabel(document: DocumentPreview): string {
  if (document.kind === "pdf") return "PDF";
  if (["docx", "xlsx", "pptx"].includes(document.kind))
    return document.kind.toUpperCase();
  const extension = document.filename.match(/\.([a-z0-9]{1,10})$/i)?.[1];
  return (
    extension?.toUpperCase() ?? (document.kind === "image" ? "Image" : "File")
  );
}
