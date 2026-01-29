// Backend API Types matching Go structures

export interface AuthStatus {
	authenticated: boolean;
}

// File types
export interface File {
	id: string;
	name: string;
	type: string;
	size: number;
	path: string;
	url: string;
	content: string;
	createdAt: string;
}

export interface Attachment {
	id: string;
	messageId: number;
	file: File;
}

export interface Message {
	id: number;

	convId: string;
	role: string;
	model?: string;

	content: string;
	reasoning?: string;
	status: string;
	tools?: ToolCall[];

	parentId?: number;

	children: number[];

	attachments?: Attachment[];
	error?: string;
	// New metadata fields from streaming/completion
	speed?: number;
	tokenCount?: number;
	contextSize?: number;
}

export interface Conversation {
	id: string;

	userId: string;
	title?: string;

	createdAt: string;
	updatedAt: string;

	// Client-only compatibility fields
	messages: Record<number, Message>; // Always initialized to {} in frontend
	root?: number[];
	activeMessageId?: number;
	activeBranches?: Record<number, number>; // messageId -> activeChildId mapping
}

export interface ChatRequest {
	conversationId: string | null;
	parentId: number;
	model: string;
	content: string;
	webSearch?: boolean;
	attachedFileIds?: string[];
}

export interface ChatResponse {
	messages: Record<number, Message>;
}
export interface RetryResponse {
	messages: Record<number, Message>;
}

export interface UpdateRequest {
	conversationId: string;
	messageId: number;
	content: string;
}

export interface UpdateResponse {
	messages: Record<number, Message>;
}

// Tool call types
export interface ToolCall {
	id: string;
	reference_id?: string;
	name: string;
	args?: string;
	tool_output?: string;
}

// Streaming types
export interface StreamMetadata {
	conversationId: string;
	userMessageId: number;
}

export interface StreamChunk {
	content?: string;
	reasoning?: string;
	tool_call?: ToolCall;
}

export interface StreamStats {
	// PromptTokens or Context Size
	PromptTokens?: number;
	// CompletionTokens or Response message size
	CompletionTokens?: number;
	// Tokens per second
	Speed?: number;
}

export interface StreamComplete {
	userMessageId: number;
	assistantMessageId: number;
	streamStats?: StreamStats;
}
// Frontend types for compatibility
export interface FrontendMessage {
	id: string;
	role: "user" | "assistant";
	model?: string;
	content: string;
	reasoning?: string;
	reasoningDuration?: number; // Duration in seconds for reasoning (if reasoning was used)
	toolCalls?: ToolCall[];
	status?: "completed" | "pending";
	error?: string;
	timestamp: number;
	attachments?: Attachment[];
	// Optional frontend-facing metadata
	speed?: number;
	tokenCount?: number;
	contextSize?: number;
}

// Note: Backend now uses UUIDs. When creating a new conversation implicitly,
// read the real UUID from the returned messages' convId. - removed
// Convert backend message to frontend message
export const backendToFrontendMessage = (
	backendMsg: Message,
): FrontendMessage => {
	// Safety checks for null/undefined message
	if (!backendMsg || typeof backendMsg !== "object") {
		console.error("Invalid backend message provided:", backendMsg);
		return {
			id: "error",
			role: "assistant",
			content: "Error: Invalid message data",
			status: "completed",
			error: "Invalid message data",
			timestamp: Date.now(),
		};
	}

	// Validate required fields
	if (!backendMsg.role) {
		console.error("Backend message missing valid role:", backendMsg);
	}

	return {
		id: (backendMsg.id || "unknown").toString(),
		role:
			backendMsg.role === "user" || backendMsg.role === "assistant"
				? backendMsg.role
				: "assistant",
		content: backendMsg.content || "",
		reasoning: backendMsg.reasoning,
		toolCalls: backendMsg.tools,
		status: backendMsg.status === "pending" ? "pending" : "completed",
		error: backendMsg.error,
		timestamp: Date.now(), // Backend doesn't provide timestamp, use current time
		attachments: backendMsg.attachments,
		model: backendMsg.model,
		// map backend metadata into frontend message
		speed: backendMsg.speed,
		tokenCount: backendMsg.tokenCount,
		contextSize: backendMsg.contextSize,
	};
};

// Provider API Types
export interface ProviderRequest {
	base_url: string;
	api_key: string;
}

export interface ProviderResponse {
	id: string;
	base_url: string;
}

export interface Model {
	id: string; // provider id + name (for quick finding) e.g: provider-123/meta/llama-3b

	name: string; // original name from provider

	provider: string; // provider id

	is_enabled: boolean; // whether the model is enabled (shown/usable)
}

export interface ModelsResponse {
	models: Model[];
}

// Settings API Types
export interface Settings {
	settings: Record<string, string>;
}

// Frontend types for providers
export interface FrontendProvider {
	id: string;
	name: string;
	baseUrl: string;
}

// File upload types
export interface FileUploadResponse extends File {}

// MCP Server types
export interface MCPServerRequest {
	id?: string;
	name: string;
	endpoint: string;
	api_key: string;
}

export interface MCPServerResponse {
	id: string;
	name: string;
	endpoint: string;
	tools: Tool[];
}
// Tool types
export interface Tool {
	id: string;
	mcp_server_id?: string;
	name: string;
	description?: string;
	input_schema?: Record<string, any>;
	require_approval?: boolean;
	is_enabled?: boolean;
}

export interface ToolListResponse {
	tools: Tool[];
}
