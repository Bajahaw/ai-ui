import { useState } from "react";
import { FileIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface FileThumbProps {
  src: string;
  alt: string;
  className?: string;
  iconClassName?: string;
}

/** Image thumb with icon fallback when load/generation fails. */
export function FileThumb({
  src,
  alt,
  className,
  iconClassName,
}: FileThumbProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null);
  const failed = !src || failedSrc === src;

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
