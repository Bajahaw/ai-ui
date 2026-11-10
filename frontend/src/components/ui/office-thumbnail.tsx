import { useEffect, useRef, useState } from "react";
import { FileIcon } from "lucide-react";
import {
  acquireOfficeThumbnail,
  type ThumbnailFile,
} from "@/lib/document-rendering/office-thumbnail";
import { cn } from "@/lib/utils";

export function OfficeThumbnail({
  file,
  alt,
  className,
  iconClassName,
}: {
  file: ThumbnailFile;
  alt: string;
  className?: string;
  iconClassName?: string;
}) {
  const hostRef = useRef<HTMLSpanElement>(null);
  const [visible, setVisible] = useState(false);
  const [src, setSrc] = useState("");
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(
      ([entry]) => setVisible(entry.isIntersecting),
      { rootMargin: "100px" },
    );
    observer.observe(hostRef.current!);
    return () => observer.disconnect();
  }, []);

  const { id, name, type, path } = file;
  useEffect(() => {
    if (!visible || src || failed) return;
    let active = true;
    const job = acquireOfficeThumbnail({ id, name, type, path });
    void job.promise
      .then((url) => {
        if (active) setSrc(url);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
      job.release();
    };
  }, [visible, id, name, type, path, src, failed]);

  return (
    <span ref={hostRef} className="flex size-full items-center justify-center">
      {src && !failed ? (
        <img
          src={src}
          alt={alt}
          className={className}
          decoding="async"
          onError={() => setFailed(true)}
        />
      ) : (
        <FileIcon
          className={cn("h-8 w-8 text-muted-foreground/50", iconClassName)}
        />
      )}
    </span>
  );
}
