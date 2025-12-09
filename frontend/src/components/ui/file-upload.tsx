"use client";

import { useState, useRef, useCallback } from "react";
import { PlusIcon, FileIcon, ImageIcon, XIcon } from "lucide-react";
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

export interface FileUploadProps {
	onFileUploaded: (fileUrl: string, file: File) => void;
	onError?: (error: string) => void;
	disabled?: boolean;
	className?: string;
	accept?: string;
}

export interface UploadedFile {
	file: File;
	url: string;
	preview?: string;
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
				const fileUrl = await uploadFile(file);
				onFileUploaded(fileUrl, file);
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
			const file = event.target.files?.[0];
			if (file) {
				handleFileSelect(file);
			}
			// Clear the input value so the same file can be selected again
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
					<ImageIcon className="size-4 text-blue-500" />
				) : (
					<FileIcon className="size-4 text-gray-500" />
				)}
			</div>

			<div className="flex-1 min-w-0">
				<div className="text-sm font-medium truncate">{file.file.name}</div>
				<div className="text-xs text-muted-foreground">
					{formatFileSize(file.file.size)}
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

export interface AttachmentMessageProps {
	attachment: string;
	filename?: string;
	className?: string;
}

export const AttachmentMessage = ({
	attachment,
	filename,
	className,
}: AttachmentMessageProps) => {
	const isImage = filename
		? isImageFile(filename)
		: attachment.match(/\.(jpg|jpeg|png|gif|webp|svg|bmp)$/i);

	// Create markdown content based on file type
	const markdownContent = isImage
		? `![${filename || "Attached image"}](${attachment})`
		: `[${filename || attachment.split("/").pop() || "Download file"}](${attachment})`;

	// Render markdown using the Response component
	return (
		<div className={cn("max-w-full", className)}>
			<Response>{markdownContent}</Response>
		</div>
	);
};
