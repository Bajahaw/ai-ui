import { useCallback, useMemo, useRef, useState, type ReactNode } from "react";
import type { DocumentPreview } from "@/lib/document-preview";
import { DocumentPreviewContext } from "@/hooks/useDocumentPreview";

export function DocumentPreviewProvider({
  scopeKey,
  children,
}: {
  scopeKey: string;
  children?: ReactNode;
}) {
  const [document, setDocument] = useState<DocumentPreview | null>(null);
  const [scope, setScope] = useState(scopeKey);
  const triggerRef = useRef<HTMLElement | null>(null);

  // Reset on conversation navigation without remounting the chat or sidebar.
  if (scope !== scopeKey) {
    setScope(scopeKey);
    setDocument(null);
  }

  const openPreview = useCallback((next: DocumentPreview) => {
    triggerRef.current =
      window.document.activeElement instanceof HTMLElement
        ? window.document.activeElement
        : null;
    setDocument(next);
  }, []);
  const restoreFocus = useCallback(() => {
    if (triggerRef.current?.isConnected)
      triggerRef.current.focus({ preventScroll: true });
  }, []);
  const closePreview = useCallback(() => {
    setDocument(null);
    restoreFocus();
  }, [restoreFocus]);
  const value = useMemo(
    () => ({ document, openPreview, closePreview, restoreFocus }),
    [document, openPreview, closePreview, restoreFocus],
  );

  return (
    <DocumentPreviewContext.Provider value={value}>
      {children}
    </DocumentPreviewContext.Provider>
  );
}
