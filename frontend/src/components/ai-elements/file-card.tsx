import React from "react";
import {
  DownloadIcon,
  ExternalLinkIcon,
  EyeIcon,
  FileArchiveIcon,
  FileAudioIcon,
  FileIcon,
  FileImageIcon,
  FileSpreadsheetIcon,
  FileTextIcon,
  FileVideoIcon,
  PresentationIcon,
} from "lucide-react";
import { Card } from "@/components/ui/card";
import { useDocumentPreview } from "@/hooks/useDocumentPreview";
import { fileLinkName, getPreviewKind } from "@/lib/document-preview";

function labelText(node: React.ReactNode): string {
  return React.Children.toArray(node)
    .map((child): string => {
      if (typeof child === "string" || typeof child === "number")
        return String(child);
      if (React.isValidElement<{ children?: React.ReactNode }>(child))
        return labelText(child.props.children);
      return "";
    })
    .join("");
}

const fileTypes = [
  {
    extensions: ["XLSX", "XLS", "CSV"],
    Icon: FileSpreadsheetIcon,
    label: "Spreadsheet",
  },
  {
    extensions: ["PPT", "PPTX", "PPS", "PPSX"],
    Icon: PresentationIcon,
    label: "Presentation",
  },
  {
    extensions: ["PNG", "JPG", "JPEG", "GIF", "WEBP", "BMP", "SVG", "HEIC"],
    Icon: FileImageIcon,
    label: "Image",
  },
  {
    extensions: ["MP3", "WAV", "OGG", "M4A", "AAC", "FLAC"],
    Icon: FileAudioIcon,
    label: "Audio",
  },
  {
    extensions: ["MP4", "MOV", "WEBM", "AVI", "MKV", "M4V"],
    Icon: FileVideoIcon,
    label: "Video",
  },
  {
    extensions: ["ZIP", "TAR", "GZ", "RAR", "7Z"],
    Icon: FileArchiveIcon,
    label: "Archive",
  },
  { extensions: ["PDF"], Icon: FileTextIcon, label: "PDF Document" },
  {
    extensions: ["GO", "JS", "TS", "JSON"],
    Icon: FileTextIcon,
    label: "Code Text",
  },
  {
    extensions: [
      "DOC",
      "DOCX",
      "TXT",
      "MD",
      "YAML",
      "YML",
      "XML",
      "HTML",
      "CSS",
      "SCSS",
      "SQL",
      "PY",
      "RB",
      "JAVA",
      "C",
      "CPP",
      "RS",
    ],
    Icon: FileTextIcon,
    label: "Document",
  },
];

function fileAppearance(href: string) {
  const match = fileLinkName(href).match(/\.([a-zA-Z0-9]+)$/);
  const extension = match ? match[1].toUpperCase() : "FILE";
  const type = fileTypes.find((entry) => entry.extensions.includes(extension));
  return {
    extension,
    Icon: type?.Icon ?? FileIcon,
    typeLabel:
      type?.label ?? (extension === "FILE" ? "Unknown File" : "Document"),
  };
}

interface FileCardProps {
  href: string;
  filename: React.ReactNode;
  isExternal?: boolean;
}

export function FileCard({
  href,
  filename,
  isExternal = false,
}: FileCardProps) {
  const preview = useDocumentPreview();
  const kind = getPreviewKind(href);
  const canPreview = !!preview && !!kind;
  const name = labelText(filename) || fileLinkName(href);
  const { extension, Icon, typeLabel } = fileAppearance(href);
  const isSameOrigin =
    !isExternal &&
    new URL(href, window.location.origin).origin === window.location.origin;

  return (
    <div className="group relative my-4 max-w-full">
      <a
        href={href}
        {...(isSameOrigin
          ? { download: true }
          : { target: "_blank", rel: "noopener noreferrer" })}
        className="block rounded-xl no-underline outline-none focus-visible:ring-2 focus-visible:ring-ring"
        aria-label={canPreview ? `Preview ${name}` : undefined}
        aria-haspopup={
          canPreview && window.innerWidth < 1024 ? "dialog" : undefined
        }
        onClick={(event) => {
          if (
            !preview ||
            !kind ||
            event.button !== 0 ||
            event.metaKey ||
            event.ctrlKey ||
            event.shiftKey ||
            event.altKey
          )
            return;
          event.preventDefault();
          event.currentTarget.focus({ preventScroll: true });
          preview.openPreview({ href, filename: name, kind });
        }}
      >
        <Card className="flex items-center justify-between p-4 rounded-xl transition-colors !duration-100 hover:bg-secondary/20">
          <div className="flex items-center gap-4 overflow-hidden">
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-secondary text-secondary-foreground">
              <Icon className="h-6 w-6 text-muted-foreground" />
            </div>
            <div className="flex flex-col overflow-hidden">
              <div className="flex items-center gap-2">
                <span className="truncate text-sm text-foreground">
                  {filename}
                </span>
                {isExternal && <ExternalLinkIcon className="h-3 w-3" />}
              </div>
              <span className="truncate text-xs text-muted-foreground">
                {typeLabel} • {extension}
              </span>
            </div>
          </div>
          <div className="mx-4 shrink-0">
            {canPreview ? (
              <EyeIcon className="mr-8 h-4 w-4 text-muted-foreground" />
            ) : (
              <DownloadIcon className="h-4 w-4 text-muted-foreground" />
            )}
          </div>
        </Card>
      </a>
      {canPreview && (
        <a
          href={href}
          {...(isSameOrigin
            ? { download: true }
            : { target: "_blank", rel: "noopener noreferrer" })}
          className="absolute right-3 top-1/2 flex size-9 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={`Download ${name}`}
          title="Download"
        >
          <DownloadIcon className="size-4" />
        </a>
      )}
    </div>
  );
}
