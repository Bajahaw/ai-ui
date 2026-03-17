import { useState, useEffect, useRef, useMemo, useCallback, memo, forwardRef, useImperativeHandle } from "react";
import { cn } from "@/lib/utils";
import { useModels } from "@/hooks/useModels";
import { useSettings } from "@/hooks/useSettings";
import { useSettingsData } from "@/hooks/useSettingsData";

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
	PromptInputTextareaHandle,
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
	ToolApproval,
} from "@/components/ai-elements/tool";
import {
	// GlobeIcon,
	AlertCircleIcon,
	RotateCcwIcon,
	CopyIcon,
	EditIcon,
	InfoIcon,
	CheckIcon,
	XIcon,
	Plus,
} from "lucide-react";
import { Loader } from "@/components/ai-elements/loader";
import { Actions, Action } from "@/components/ai-elements/actions";
import { Attachment, FrontendMessage, ToolCall, WelcomeStats } from "@/lib/api/types";
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
import { ModelOption } from "@/components/ai-elements/model-select";

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

const ToolCallItem = ({ toolCall, settingsData }: { toolCall: ToolCall; settingsData: any }) => {
	// Find tool definition
	const tool = settingsData.tools.find((t: any) => t.name === toolCall.name);

	// Determine the state based on whether output has arrived or approval is needed
	const initialState = toolCall.tool_output
		? ("output-available" as const)
		: tool?.require_approval
			? ("awaiting-approval" as const)
			: ("input-available" as const);

	const [localState, setLocalState] = useState(initialState);

	// Update local state if the toolCall changes (e.g. output arrives)
	useEffect(() => {
		setLocalState(initialState);
	}, [initialState]);

	return (
		<Tool key={toolCall.id} defaultOpen={localState === "awaiting-approval"}>
			<ToolHeader
				type={`tool-${toolCall.name}` as `tool-${string}`}
				state={localState}
				mcpUrl={(() => {
					if (!tool || !tool.mcp_server_id) return undefined;
					const server = settingsData.mcpServers.find((s: any) => s.id === tool.mcp_server_id);
					return server?.endpoint;
				})()}
			/>
			<ToolContent>
				<ToolInput input={safeParseJSON(toolCall.args)} />
				{localState === "awaiting-approval" && (
					<ToolApproval
						toolCallId={toolCall.id}
						onAction={(approved) => {
							if (approved) {
								setLocalState("input-available");
							}
						}}
					/>
				)}
				{toolCall.tool_output && (
					<ToolOutput output={toolCall.tool_output} errorText={undefined} />
				)}
			</ToolContent>
		</Tool>
	);
};

interface ChatInterfaceProps {
	messages: FrontendMessage[];
	webSearch: boolean;
	currentConversation: ClientConversation | undefined;
	stats?: WelcomeStats;
	isAuthenticated?: boolean;
	isAuthChecking?: boolean;
	isConversationLoading?: boolean;
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
	onCancelStream: () => Promise<void>;
}

type PromptAreaHandle = {
	focus(options?: FocusOptions): void;
};

type PromptAreaProps = {
	onSend: (message: string, attachments?: Attachment[]) => void;
	placeholder: string;
	isDisabled: boolean;
	models: ModelOption[];
	modelsLoading: boolean;
	model: string;
	onModelChange: (model: string) => void;
	settingsLoading: boolean;
	isModelValid: boolean;
	hasPendingMessages: boolean;
	hasRecentError: boolean;
	onStop: () => void;
	isAuthenticated: boolean;
};

