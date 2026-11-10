import { createContext, useContext } from "react";
import type { DocumentPreview } from "@/lib/document-preview";

type PreviewContext = {
  document: DocumentPreview | null;
  openPreview: (document: DocumentPreview) => void;
  closePreview: () => void;
  restoreFocus: () => void;
};

export const DocumentPreviewContext = createContext<PreviewContext | null>(
  null,
);

export function useDocumentPreview() {
  return useContext(DocumentPreviewContext);
}
