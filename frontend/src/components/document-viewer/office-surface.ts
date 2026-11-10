import type { DocumentPreview } from "@/lib/document-preview";

export interface OfficeSurface {
  load(source: ArrayBuffer): Promise<void>;
  destroy(): void;
  zoomIn(): void;
  zoomOut(): void;
  fitWidth(): void;
}

export async function createOfficeSurface(
  host: HTMLElement,
  kind: DocumentPreview["kind"],
  onError: (error: Error) => void,
  onScaleChange: (scale: number) => void,
): Promise<OfficeSurface> {
  const options = {
    mode:
      typeof Worker !== "undefined" && typeof OffscreenCanvas !== "undefined"
        ? ("worker" as const)
        : ("main" as const),
    onError,
    onScaleChange,
    // Previewing untrusted attachments must not navigate to embedded links.
    enableHyperlinks: false,
  };
  const scrollOptions = {
    ...options,
    enableTextSelection: true,
    gap: 16,
    background: "transparent",
  };
  // Keep each format's renderer and WASM out of the initial chat bundle.
  switch (kind) {
    case "docx": {
      const { DocxScrollViewer } = await import("@silurus/ooxml/docx");
      return new DocxScrollViewer(host, scrollOptions);
    }
    case "pptx": {
      const { PptxScrollViewer } = await import("@silurus/ooxml/pptx");
      return new PptxScrollViewer(host, scrollOptions);
    }
    case "xlsx": {
      const { XlsxViewer } = await import("@silurus/ooxml/xlsx");
      return new XlsxViewer(host, { ...options, showZoomSlider: false });
    }
    default:
      throw new Error("Unsupported Office format");
  }
}