const PromptArea = memo(forwardRef<PromptAreaHandle, PromptAreaProps>(function PromptArea({
	onSend,
	placeholder,
	isDisabled,
	models,
	modelsLoading,
	model,
	onModelChange,
	settingsLoading,
	isModelValid,
	hasPendingMessages,
	hasRecentError,
	onStop,
	isAuthenticated,
}, ref) {
	const [isInputEmpty, setIsInputEmpty] = useState(true);
	const [uploadedFiles, setUploadedFiles] = useState<UploadedFile[]>([]);
	const [uploadError, setUploadError] = useState<string | null>(null);
	const [fileManagerOpen, setFileManagerOpen] = useState(false);
	const promptInputRef = useRef<PromptInputTextareaHandle>(null);

	useImperativeHandle(ref, () => ({
		focus: (options?: FocusOptions) => promptInputRef.current?.focus(options),
	}), []);

	const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
		setIsInputEmpty(e.target.value.trim() === "");
	}, []);

	const handleAttachFiles = useCallback((files: import("@/lib/api/types").File[]) => {
		const newUploadedFiles = files.map(fileData => ({
			file: new File([], fileData.name),
			fileData,
		}));
		setUploadedFiles(prev => [...prev, ...newUploadedFiles]);
		setUploadError(null);
	}, []);

	const handleRemoveFile = useCallback((index: number) => {
		setUploadedFiles(prev => prev.filter((_, i) => i !== index));
		setUploadError(null);
	}, []);

	const handleFilesPasted = useCallback(async (files: File[]) => {
		if (hasPendingMessages || files.length === 0) return;
		for (const file of files) {
			try {
				setUploadError(null);
				const fileData = await uploadFile(file);
				setUploadedFiles(prev => [...prev, { file, fileData }]);
			} catch (error) {
				const errorMessage = error instanceof FileUploadError
					? error.message
					: "Failed to upload pasted file";
				setUploadError(errorMessage);
				break;
			}
		}
	}, [hasPendingMessages]);

	const handleSubmit = useCallback((e: React.FormEvent) => {
		e.preventDefault();
		if (!promptInputRef.current?.value.trim() && uploadedFiles.length === 0) return;
		if (!isModelValid) return;
		const message = promptInputRef.current?.value ?? "";
		const attachments: Attachment[] | undefined = uploadedFiles.length > 0
			? uploadedFiles.map(({ fileData }) => ({
				id: fileData.id,
				messageId: -1,
				file: fileData,
			}))
			: undefined;
		promptInputRef.current?.clear();
		setIsInputEmpty(true);
		setUploadedFiles([]);
		setUploadError(null);
		onSend(message, attachments);
	}, [isModelValid, uploadedFiles, onSend]);

	return (
		<div className="flex-shrink-0 flex justify-center !p-6 !pt-0">
			<PromptInput
				onSubmit={handleSubmit}
				className="chat-interface w-full max-w-3xl mx-auto"
			>
				{uploadedFiles.length > 0 && (
					<div className="px-3 py-2 border-b">
						<FilesList files={uploadedFiles} onRemove={handleRemoveFile} />
					</div>
				)}
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
					onChange={handleInputChange}
					placeholder={placeholder}
					disabled={isDisabled}
					onFilesPasted={handleFilesPasted}
				/>
				<PromptInputToolbar>
					<PromptInputTools>
						<PromptInputButton
							variant="ghost"
							onClick={() => setFileManagerOpen(true)}
							disabled={hasPendingMessages || isDisabled}
							title={!isAuthenticated ? "Sign in to attach files" : "Attach files"}
						>
							<Plus size={16} />
						</PromptInputButton>
						<PromptInputModelSelect
							models={models}
							value={isModelValid ? model : undefined}
							onChange={onModelChange}
							loading={modelsLoading}
							disabled={settingsLoading || isDisabled}
							helperMessage="Add AI providers in settings"
							variant="ghost"
							size="sm"
							showCount={models.length > 0}
							triggerClassName="max-sm:max-w-[200px] max-sm:flex max-sm:items-center max-sm:gap-1 font-medium max-sm:[&>span]:min-w-0 max-sm:[&>span]:flex-1 max-sm:[&>span]:truncate"
						/>
					</PromptInputTools>
					<PromptInputSubmit
						disabled={
							isDisabled ||
							(!hasPendingMessages && (isInputEmpty && uploadedFiles.length === 0)) ||
							(!hasPendingMessages && (!model || models.length === 0))
						}
						status={
							hasPendingMessages
								? "streaming"
								: hasRecentError
									? "error"
									: undefined
						}
						onStop={onStop}
					/>
				</PromptInputToolbar>
			</PromptInput>
			<FileManagerDialog
				open={fileManagerOpen}
				onOpenChange={setFileManagerOpen}
				onAttach={handleAttachFiles}
			/>
		</div>
	);
}));

