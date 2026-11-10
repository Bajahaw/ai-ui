import { DownloadIcon, FileIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { DocumentPreview } from "@/lib/document-preview";

export function UnsupportedViewer({ document }: { document: DocumentPreview }) {
  return (
    <div
      className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 overflow-auto p-8 text-center"
      role="status"
    >
      <FileIcon className="size-8 text-muted-foreground" aria-hidden="true" />
      <p className="max-w-sm text-sm text-muted-foreground">
        Preview isn’t available for this file type yet.
      </p>
      <Button variant="outline" size="sm" asChild>
        <a
          href={document.href}
          download={document.downloadFilename ?? true}
          target="_blank"
          rel="noopener noreferrer"
        >
          <DownloadIcon />
          Download file
        </a>
      </Button>
    </div>
  );
}
