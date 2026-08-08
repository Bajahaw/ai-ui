import { FileUploadResponse, File as ApiFile } from "./types";

import { getHeaders } from "./headers";

export class FileUploadError extends Error {
  constructor(
    message: string,
    public status?: number,
  ) {
    super(message);
    this.name = "FileUploadError";
  }
}

export const getFiles = async (): Promise<ApiFile[]> => {
  const response = await fetch("/api/files/all", {
    method: "GET",
    headers: getHeaders(),
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error("Failed to fetch files");
  }

  return response.json();
};

export const uploadFile = async (file: File): Promise<FileUploadResponse> => {
  if (!file) {
    throw new FileUploadError("No file provided");
  }

  // Check file size (50MB limit as per backend)
  const maxSize = 50 * 1024 * 1024; // 50MB
  if (file.size > maxSize) {
    throw new FileUploadError("File size exceeds 50MB limit");
  }

  const formData = new FormData();
  formData.append("file", file);
  formData.append("lastModified", file.lastModified.toString());

  try {
    const response = await fetch("/api/files/upload", {
      headers: getHeaders(),
      method: "POST",
      credentials: "include",
      body: formData,
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new FileUploadError(`Upload failed: ${errorText}`, response.status);
    }

    const result: FileUploadResponse = await response.json();
    return result;
  } catch (error) {
    if (error instanceof FileUploadError) {
      throw error;
    }
    throw new FileUploadError(
      `Network error during upload: ${error instanceof Error ? error.message : "Unknown error"}`,
    );
  }
};

export const deleteFile = async (id: string): Promise<void> => {
  const response = await fetch(`/api/files/delete/${id}`, {
    headers: getHeaders(),
    method: "DELETE",
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error("Failed to delete file");
  }
};

export const extractContent = async (fileIds: string[]): Promise<ApiFile[]> => {
  const response = await fetch(`/api/files/extract-content`, {
    method: "POST",
    headers: {
      ...getHeaders(),
      "Content-Type": "application/json",
    },
    credentials: "include",
    body: JSON.stringify({ fileIds }),
  });

  if (!response.ok) {
    throw new Error("Failed to extract content");
  }

  return response.json();
};

export const getFileExtension = (filename: string): string => {
  return filename.split(".").pop()?.toLowerCase() || "";
};

export const isImageFile = (filename: string): boolean => {
  const imageExtensions = ["jpg", "jpeg", "png", "gif", "webp", "svg", "bmp"];
  const extension = getFileExtension(filename);
  return imageExtensions.includes(extension);
};

/** Types the backend can thumbnail: raster images + PDF first page (not SVG). */
export const isThumbnailable = (filename: string, mimeType?: string): boolean => {
  const mt = (mimeType || "").toLowerCase();
  if (mt === "application/pdf") return true;
  if (mt.startsWith("image/") && mt !== "image/svg+xml") return true;
  const extension = getFileExtension(filename);
  return ["jpg", "jpeg", "png", "gif", "webp", "bmp", "pdf"].includes(extension);
};

export const fileResourceUrl = (filePath: string): string => {
  if (!filePath) return "";
  return filePath.startsWith("/") ? filePath : `/${filePath}`;
};

/**
 * Convention: data/resources/{stem}.ext -> /data/resources/thumbs/{stem}.jpg
 * Backend generates on demand when missing.
 */
export const fileThumbnailUrl = (filePath: string): string => {
  const url = fileResourceUrl(filePath);
  const filename = url.split("/").pop() || "";
  const dot = filename.lastIndexOf(".");
  const stem = dot > 0 ? filename.slice(0, dot) : filename;
  if (!stem) return "";
  return `/data/resources/thumbs/${stem}.jpg`;
};

export const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return "0 Bytes";

  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
};