export const ChatInterface = ({
	messages,
	webSearch,
	currentConversation,
	stats,
	isAuthenticated = false,
	isAuthChecking = false,
	isConversationLoading = false,
	// onWebSearchToggle,
	onSendMessage,
	onRetryMessage,
	onSwitchBranch,
	getBranchInfo,
	onUpdateMessage,
	onCancelStream,
}: ChatInterfaceProps) => {
	const isComposerDisabled = isAuthChecking || !isAuthenticated;
	const { models, isLoading: modelsLoading } = useModels();
	const {
		updateSingleSetting,
		getSingleSetting,
		isLoading: settingsLoading,
	} = useSettings();

	const { data: settingsData, loaded: settingsDataLoaded, fetchAll: fetchSettingsData } = useSettingsData();

	useEffect(() => {
		if (!settingsDataLoaded) {
			fetchSettingsData();
		}
	}, [settingsDataLoaded, fetchSettingsData]);

	// Auto-select default model when models become available
	const [model, setModel] = useState<string>("");
	const [retryingMessageId, setRetryingMessageId] = useState<string | null>(
		null,
	);
	const [editingMessageId, setEditingMessageId] = useState<string | null>(null);
	const [updatingMessageId, setUpdatingMessageId] = useState<string | null>(
		null,
	);
	const editableMessageRefs = useRef<Record<string, EditableMessageRef | null>>(
		{},
	);
	const messageRenderKeysRef = useRef<WeakMap<FrontendMessage, string>>(new WeakMap());
	const messageRenderKeysByIdRef = useRef<Map<string, string>>(new Map());
	const nextMessageRenderKeyRef = useRef(0);
	const conversationRef = useRef<HTMLDivElement>(null);
	const bottomAnchorRef = useRef<HTMLDivElement>(null);
	const promptAreaRef = useRef<PromptAreaHandle>(null);
	const [hasInteracted, setHasInteracted] = useState(false);
	const isConversationLoadingRef = useRef(isConversationLoading);
	const hasFocusedInitialLoadRef = useRef(false);
	const previousConversationIdRef = useRef<string | undefined>(undefined);
	const pendingInitialScrollConversationIdRef = useRef<string | null>(null);
	const initialScrollUserInteractedRef = useRef(false);

	// Keep a ref in sync so scroll-settle closures always see the current loading state.
	useEffect(() => {
		isConversationLoadingRef.current = isConversationLoading;
	}, [isConversationLoading]);

	// Focus composer once on initial app load when it becomes usable.
	useEffect(() => {
		if (hasFocusedInitialLoadRef.current) return;
		if (isComposerDisabled) return;

		const input = promptAreaRef.current;
		if (!input) return;

		input.focus({ preventScroll: true });
		hasFocusedInitialLoadRef.current = true;
	}, [isComposerDisabled]);

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

	// Reset interaction flag when conversation changes.
	// Initial scroll is handled by a dedicated effect so it also works with lazy-loaded messages.
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
			pendingInitialScrollConversationIdRef.current = currentId ?? null;
			initialScrollUserInteractedRef.current = false;
			messageRenderKeysRef.current = new WeakMap();
			messageRenderKeysByIdRef.current.clear();
			nextMessageRenderKeyRef.current = 0;
		}
	}, [currentConversation?.id]);

	useEffect(() => {
		const currentId = currentConversation?.id;
		if (!currentId) return;
		if (pendingInitialScrollConversationIdRef.current !== currentId) return;

		// If initial loading completed with no messages, clear the pending initial scroll.
		if (!isConversationLoading && messages.length === 0) {
			pendingInitialScrollConversationIdRef.current = null;
			return;
		}

		if (messages.length === 0) return;

		const container = conversationRef.current;
		const bottomAnchor = bottomAnchorRef.current;
		if (!container) return;
		if (!bottomAnchor) return;
		const observedContent = container.firstElementChild;
		let settleTimeoutId: number | null = null;

		const scrollToConversationBottom = () => {
			bottomAnchor.scrollIntoView({ block: "end", behavior: "auto" });
			container.scrollTop = container.scrollHeight;
		};

		const clearPendingInitialScroll = () => {
			if (pendingInitialScrollConversationIdRef.current === currentId) {
				pendingInitialScrollConversationIdRef.current = null;
			}
		};

		const scheduleSettle = () => {
			if (settleTimeoutId !== null) {
				window.clearTimeout(settleTimeoutId);
			}

			// Don't start the settle countdown while data is still loading — images and SVGs
			// inside the freshly-loaded messages haven't had a chance to paint yet.
			if (isConversationLoadingRef.current) return;

			settleTimeoutId = window.setTimeout(() => {
				if (initialScrollUserInteractedRef.current) return;
				clearPendingInitialScroll();
			}, 2500);
		};

		const stopStickyInitialScroll = () => {
			initialScrollUserInteractedRef.current = true;
			clearPendingInitialScroll();
		};

		const resizeObserver = new ResizeObserver(() => {
			if (pendingInitialScrollConversationIdRef.current !== currentId) return;
			if (initialScrollUserInteractedRef.current) return;

			scrollToConversationBottom();
			scheduleSettle();
		});

		const mutationObserver = new MutationObserver(() => {
			if (pendingInitialScrollConversationIdRef.current !== currentId) return;
			if (initialScrollUserInteractedRef.current) return;

			scrollToConversationBottom();
			scheduleSettle();
		});

		if (observedContent instanceof HTMLElement) {
			resizeObserver.observe(observedContent);
			mutationObserver.observe(observedContent, {
				childList: true,
				subtree: true,
				attributes: true,
			});
		}

		container.addEventListener("wheel", stopStickyInitialScroll, { passive: true });
		container.addEventListener("touchstart", stopStickyInitialScroll, { passive: true });
		container.addEventListener("pointerdown", stopStickyInitialScroll);

		scrollToConversationBottom();
		requestAnimationFrame(scrollToConversationBottom);
		scheduleSettle();

		return () => {
			if (settleTimeoutId !== null) {
				window.clearTimeout(settleTimeoutId);
			}

			resizeObserver.disconnect();
			mutationObserver.disconnect();
			container.removeEventListener("wheel", stopStickyInitialScroll);
			container.removeEventListener("touchstart", stopStickyInitialScroll);
			container.removeEventListener("pointerdown", stopStickyInitialScroll);
		};
	}, [currentConversation?.id, messages.length, isConversationLoading]);

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
		.some((message) => !!message.error);

	// Validate selected model exists in the loaded models list
	const isModelValid = useMemo(() => {
		return !!model && models.some((m) => m.id === model);
	}, [model, models]);

	// Clear inline warning when model becomes valid
	// (no-op — model warning state removed)

	const handleSend = useCallback((message: string, attachments?: Attachment[]) => {
		setHasInteracted(true);
		if (messages.length > 0) scrollToBottom();
		onSendMessage(message, webSearch, model, attachments);
	}, [messages.length, webSearch, model, scrollToBottom, onSendMessage]);

	const copyMessage = (messageId: string | null, fallbackContent: string) => {
		const element = messageId
			? editableMessageRefs.current[messageId]?.getContentElement()
			: null;

		if (element) {
			const range = document.createRange();
			range.selectNodeContents(element);
			const selection = window.getSelection();
			selection?.removeAllRanges();
			selection?.addRange(range);
			// execCommand('copy') is deprecated but remains the only way to copy
			// a live selection with full computed styles (fonts, tables, colours, etc.).
			// The modern Clipboard API does not support selection-based copying.
			// eslint-disable-next-line @typescript-eslint/no-deprecated
			const success = document.execCommand("copy");
			selection?.removeAllRanges();
			if (!success) {
				navigator.clipboard.writeText(fallbackContent).catch(console.error);
			}
		} else {
			navigator.clipboard.writeText(fallbackContent).catch(console.error);
		}
	};

	const handleRetryMessage = async (messageId: string) => {
		// Prevent retry when model is invalid
		if (!isModelValid) {
			return;
		}

		// Match send-message behavior: keep extra bottom space and move viewport
		// so the retried streaming response stays in view.
		setHasInteracted(true);
		scrollToBottom();

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
					currentConversation?.backendConversation?.messages?.[messageId];

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
			// If assistant message is pending, replace action buttons with a small spinner
			if (message.role === "assistant" && message.status === "pending") {
				return (
					<div className="flex items-center py-2 text-muted-foreground">
						<Loader size={16} />
					</div>
				);
			}
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
								message.error ? "Copy error" : "Copy message"
							}
							onClick={() =>
								copyMessage(
									message.error ? null : message.id,
									message.error
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
									message.error
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
									message.error
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

					{/* Info button showing model / tokens / context / speed (assistant only) */}
					{message.role === "assistant" && (
						<Action
							tooltip={
								`${message.model || "model unknown"}
								Tokens: ${message.tokenCount ?  `${Math.round(message.tokenCount * 0.001 * 10) / 10}` : "0"}k
								Context: ${message.contextSize ? `${Math.round(message.contextSize * 0.001 * 10) / 10}` : "0"}k
								Speed: ${ message.speed !== undefined ? `${message.speed}` : "0" } t/s`}
							aria-label="Message metadata"
						>
							<InfoIcon className="size-4" />
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
		if (message.error) {
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

		return (
			<EditableMessage
				ref={(ref) => {
					if (ref) {
						editableMessageRefs.current[message.id] = ref;
					}
				}}
				content={message.content}
				status={message.status}
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
		let messageKey = messageRenderKeysRef.current.get(message);
		if (!messageKey) {
			messageKey = messageRenderKeysByIdRef.current.get(message.id);
			if (!messageKey) {
				messageKey = `msg-${nextMessageRenderKeyRef.current++}`;
			}
			messageRenderKeysRef.current.set(message, messageKey);
		}
		messageRenderKeysByIdRef.current.set(message.id, messageKey);

		return (
			<div key={messageKey}>
				{/* Render message with optional attachments in a single component to manage margins better */}
				<MessageComponent
					from={message.role}
					status={message.status}
					error={message.error}
					className={message.role === "user" ? "pb-1" : ""}
				>
					<div className={cn(
						"flex flex-col gap-3 w-full",
						message.role === "user" ? "items-end" : "items-start"
					)}>
						{/* Render attachments first if they exists */}
						{hasAttachments && (
							<AttachmentMessage
								attachments={message.attachments}
								role={message.role}
							/>
						)}

						{/* Render main message content */}
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
											{message.toolCalls.map((toolCall) => (
												<ToolCallItem
													key={toolCall.id}
													toolCall={toolCall}
													settingsData={settingsData}
												/>
											))}
										</div>
									)}
									{renderMessageContent(message)}
									{renderMessageActions(message)}
								</div>
							)}
						</MessageContent>
					</div>
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
				{messages.length === 0 && !isConversationLoading ? (
					<div className="h-full flex items-center justify-center">
						<Welcome
							stats={stats}
							isLoading={isAuthChecking}
							message={
								isAuthChecking
									? undefined
									: isAuthenticated
										? "Here is a glimpse of your previous interactions"
										: "Sign in from the sidebar to load your conversations and start chatting"
							}
						/>
					</div>
				) : (
					<ConversationContent className="chat-interface w-full max-w-3xl mx-auto !px-5 lg:!px-3">
						<div className="space-y-4">
							{messages.map((message) => renderMessage(message))}
							{/* Add overscroll spacer only when user has interacted with conversation */}
							{hasInteracted && <div style={{ minHeight: 'calc(-450px + 100vh)' }} />}
							<div ref={bottomAnchorRef} aria-hidden="true" />
						</div>
					</ConversationContent>
				)}
				<ConversationScrollButton />
			</Conversation>

			<PromptArea
				ref={promptAreaRef}
				onSend={handleSend}
				placeholder={ "Ask anything here ..." }
				isDisabled={isComposerDisabled}
				models={models}
				modelsLoading={modelsLoading}
				model={model}
				onModelChange={handleModelChange}
				settingsLoading={settingsLoading}
				isModelValid={isModelValid}
				hasPendingMessages={hasPendingMessages}
				hasRecentError={hasRecentError}
				onStop={onCancelStream}
				isAuthenticated={isAuthenticated}
			/>
		</div>
	);
};
