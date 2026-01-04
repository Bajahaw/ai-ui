import { useState, useEffect, useRef, useMemo, useCallback } from "react";
import { useModels } from "@/hooks/useModels";
import { useSettings } from "@/hooks/useSettings";

import {
	Message as MessageComponent,
	MessageContent,
} from "@/components/ai-elements/message";
import {
	PromptInput,
	PromptInputButton,
	PromptInputModelSelect,
	PromptInputSubmit,
	PromptInputTextarea,
	PromptInputToolbar,
	PromptInputTools,
} from "@/components/ai-elements/prompt-input";

import { Welcome } from "@/components/ai-elements/welcome";

import {
	Conversation,
	ConversationContent,
	ConversationScrollButton,
} from "@/components/ai-elements/conversation";
import {
	Reasoning,
	ReasoningTrigger,
	ReasoningContent,
} from "@/components/ai-elements/reasoning";
import {
	Tool,
	ToolHeader,
	ToolContent,
	ToolInput,
	ToolOutput,
} from "@/components/ai-elements/tool";
import {
	// GlobeIcon,
	AlertCircleIcon,
	RotateCcwIcon,
	CopyIcon,
	EditIcon,
	CheckIcon,
	XIcon,
} from "lucide-react";
import { Loader } from "@/components/ai-elements/loader";
import { Actions, Action } from "@/components/ai-elements/actions";
import { FrontendMessage } from "@/lib/api/types";
import { BranchNavigation } from "@/components/BranchNavigation";
import { ClientConversation } from "@/lib/clientConversationManager";
import {
	EditableMessage,
	EditableMessageRef,
} from "@/components/ai-elements/editable-message";
import {
	FilesList,
	AttachmentMessage,
	UploadedFile,
} from "@/components/ui/file-upload";
import { uploadFile, FileUploadError } from "@/lib/api/files";
import { FileManagerDialog } from "@/components/file-manager/FileManagerDialog";
import { Paperclip } from "lucide-react";

// Dynamic models are now loaded from providers via useModels hook

const safeParseJSON = (jsonString: string | undefined) => {
	if (!jsonString) return {};
	try {
		return JSON.parse(jsonString);
	} catch (e) {
		console.error("Failed to parse JSON:", e);
		return { error: "Invalid JSON", raw: jsonString };
	}
};

interface ChatInterfaceProps {
	messages: FrontendMessage[];
	webSearch: boolean;
	currentConversation: ClientConversation | undefined;
	onWebSearchToggle: (enabled: boolean) => void;
	onSendMessage: (
		message: string,
		webSearch: boolean,
		model: string,
		attachments?: import("@/lib/api/types").Attachment[],
	) => Promise<void>;
	onRetryMessage: (messageId: string, model: string) => Promise<void>;
	onSwitchBranch: (messageId: number, branchIndex: number) => void;
	getBranchInfo: (messageId: number) => {
		count: number;
		activeIndex: number;
		hasMultiple: boolean;
	};
	onUpdateMessage: (messageId: string, newContent: string) => Promise<void>;
}

