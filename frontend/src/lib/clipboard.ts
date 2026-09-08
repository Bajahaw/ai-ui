function addFile(files: File[], file: File | null | undefined) {
  if (!file || file.size === 0) return;
  files.push(file);
}

export function filesFromClipboard(
  data: DataTransfer | null | undefined,
): File[] {
  if (!data) return [];

  const fromFiles: File[] = [];
  const fileList = data.files;
  if (fileList) {
    for (let i = 0; i < fileList.length; i++) {
      addFile(fromFiles, fileList.item(i) ?? fileList[i]);
    }
  }
  if (fromFiles.length > 0) return fromFiles;

  const fromItems: File[] = [];
  const items = data.items;
  if (items) {
    for (let i = 0; i < items.length; i++) {
      const item = items[i];
      if (!item) continue;
      if (item.kind !== "file" && !item.type.startsWith("image/")) continue;
      addFile(fromItems, item.getAsFile());
    }
  }
  return fromItems;
}

export async function readClipboardImageFiles(): Promise<File[]> {
  if (typeof navigator === "undefined" || !navigator.clipboard?.read) {
    return [];
  }

  try {
    const items = await navigator.clipboard.read();
    const files: File[] = [];
    for (const item of items) {
      for (const type of item.types) {
        if (!type.startsWith("image/")) continue;
        const blob = await item.getType(type);
        const ext = type.split("/")[1]?.split("+")[0] || "png";
        files.push(
          new File([blob], `pasted-image.${ext}`, {
            type: blob.type || type,
          }),
        );
      }
    }
    return files;
  } catch {
    return [];
  }
}

export async function filesFromImageElements(
  images: ArrayLike<HTMLImageElement>,
): Promise<File[]> {
  const files: File[] = [];
  for (let i = 0; i < images.length; i++) {
    const src = images[i]?.currentSrc || images[i]?.src;
    if (!src || (!src.startsWith("blob:") && !src.startsWith("data:"))) {
      continue;
    }
    try {
      const blob = await (await fetch(src)).blob();
      if (!blob.size) continue;
      const type = blob.type || "image/png";
      const ext = type.split("/")[1]?.split("+")[0] || "png";
      files.push(new File([blob], `pasted-image.${ext}`, { type }));
    } catch {
      continue;
    }
  }
  return files;
}
