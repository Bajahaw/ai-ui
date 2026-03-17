import { useState } from "react";

export function downloadTextFile(content: string, mimeType: string, filename: string) {
  if (!content) {
    return;
  }

  const blob = new Blob([content], { type: mimeType });
  const downloadUrl = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = downloadUrl;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(downloadUrl);
}

export function useCopyFeedback(timeoutMs = 1800) {
  const [isCopying, setIsCopying] = useState(false);
  const [isCopied, setIsCopied] = useState(false);

  const copyText = async (text: string) => {
    if (!text || isCopying || typeof window === "undefined" || !navigator.clipboard?.writeText) {
      return false;
    }

    setIsCopying(true);
    try {
      await navigator.clipboard.writeText(text);
      setIsCopied(true);
      setTimeout(() => setIsCopied(false), timeoutMs);
      return true;
    } catch {
      return false;
    } finally {
      setIsCopying(false);
    }
  };

  return {
    isCopying,
    isCopied,
    copyText,
  };
}
