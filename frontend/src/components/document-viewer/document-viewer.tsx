import {
  lazy,
  Suspense,
  useEffect,
  useRef,
  useSyncExternalStore,
  type ComponentType,
} from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { DownloadIcon, FileTextIcon, Loader2Icon, XIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useDocumentPreview } from "@/hooks/useDocumentPreview";
import type { DocumentPreview } from "@/lib/document-preview";
import "./document-viewer.css";

const renderers = {
  pdf: lazy(() => import("./pdf-viewer")),
} satisfies Record<
  DocumentPreview["kind"],
  ComponentType<{ document: DocumentPreview }>
>;
const desktopQuery = "(min-width: 1024px)";
function subscribeToViewport(callback: () => void) {
  const media = window.matchMedia(desktopQuery);
  media.addEventListener("change", callback);
  return () => media.removeEventListener("change", callback);
}
function isDesktop() {
  return window.matchMedia(desktopQuery).matches;
}

function ViewerContents({
  document,
  mobile,
}: {
  document: DocumentPreview;
  mobile: boolean;
}) {
  const preview = useDocumentPreview()!;
  const Renderer = renderers[document.kind];
  const closeRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!mobile) closeRef.current?.focus({ preventScroll: true });
  }, [mobile]);

  return (
    <>
      {mobile && (
        <div
          className="mx-auto mt-2 h-1 w-9 shrink-0 rounded-full bg-muted-foreground/30"
          aria-hidden="true"
        />
      )}
      <header className="flex min-h-14 shrink-0 items-center gap-2 border-b px-3">
        <FileTextIcon
          className="size-4 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
        {mobile ? (
          <Dialog.Title
            className="min-w-0 flex-1 truncate text-sm font-medium"
            title={document.filename}
          >
            {document.filename}
          </Dialog.Title>
        ) : (
          <h2
            id="document-viewer-title"
            className="min-w-0 flex-1 truncate text-sm font-medium"
            title={document.filename}
          >
            {document.filename}
          </h2>
        )}
        <span className="text-xs text-muted-foreground">PDF</span>
        <Button variant="ghost" size="icon" asChild>
          <a
            href={document.href}
            download
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`Download ${document.filename}`}
            title="Download"
          >
            <DownloadIcon />
          </a>
        </Button>
        <Button
          ref={closeRef}
          variant="ghost"
          size="icon"
          onClick={preview.closePreview}
          aria-label="Close document preview"
          title="Close preview (Esc)"
        >
          <XIcon />
        </Button>
      </header>
      <Suspense
        fallback={
          <div
            className="flex flex-1 items-center justify-center gap-2 text-sm text-muted-foreground"
            role="status"
          >
            <Loader2Icon className="size-4 animate-spin" />
            Loading viewer…
          </div>
        }
      >
        <Renderer key={document.href} document={document} />
      </Suspense>
    </>
  );
}

export function DocumentViewer() {
  const preview = useDocumentPreview();
  const desktop = useSyncExternalStore(
    subscribeToViewport,
    isDesktop,
    () => false,
  );
  const document = preview?.document;
  const closePreview = preview?.closePreview;

  useEffect(() => {
    if (!document || !desktop) return;
    const onKeyDown = (event: KeyboardEvent) => {
      // Let menus and other dialogs handle their own Escape first.
      if (
        event.key === "Escape" &&
        !event.defaultPrevented &&
        !window.document.querySelector('[role="dialog"]')
      ) {
        event.preventDefault();
        closePreview?.();
      }
    };
    window.document.addEventListener("keydown", onKeyDown);
    return () => window.document.removeEventListener("keydown", onKeyDown);
  }, [document, desktop, closePreview]);

  if (!document || !preview) return null;

  if (desktop) {
    return (
      <aside
        className="document-viewer-panel flex min-h-0 shrink-0 flex-col border-l bg-background"
        aria-labelledby="document-viewer-title"
      >
        <ViewerContents document={document} mobile={false} />
      </aside>
    );
  }

  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open) preview.closePreview();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="document-viewer-backdrop fixed inset-0 z-50 bg-black/50" />
        <Dialog.Content
          className="document-viewer-sheet fixed inset-x-0 bottom-0 z-50 flex min-h-0 flex-col overflow-hidden rounded-t-3xl border bg-background shadow-xl outline-none"
          aria-describedby={undefined}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            preview.restoreFocus();
          }}
          onTouchStart={(event) => event.stopPropagation()}
          onTouchEnd={(event) => event.stopPropagation()}
        >
          <ViewerContents document={document} mobile />
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
