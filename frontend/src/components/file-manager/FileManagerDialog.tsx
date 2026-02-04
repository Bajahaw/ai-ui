import { useState, useEffect, useCallback, useRef } from "react";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Input } from "@/components/ui/input";
import { Loader2, FileIcon, Upload, Search, Check, Paperclip, Trash2, ScanText } from "lucide-react";
import { getFiles, uploadFile, formatFileSize, isImageFile, deleteFile } from "@/lib/api/files";
import { File as ApiFile } from "@/lib/api/types";
import { cn } from "@/lib/utils";

interface FileManagerDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	onAttach: (files: ApiFile[]) => void;
}

export function FileManagerDialog({
	open,
	onOpenChange,
	onAttach,
}: FileManagerDialogProps) {
	const [files, setFiles] = useState<ApiFile[]>([]);
	const [loading, setLoading] = useState(false);
	const [uploading, setUploading] = useState(false);
	const [deleting, setDeleting] = useState(false);
	const [selectedFileIds, setSelectedFileIds] = useState<Set<string>>(new Set());
	const [searchQuery, setSearchQuery] = useState("");
	const [isDragging, setIsDragging] = useState(false);
	const fileInputRef = useRef<HTMLInputElement>(null);

	const fetchFiles = useCallback(async () => {
		setLoading(true);
		try {
			const data = await getFiles();
			// Sort by uploadedAt (if provided) else createdAt, desc
			const sorted = data.sort((a, b) =>
				new Date(b.uploadedAt || b.createdAt).getTime() - new Date(a.uploadedAt || a.createdAt).getTime()
			);
			setFiles(sorted);
		} catch (error) {
			console.error("Failed to fetch files:", error);
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		if (open) {
			fetchFiles();
			setSelectedFileIds(new Set());
			setSearchQuery("");
		}
	}, [open, fetchFiles]);

	const handleFileSelect = (fileId: string) => {
		const newSelected = new Set(selectedFileIds);
		if (newSelected.has(fileId)) {
			newSelected.delete(fileId);
		} else {
			newSelected.add(fileId);
		}
		setSelectedFileIds(newSelected);
	};

	const handleUploadFiles = async (fileList: FileList | File[]) => {
		if (!fileList || fileList.length === 0) return;

		const filesToUpload: File[] = [];
		const existingIdsToSelect: string[] = [];

		Array.from(fileList).forEach(file => {
			// Check for duplicates: name, size, and date (within 1s tolerance for backend truncation)
			const existingFile = files.find(f => 
				f.name === file.name && 
				f.size === file.size && 
				Math.abs(new Date(f.createdAt).getTime() - file.lastModified) < 1000
			);

			if (existingFile) {
				existingIdsToSelect.push(existingFile.id);
			} else {
				filesToUpload.push(file);
			}
		});

		// Select existing files immediately
		if (existingIdsToSelect.length > 0) {
			setSelectedFileIds(prev => {
				const newSelected = new Set(prev);
				existingIdsToSelect.forEach(id => newSelected.add(id));
				return newSelected;
			});
		}

		// If no new files to upload, we're done
		if (filesToUpload.length === 0) {
			if (fileInputRef.current) {
				fileInputRef.current.value = "";
			}
			return;
		}

		setUploading(true);
		try {
			const uploadPromises = filesToUpload.map(file => uploadFile(file));
			const uploadedFiles = await Promise.all(uploadPromises);
			// Ensure newly-uploaded files have an `uploadedAt` timestamp so the UI
			// can sort/display consistently. If backend provided one, keep it.
			const uploadedWithTs = uploadedFiles.map(f => ({
				...f,
				uploadedAt: (f as any).uploadedAt || new Date().toISOString(),
			} as ApiFile));
			// Add new files to the list and select them
			setFiles(prev => [...uploadedWithTs, ...prev]);
			
			setSelectedFileIds(prev => {
				const newSelected = new Set(prev);
				uploadedFiles.forEach(f => newSelected.add(f.id));
				return newSelected;
			});
			
		} catch (error) {
			console.error("Upload failed:", error);
			// TODO: Show error toast
		} finally {
			setUploading(false);
			if (fileInputRef.current) {
				fileInputRef.current.value = "";
			}
		}
	};

	const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
		if (e.target.files) {
			handleUploadFiles(e.target.files);
		}
	};

	const handleDragOver = (e: React.DragEvent) => {
		e.preventDefault();
		setIsDragging(true);
	};

	const handleDragLeave = (e: React.DragEvent) => {
		e.preventDefault();
		setIsDragging(false);
	};

	const handleDrop = (e: React.DragEvent) => {
		e.preventDefault();
		setIsDragging(false);
		if (e.dataTransfer.files) {
			handleUploadFiles(e.dataTransfer.files);
		}
	};

	const handlePaste = (e: React.ClipboardEvent) => {
		if (e.clipboardData.files && e.clipboardData.files.length > 0) {
			handleUploadFiles(e.clipboardData.files);
		}
	};

	const handleAttach = () => {
		const selectedFiles = files.filter(f => selectedFileIds.has(f.id));
		onAttach(selectedFiles);
		onOpenChange(false);
	};

	const handleDelete = async () => {
		if (selectedFileIds.size === 0) return;
		
		setDeleting(true);
		try {
			const idsToDelete = Array.from(selectedFileIds);
			for (const id of idsToDelete) {
				await deleteFile(id);
			}
			
			setFiles(prev => prev.filter(f => !selectedFileIds.has(f.id)));
			setSelectedFileIds(new Set());
		} catch (error) {
			console.error("Failed to delete files:", error);
			// TODO: Show error toast
		} finally {
			setDeleting(false);
		}
	};

	const handleOCR = async () => {
		if (selectedFileIds.size === 0) return;
		// Backend endpoint to be implemented later
		console.log("Trigger OCR for:", Array.from(selectedFileIds));
	};

	const filteredFiles = files.filter(f => 
		f.name.toLowerCase().includes(searchQuery.toLowerCase())
	);

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent 
				className="w-[calc(100%-1rem)] sm:w-[calc(100%-3rem)] max-w-7xl sm:max-w-4xl h-[65vh] p-0 flex flex-col rounded-xl"
				onPaste={handlePaste}
				onOpenAutoFocus={(e) => e.preventDefault()}
			>
				<DialogHeader className="px-6 py-4 border-b flex-shrink-0">
					<DialogTitle className="flex items-center gap-2">
						<Paperclip className="h-5 w-5" />
						File Library
					</DialogTitle>
				</DialogHeader>

				<div className="p-4 flex items-center gap-2 flex-shrink-0">
					<div className="relative flex-1">
						<Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
						<Input
							placeholder="Search files..."
							value={searchQuery}
							onChange={(e) => setSearchQuery(e.target.value)}
							className="pl-9 bg-background focus-visible:ring-1"
						/>
					</div>
					<div className="flex items-center gap-2">
						<Button
							variant="outline"
							size="icon"
							onClick={handleOCR}
							disabled={selectedFileIds.size === 0}
							title="Trigger OCR"
							className="h-9 w-9"
						>
							<ScanText className="h-4 w-4" />
						</Button>
						<Button
							variant="outline"
							size="icon"
							onClick={handleDelete}
							disabled={selectedFileIds.size === 0 || deleting}
							title="Delete files"
							className="h-9 w-9 text-destructive hover:text-destructive hover:bg-destructive/5"
						>
							{deleting ? (
								<Loader2 className="h-4 w-4 animate-spin" />
							) : (
								<Trash2 className="h-4 w-4" />
							)}
						</Button>
					</div>
				</div>

				<ScrollArea className="flex-1 px-4 min-h-0">
					{loading ? (
						<div className="flex items-center justify-center h-full min-h-[200px]">
							<Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
						</div>
					) : filteredFiles.length === 0 ? (
						<div className="flex flex-col items-center justify-center h-full min-h-[200px] text-muted-foreground gap-2">
							<FileIcon className="h-12 w-12 opacity-20" />
							<p>No files found</p>
						</div>
					) : (
						<div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 gap-4">
							{filteredFiles.map((file) => {
								const isSelected = selectedFileIds.has(file.id);
								const isImage = isImageFile(file.name);
								
								return (
									<div
										key={file.id}
										onClick={() => handleFileSelect(file.id)}
										className={cn(
											"group relative flex flex-col rounded-lg border bg-muted/50 text-card-foreground cursor-pointer transition-all overflow-hidden hover:bg-muted/80",
											isSelected && "border-primary/50 ring-1 ring-primary/50"
										)}
									>
										<div className="aspect-square w-full bg-background flex items-center justify-center overflow-hidden relative border-b">
											{isImage ? (
												<img 
													src={file.url} 
													alt={file.name}
													className="w-full h-full object-cover transition-transform group-hover:scale-105"
													loading="lazy"
												/>
											) : (
												<FileIcon className="h-8 w-8 text-muted-foreground/50" />
											)}

											{file.content && (
												<div className="absolute bottom-1 right-1 bg-green-500/80 text-[8px] text-white px-1 py-0.5 rounded-sm font-bold uppercase tracking-wider transform scale-90 origin-bottom-right z-10">
													<ScanText className="inline-block h-4 w-3" />
												</div>
											)}
											
											{isSelected && (
												<div className="absolute inset-0 bg-primary/10 flex items-center justify-center">
													<div className="bg-primary text-primary-foreground rounded-full p-1 shadow-sm">
														<Check className="h-4 w-4" />
													</div>
												</div>
											)}
										</div>
										
										<div className="p-2 space-y-0.5">
											<p className="text-xs truncate" title={file.name}>
												{file.name}
											</p>
											<div className="flex items-center justify-between text-[10px] text-muted-foreground">
												<span>{formatFileSize(file.size)}</span>
												<span>{new Date(file.createdAt).toLocaleDateString()}</span>
											</div>
										</div>
									</div>
								);
							})}
						</div>
					)}
				</ScrollArea>

				<div className="p-4 border-t bg-muted/10 space-y-4 flex-shrink-0">
					<div 
						className={cn(
							"border-2 border-dashed rounded-lg p-4 flex flex-col items-center justify-center text-center cursor-pointer transition-colors",
							isDragging ? "border-primary bg-primary/5" : "border-muted-foreground/25",
							uploading && "opacity-50 pointer-events-none"
						)}
						onDragOver={handleDragOver}
						onDragLeave={handleDragLeave}
						onDrop={handleDrop}
						onClick={() => fileInputRef.current?.click()}
					>
						<input
							type="file"
							ref={fileInputRef}
							className="hidden"
							multiple
							onChange={handleFileChange}
						/>
						{uploading ? (
							<Loader2 className="h-8 w-8 animate-spin text-muted-foreground mb-2" />
						) : (
							<Upload className="h-8 w-8 text-muted-foreground mb-2" />
						)}
						<p className="text-sm font-medium">
							{uploading ? "Uploading..." : "Click to upload or drag and drop"}
						</p>
					</div>

					<div className="flex items-center justify-between w-full">
						<div className="text-sm text-muted-foreground">
							{selectedFileIds.size} file{selectedFileIds.size !== 1 ? "s" : ""} selected
						</div>
						<div className="flex gap-2">
							<Button variant="outline" onClick={() => onOpenChange(false)}>
								Cancel
							</Button>
							<Button onClick={handleAttach} disabled={selectedFileIds.size === 0 || deleting}>
								Attach Selected
							</Button>
						</div>
					</div>
				</div>
			</DialogContent>
		</Dialog>
	);
}
