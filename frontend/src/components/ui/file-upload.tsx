"use client";

import { useState, useRef, useCallback } from "react";
import { PlusIcon, FileIcon, XIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PromptInputButton } from "@/components/ai-elements/prompt-input";
import { cn } from "@/lib/utils";
import {
	uploadFile,
	isImageFile,
	formatFileSize,
	FileUploadError,
} from "@/lib/api/files";
import { Response } from "@/components/ai-elements/response";
import type { File as APIFile } from "@/lib/api/types";

export interface FileUploadProps {
	onFileUploaded: (fileData: APIFile, file: File) => void;
	onError?: (error: string) => void;
	disabled?: boolean;
	className?: string;
	accept?: string;
}

export interface UploadedFile {
	file: File;
	fileData: APIFile;
}

export const FileUpload = ({
	onFileUploaded,
	onError,
	disabled = false,
	className,
	accept = "*/*",
}: FileUploadProps) => {
	const [uploading, setUploading] = useState(false);
	const fileInputRef = useRef<HTMLInputElement>(null);

	const handleFileSelect = useCallback(
		async (file: File) => {
			if (disabled || uploading) return;

			setUploading(true);
			try {
				const fileData = await uploadFile(file);
				onFileUploaded(fileData, file);
			} catch (error) {
				const errorMessage =
					error instanceof FileUploadError
						? error.message
						: "Failed to upload file";
				onError?.(errorMessage);
			} finally {
				setUploading(false);
			}
		},
		[disabled, uploading, onFileUploaded, onError],
	);

	const handleFileInputChange = useCallback(
		(event: React.ChangeEvent<HTMLInputElement>) => {
			const files = event.target.files;
			if (files) {
				// Upload all selected files
				Array.from(files).forEach(file => handleFileSelect(file));
			}
			// Clear the input value so the same files can be selected again
			if (fileInputRef.current) {
				fileInputRef.current.value = "";
			}
		},
		[handleFileSelect],
	);

	const handleButtonClick = useCallback(() => {
		if (disabled || uploading) return;
		fileInputRef.current?.click();
	}, [disabled, uploading]);

	return (
		<>
			<input
				ref={fileInputRef}
				type="file"
				accept={accept}
				multiple
				onChange={handleFileInputChange}
				className="hidden"
				disabled={disabled || uploading}
			/>
			<PromptInputButton
				variant="ghost"
				onClick={handleButtonClick}
				disabled={disabled || uploading}
				title="Attach file"
				className={className}
			>
				<PlusIcon size={16} />
			</PromptInputButton>
		</>
	);
};

export interface FilePreviewProps {
	file: UploadedFile;
	onRemove?: () => void;
	className?: string;
}

export const FilePreview = ({
	file,
	onRemove,
	className,
}: FilePreviewProps) => {
	const isImage = isImageFile(file.file.name);

	return (
		<div
			className={cn(
				"flex items-center gap-2 p-2 bg-muted/50 rounded-lg border",
				"max-w-xs",
				className,
			)}
		>
			<div className="flex-shrink-0">
				{isImage ? (
					<div className="size-10 rounded overflow-hidden border bg-background">
						<img
							src={file.fileData.url}
							alt={file.file.name}
							className="size-full object-cover"
						/>
					</div>
				) : (
					<FileIcon className="size-4 text-gray-500" />
				)}
			</div>

			<div className="flex-1 min-w-0">
				<div className="text-sm font-medium truncate">{file.file.name}</div>
				<div className="text-xs text-muted-foreground">
					<div className="flex items-center gap-2">
						<span>{formatFileSize(file.fileData.size)}</span>
						<span className="text-[11px]">
							{new Date(file.fileData.uploadedAt || file.fileData.createdAt).toLocaleString()}
						</span>
					</div>
				</div>
			</div>

			{onRemove && (
				<Button
					type="button"
					variant="ghost"
					size="icon"
					className="size-6 hover:bg-destructive/10 hover:text-destructive"
					onClick={onRemove}
				>
					<XIcon className="size-3" />
				</Button>
			)}
		</div>
	);
};

export interface FilesListProps {
	files: UploadedFile[];
	onRemove?: (index: number) => void;
	className?: string;
}

export const FilesList = ({
	files,
	onRemove,
	className,
}: FilesListProps) => {
	if (files.length === 0) return null;

	return (
		<div className={cn("flex flex-wrap gap-2", className)}>
			{files.map((file, index) => (
				<FilePreview
					key={file.fileData.id}
					file={file}
					onRemove={onRemove ? () => onRemove(index) : undefined}
				/>
			))}
		</div>
	);
};

export interface AttachmentMessageProps {
	attachments?: import("@/lib/api/types").Attachment[];
	// Legacy support for single attachment
	attachment?: string;
	filename?: string;
	className?: string;
}

export const AttachmentMessage = ({
	attachments,
	attachment,
	filename,
	className,
}: AttachmentMessageProps) => {
	// Handle legacy single attachment
	if (attachment && !attachments) {
		const isImage = filename
			? isImageFile(filename)
			: attachment.match(/\.(jpg|jpeg|png|gif|webp|svg|bmp)$/i);

		const markdownContent = isImage
			? `![${filename || "Attached image"}](${attachment})`
			: `[${filename || attachment.split("/").pop() || "Download file"}](${attachment})`;

		return (
			<div className={cn("max-w-full", className)}>
				<Response>{markdownContent}</Response>
			</div>
		);
	}

	// Handle multiple attachments
	if (!attachments || attachments.length === 0) return null;

	const markdownContent = attachments
		.map((att) => {
			const isImage = att.file.type.startsWith("image/");
			const filename = att.file.url.split("/").pop() || "file";

			return isImage
				? `![${filename}](${att.file.url})`
				: `[${filename}](${att.file.url})`;
		})
		.join("\n\n");

	return (
		<div className={cn("max-w-full", className)}>
			<Response>{markdownContent}</Response>
		</div>
	);
};