export const ChatInterface = ({
	messages,
	webSearch,
	currentConversation,
	// onWebSearchToggle,
	onSendMessage,
	onRetryMessage,
	onSwitchBranch,
	getBranchInfo,
	onUpdateMessage,
}: ChatInterfaceProps) => {
	const { models, isLoading: modelsLoading } = useModels();
	const [fileManagerOpen, setFileManagerOpen] = useState(false);
	const {
		updateSingleSetting,
		getSingleSetting,
		isLoading: settingsLoading,
	} = useSettings();

	// Auto-select default model when models become available
	const [model, setModel] = useState<string>("");
	const [input, setInput] = useState("");
	const [retryingMessageId, setRetryingMessageId] = useState<string | null>(
		null,
	);
	const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
	const [updatingMessageId, setUpdatingMessageId] = useState<string | null>(
		null,
	);
	const [uploadedFiles, setUploadedFiles] = useState<UploadedFile[]>([]);
	const [uploadError, setUploadError] = useState<string | null>(null);
	const editableMessageRefs = useRef<Record<string, EditableMessageRef | null>>(
		{},
	);
	const conversationRef = useRef<HTMLDivElement>(null);
	const promptInputRef = useRef<HTMLTextAreaElement>(null);
	const [hasInteracted, setHasInteracted] = useState(false);
	const previousConversationIdRef = useRef<string | undefined>(undefined);

	// Scroll to bottom when user sends a message
	const scrollToBottom = useCallback(() => {
		const container = conversationRef.current;
		if (!container) return;

		// Use setTimeout to ensure the DOM has updated with the new message
		setTimeout(() => {
			container.scrollTo({
				top: container.scrollHeight,
				behavior: "smooth",
			});
		}, 100);
	}, []);

	// Reset interaction flag when conversation changes and scroll to bottom on initial load
	useEffect(() => {
		const currentId = currentConversation?.id;
		const previousId = previousConversationIdRef.current;

		// Check if this is a real conversation switch (not temp ID -> real ID transition)
		// Temp IDs start with "conv-", real IDs are UUIDs (contain dashes in UUID format)
		const isTempId = (id: string | undefined) => id?.startsWith("conv-");
		const isRealConversationSwitch =
			currentId !== previousId &&
			!(isTempId(previousId) && !isTempId(currentId)); // Not transitioning from temp to real

		// Update the ref
		previousConversationIdRef.current = currentId;

		// Only reset interaction and auto-scroll when truly switching conversations
		if (isRealConversationSwitch) {
			setHasInteracted(false);

			// Only auto-scroll when switching to a conversation that already has messages
			if (messages.length > 0) {
				const container = conversationRef.current;
				if (!container) return;

				// Use auto behavior for instant positioning without animation
				requestAnimationFrame(() => {
					container.scrollTo({
						top: container.scrollHeight,
						behavior: "auto", // Instant, no smooth scrolling
					});
				});
			}
		}
	}, [currentConversation?.id, messages.length]);

	/**
	 * Synchronize local model state with the default model setting
	 * This ensures the prompt input always reflects the current default model
	 */
	useEffect(() => {
		if (models.length > 0 && !settingsLoading) {
			const savedModel = getSingleSetting("defaultModel");

			// Check if saved model is still available in current providers
			const isModelAvailable =
				savedModel && models.find((m) => m.id === savedModel);

			if (isModelAvailable && !model) {
				setModel(savedModel); // Sync is only needed if no local model is set
			} else if (!savedModel && !model && models.length > 0) {
				// No default model exists and no local model - let auto-select hook handle this
				// This prevents race conditions between auto-select and this component
			} else if (savedModel && !isModelAvailable && models.length > 0) {
				// Saved model is no longer available, update to first available
				const fallbackModel = models[0].id;
				setModel(fallbackModel);
				updateSingleSetting("defaultModel", fallbackModel).catch((error) => {
					console.error("Failed to update default model setting:", error);
				});
				console.warn(
					`Saved model "${savedModel}" is no longer available. Falling back to "${fallbackModel}".`,
				);
			}
		}
	}, [models, settingsLoading, getSingleSetting, updateSingleSetting, model]);


	/**
	 * Handle model selection change and persist to settings
	 * Updates both local state and saves preference to backend
	 * This ensures the model choice is remembered across sessions
	 */
	const handleModelChange = async (newModel: string) => {
		setModel(newModel);

		// Default model should only be updated from settings
		// try {
		//   await updateSingleSetting("defaultModel", newModel);
		// } catch (error) {
		//   console.error("Failed to save model preference:", error);
		// }
	};

	// Check if there are any pending messages
	const hasPendingMessages = messages.some(
		(message) => message.status === "pending",
	);

	// Check if there are recent error messages (within last 2 messages)
	const hasRecentError = messages
		.slice(-2)
		.some((message) => message.status === "error");

	// Validate selected model exists in the loaded models list
	const isModelValid = useMemo(() => {
		return !!model && models.some((m) => m.id === model);
	}, [model, models]);

	// Clear inline warning when model becomes valid
	// (no-op — model warning state removed)

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		if (!input.trim() && uploadedFiles.length === 0) return;

		// Prevent sending when model is not selected or invalid
		if (!isModelValid) {
			return;
		}

		const message = input;
		// Create full attachment objects for optimistic rendering
		const attachments: import("@/lib/api/types").Attachment[] | undefined = uploadedFiles.length > 0
			? uploadedFiles.map((uploadedFile) => ({
				id: uploadedFile.fileData.id,
				messageId: -1, // Temporary, will be updated by backend
				file: uploadedFile.fileData,
			}))
			: undefined;
		setInput("");
		setUploadedFiles([]);
		setUploadError(null);

		// Mark that user has interacted with this conversation
		setHasInteracted(true);

		// Only scroll to bottom if there are already messages (not the first message)
		// This prevents auto-scroll when starting a new conversation
		if (messages.length > 0) {
			scrollToBottom();
		}

		// Send message with full attachment objects for optimistic rendering
		// and file IDs for backend API
		onSendMessage(message, webSearch, model, attachments);
	};
	
	const handleAttachFiles = useCallback((files: import("@/lib/api/types").File[]) => {
		const newUploadedFiles = files.map(fileData => ({
			file: new File([], fileData.name), // Dummy file object since we already have the data
			fileData
		}));
		setUploadedFiles(prev => [...prev, ...newUploadedFiles]);
		setUploadError(null);
	}, []);

	const handleRemoveFile = useCallback((index: number) => {
		setUploadedFiles(prev => prev.filter((_, i) => i !== index));
		setUploadError(null);
	}, []);

	const handleFilesPasted = useCallback(
		async (files: File[]) => {
			if (hasPendingMessages || files.length === 0) return;

			// Upload all pasted files
			for (const file of files) {
				try {
					setUploadError(null);
					const fileData = await uploadFile(file);
					setUploadedFiles(prev => [...prev, { file, fileData }]);
				} catch (error) {
					const errorMessage =
						error instanceof FileUploadError
							? error.message
							: "Failed to upload pasted file";
					setUploadError(errorMessage);
					break; // Stop uploading remaining files on error
				}
			}
		},
		[hasPendingMessages],
	);

	const copyMessage = async (content: string) => {
		try {
			await navigator.clipboard.writeText(content);
		} catch (error) {
			console.error("Failed to copy message:", error);
		}
	};

	const handleRetryMessage = async (messageId: string) => {
		// Prevent retry when model is invalid
		if (!isModelValid) {
			return;
		}

		setRetryingMessageId(messageId);
		try {
			await onRetryMessage(messageId, model);
		} finally {
			setRetryingMessageId(null);
		}
	};

	const handleUpdateMessage = async (messageId: string, newContent: string) => {
		setUpdatingMessageId(messageId);
		try {
			await onUpdateMessage(messageId, newContent);
			setEditingMessageId(null);
		} catch (error) {
			console.error("Failed to update message:", error);
		} finally {
			setUpdatingMessageId(null);
		}
	};

	// Create a simple cache for branch info that only updates when conversation data changes
	const branchInfoCache = useMemo(() => {
		const cache = new Map<string, { branchInfo: any; parentId: number }>();

		// Pre-compute branch info for all messages to avoid repeated calculations
		for (const message of messages) {
			if (message.role === "assistant") {
				const messageId = parseInt(message.id);
				// messages may be undefined until fetched — use optional chaining
				const assistantMessage =
					currentConversation?.backendConversation?.messages[messageId];

				if (assistantMessage?.parentId) {
					const parentId = assistantMessage.parentId;
					const branchInfo = getBranchInfo(parentId);
					cache.set(message.id, { branchInfo, parentId });
				} else {
					// For messages not yet in backend conversation (newly added), provide default info
					cache.set(message.id, {
						branchInfo: { count: 1, activeIndex: 0, hasMultiple: false },
						parentId: messageId,
					});
				}
			} else {
				cache.set(message.id, {
					branchInfo: { count: 1, activeIndex: 0, hasMultiple: false },
					parentId: parseInt(message.id),
				});
			}
		}

		return cache;
	}, [
		messages,
		currentConversation?.backendConversation,
		currentConversation?.activeBranches,
		getBranchInfo,
	]);

	const renderMessageActions = useCallback(
		(message: FrontendMessage) => {
			const messageInfo = branchInfoCache.get(message.id);
			if (!messageInfo) return null;

			const { branchInfo, parentId } = messageInfo;

			return (
				<div className="flex items-center gap-2">
					{/* Action buttons */}
					<Actions className="opacity-60 hover:opacity-100 transition-opacity">
						{message.status !== "pending" &&
							editingMessageId !== message.id && (
								<Action
									tooltip="Edit message"
									onClick={() => setEditingMessageId(message.id)}
									disabled={updatingMessageId === message.id}
								>
									<EditIcon className="size-4" />
								</Action>
							)}

						{editingMessageId === message.id && (
							<>
								<Action
									tooltip="Save changes"
									onClick={() =>
										editableMessageRefs.current[message.id]?.triggerSave()
									}
									disabled={updatingMessageId === message.id}
								>
									<CheckIcon className="size-4" />
								</Action>
								<Action
									tooltip="Cancel editing"
									onClick={() => setEditingMessageId(null)}
									disabled={updatingMessageId === message.id}
								>
									<XIcon className="size-4" />
								</Action>
							</>
						)}

						<Action
							tooltip={
								message.status === "error" ? "Copy error" : "Copy message"
							}
							onClick={() =>
								copyMessage(
									message.status === "error"
										? message.error || "Error occurred"
										: message.content,
								)
							}
						>
							<CopyIcon className="size-4" />
						</Action>

						{message.role === "assistant" && message.status !== "pending" && (
							<Action
								tooltip={
									message.status === "error"
										? "Retry getting response"
										: "Regenerate response"
								}
								onClick={() => handleRetryMessage(message.id)}
								disabled={
									retryingMessageId === message.id ||
									!isModelValid ||
									models.length === 0
								}
								className={
									message.status === "error"
										? "text-destructive hover:text-destructive-foreground hover:bg-destructive"
										: ""
								}
							>
								{retryingMessageId === message.id ? (
									<div className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
								) : (
									<RotateCcwIcon className="size-4" />
								)}
							</Action>
						)}
					</Actions>

					{/* Branch navigation for assistant messages with multiple branches */}
					{message.role === "assistant" && branchInfo.hasMultiple && (
						<BranchNavigation
							currentIndex={branchInfo.activeIndex}
							totalCount={branchInfo.count}
							onPrevious={() => {
								const newIndex = Math.max(0, branchInfo.activeIndex - 1);
								onSwitchBranch(parentId, newIndex);
							}}
							onNext={() => {
								const newIndex = Math.min(
									branchInfo.count - 1,
									branchInfo.activeIndex + 1,
								);
								onSwitchBranch(parentId, newIndex);
							}}
						/>
					)}
				</div>
			);
		},
		[
			branchInfoCache,
			editingMessageId,
			updatingMessageId,
			retryingMessageId,
			onSwitchBranch,
			handleRetryMessage,
			copyMessage,
			setEditingMessageId,
			editableMessageRefs,
		],
	);

	const renderMessageContent = (message: FrontendMessage) => {
		if (message.status === "error") {
			return (
				<div className="space-y-2">
					<div className="flex items-center gap-2 text-destructive">
						<AlertCircleIcon className="size-4 flex-shrink-0" />
						<span className="font-medium">
							{message.role === "assistant"
								? "Failed to get response"
								: "Failed to send message"}
						</span>
					</div>
					<div className="text-base text-destructive/80">
						{message.error || "An unknown error occurred"}
					</div>
				</div>
			);
		}

		if (
			message.status === "pending" &&
			message.role === "assistant" &&
			message.content === ""
		) {
			return (
				<div className="flex items-center gap-2 text-muted-foreground">
					<Loader size={16} />
				</div>
			);
		}

		return (
			<EditableMessage
				ref={(ref) => {
					if (ref) {
						editableMessageRefs.current[message.id] = ref;
					}
				}}
				content={message.content}
				isEditing={editingMessageId === message.id}
				onSave={(newContent) => handleUpdateMessage(message.id, newContent)}
				onCancel={() => setEditingMessageId(null)}
				disabled={updatingMessageId === message.id}
			/>
		);
	};

	const renderMessage = (message: FrontendMessage) => {
		const hasAttachments =
			message.attachments && message.attachments.length > 0;

		return (
			<div key={message.id}>
				{/* Render attachments message first if exists (not counted in conversation tree) */}
				{hasAttachments && (
					<MessageComponent
						from={message.role}
						status="success"
						className="pb-0"
					>
						<MessageContent className="!p-0">
							<AttachmentMessage
								attachments={message.attachments}
							/>
						</MessageContent>
					</MessageComponent>
				)}

				{/* Render main message */}
				<MessageComponent
					from={message.role}
					status={message.status}
					className={message.role === "user" ? "pb-1" : ""}
				>
					<MessageContent content={message.content}>
						{message.role === "user" ? (
							renderMessageContent(message)
						) : (
							<div className="space-y-4">
								{message.reasoning && (
									<Reasoning
										isStreaming={message.status === "pending"}
										duration={message.reasoningDuration}
										defaultOpen={false}
									>
										<ReasoningTrigger />
										<ReasoningContent>{message.reasoning}</ReasoningContent>
									</Reasoning>
								)}
								{message.toolCalls && message.toolCalls.length > 0 && (
									<div className="">
										{message.toolCalls.map((toolCall) => {
											// Determine the state based on whether output has arrived
											const toolState = toolCall.tool_output
												? "output-available" as const
												: "input-available" as const;

											return (
												<Tool key={toolCall.id} defaultOpen={false}>
													<ToolHeader
														type={`tool-${toolCall.name}` as `tool-${string}`}
														state={toolState}
													/>
													<ToolContent>
														<ToolInput
															input={safeParseJSON(toolCall.args)}
														/>
														{toolCall.tool_output && (
															<ToolOutput
																output={toolCall.tool_output}
																errorText={undefined}
															/>
														)}
													</ToolContent>
												</Tool>
											);
										})}
									</div>
								)}
								{renderMessageContent(message)}
								{message.status !== "pending" && renderMessageActions(message)}
							</div>
						)}
					</MessageContent>
				</MessageComponent>

				{message.role === "user" && message.status !== "pending" && (
					<div className="flex justify-end">
						{renderMessageActions(message)}
					</div>
				)}
			</div>
		);
	};

	return (
		<div className="flex-1 flex flex-col min-h-0">
			<Conversation ref={conversationRef} className="flex-1">
				<ConversationContent className="chat-interface w-full max-w-3xl mx-auto !px-5 lg:!px-3">
					{messages.length === 0 ? (
						<div className="h-full flex items-center justify-center">
							<Welcome />
						</div>
					) : (
						<div className="space-y-4">
							{messages.map((message) => renderMessage(message))}
							{/* Add overscroll spacer only when user has interacted with conversation */}
							{hasInteracted && <div style={{ minHeight: 'calc(-450px + 100vh)' }} />}
						</div>
					)}
				</ConversationContent>
				<ConversationScrollButton />
			</Conversation>

			<div className="flex-shrink-0 flex justify-center !p-6 !pt-0">
				<PromptInput
					onSubmit={handleSubmit}
					className="chat-interface w-full max-w-3xl mx-auto"
				>
					{/* Files list preview area */}
					{uploadedFiles.length > 0 && (
						<div className="px-3 py-2 border-b">
							<FilesList files={uploadedFiles} onRemove={handleRemoveFile} />
						</div>
					)}

					{/* Upload error display */}
					{uploadError && (
						<div className="p-3 border-b">
							<div className="flex items-center gap-2 text-destructive text-base">
								<AlertCircleIcon size={16} />
								<span>{uploadError}</span>
								<button
									onClick={() => setUploadError(null)}
									className="ml-auto text-muted-foreground hover:text-foreground"
								>
									<XIcon size={14} />
								</button>
							</div>
						</div>
					)}

					<PromptInputTextarea
						ref={promptInputRef}
						autoFocus
						onChange={(e) => setInput(e.target.value)}
						value={input}
						placeholder="Ask anything here ..."
						onFilesPasted={handleFilesPasted}
					/>
					<PromptInputToolbar>
						<PromptInputTools>
							<PromptInputButton
								variant="ghost"
								onClick={() => setFileManagerOpen(true)}
								disabled={hasPendingMessages}
								title="Attach files"
							>
								<Paperclip size={16} />
							</PromptInputButton>
							{/* <PromptInputButton
								variant={webSearch ? "default" : "ghost"}
								onClick={() => onWebSearchToggle(!webSearch)}
							>
								<GlobeIcon size={16} />
								<span className="hidden sm:inline">Search</span>
							</PromptInputButton> */}

							<PromptInputModelSelect
								models={models}
								value={isModelValid ? model : undefined}
								onChange={handleModelChange}
								loading={modelsLoading}
								disabled={settingsLoading}
								helperMessage="Add AI providers in settings"
								variant="ghost"
								size="sm"
								showCount={models.length > 0}
								triggerClassName="max-sm:max-w-[200px] max-sm:flex max-sm:items-center max-sm:gap-1 font-medium max-sm:[&>span]:min-w-0 max-sm:[&>span]:flex-1 max-sm:[&>span]:truncate"
							/>
						</PromptInputTools>
						<PromptInputSubmit
							disabled={
								(!input.trim() && uploadedFiles.length === 0) ||
								!model ||
								models.length === 0
							}
							status={
								hasPendingMessages
									? "submitted"
									: hasRecentError
										? "error"
										: undefined
							}
						/>
					</PromptInputToolbar>
				</PromptInput>
			</div>

			<FileManagerDialog
				open={fileManagerOpen}
				onOpenChange={setFileManagerOpen}
				onAttach={handleAttachFiles}
			/>
		</div>
	);
};
