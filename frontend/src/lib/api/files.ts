import { FileUploadResponse } from "./types";
import { getApiUrl } from "../config";

export class FileUploadError extends Error {
  constructor(
    message: string,
    public status?: number,
  ) {
    super(message);
    this.name = "FileUploadError";
  }
}

export const uploadFile = async (file: File): Promise<string> => {
  if (!file) {
    throw new FileUploadError("No file provided");
  }

  // Check file size (10MB limit as per backend)
  const maxSize = 10 * 1024 * 1024; // 10MB
  if (file.size > maxSize) {
    throw new FileUploadError("File size exceeds 10MB limit");
  }

  const formData = new FormData();
  formData.append("file", file);

  try {
    const response = await fetch(getApiUrl("/api/files/upload"), {
      method: "POST",
      credentials: "include",
      body: formData,
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new FileUploadError(`Upload failed: ${errorText}`, response.status);
    }

    const result: FileUploadResponse = await response.json();
    return result.fileUrl;
  } catch (error) {
    if (error instanceof FileUploadError) {
      throw error;
    }
    throw new FileUploadError(
      `Network error during upload: ${error instanceof Error ? error.message : "Unknown error"}`,
    );
  }
};

export const getFileExtension = (filename: string): string => {
  return filename.split(".").pop()?.toLowerCase() || "";
};

export const isImageFile = (filename: string): boolean => {
  const imageExtensions = ["jpg", "jpeg", "png", "gif", "webp", "svg", "bmp"];
  const extension = getFileExtension(filename);
  return imageExtensions.includes(extension);
};

export const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return "0 Bytes";

  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
};
