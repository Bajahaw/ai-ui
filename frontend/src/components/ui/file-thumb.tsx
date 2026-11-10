import { useState } from "react";
import { FileIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { getPreviewKind } from "@/lib/document-preview";
import type { ThumbnailFile } from "@/lib/document-rendering/office-thumbnail";
import { OfficeThumbnail } from "./office-thumbnail";

interface FileThumbProps {
  src: string;
  alt: string;
  className?: string;
  iconClassName?: string;
  file?: ThumbnailFile;
}

/** Image thumb with icon fallback when load/generation fails. */
export function FileThumb({
  src,
  alt,
  className,
  iconClassName,
  file,
}: FileThumbProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null);
  const failed = !src || failedSrc === src;

  const kind =
    file &&
    getPreviewKind("/thumbnail", { filename: file.name, mimeType: file.type });
  if (file && (kind === "docx" || kind === "xlsx" || kind === "pptx"))
    return (
      <OfficeThumbnail
        key={`${file.id}:${file.path}`}
        file={file}
        alt={alt}
        className={className}
        iconClassName={iconClassName}
      />
    );

  if (failed) {
    return (
      <FileIcon
        className={cn("h-8 w-8 text-muted-foreground/50", iconClassName)}
      />
    );
  }

  return (
    <img
      src={src}
      alt={alt}
      className={className}
      loading="lazy"
      decoding="async"
      onError={() => setFailedSrc(src)}
    />
  );
}
