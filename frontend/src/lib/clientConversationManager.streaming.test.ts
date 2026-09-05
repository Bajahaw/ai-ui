import { describe, expect, it } from "vitest";
import { ClientConversationManager } from "./clientConversationManager";
import type { Conversation, Message, ToolCall } from "@/lib/api";

function backendMessage(
  overrides: Partial<Message> &
    Pick<Message, "id" | "role" | "content" | "convId">,
): Message {
  return {
    status: "completed",
    children: [],
    ...overrides,
  };
}

function seedConversation(
  manager: ClientConversationManager,
  id = "conv-1",
  messages: Record<number, Message> = {},
): ReturnType<ClientConversationManager["getConversation"]> {
  const conversation: Conversation = {
    id,
    userId: "user-1",
    title: "Chat",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    messages: {},
  };
  manager.loadBackendConversations([conversation]);
  if (Object.keys(messages).length > 0) {
    manager.updateWithChatResponse(id, messages);
  }
  return manager.getConversation(id);
}

function priorTurn(convId = "conv-1"): Record<number, Message> {
  return {
    1: backendMessage({
      id: 1,
      convId,
      role: "user",
      content: "hi",
      children: [2],
    }),
    2: backendMessage({
      id: 2,
      convId,
      role: "assistant",
      content: "hey",
      parentId: 1,
    }),
  };
}

function startInFlightFollowUp(manager: ClientConversationManager) {
  const conv = seedConversation(manager, "conv-1", priorTurn())!;
  manager.addMessageOptimistically("conv-1", "follow up");

  const userTemp = conv.messages.find(
    (m) => m.role === "user" && conv.pendingMessageIds.has(m.id),
  )!;
  const assistantTemp = conv.messages.find(
    (m) => m.role === "assistant" && conv.pendingMessageIds.has(m.id),
  )!;
  const userTempId = userTemp.id;
  const assistantTempId = assistantTemp.id;

  userTemp.id = "3";
  userTemp.status = "completed";
  conv.pendingMessageIds.delete(userTempId);
  conv.pendingMessageIds.delete(assistantTempId);
  assistantTemp.id = "4";
  conv.pendingMessageIds.add("4");

  manager.updateMessageContent("conv-1", "4", "partial streamed text");
  const live = conv.messages.find((m) => m.id === "4")!;
  live.reasoning = "thinking...";
  manager.addToolCall("conv-1", "4", {
    id: "tc-1",
    name: "search",
    args: '{"q":"x"}',
  });

  return { conv, assistant: live };
}

function stalePendingTree(): Record<number, Message> {
  return {
    ...priorTurn(),
    3: backendMessage({
      id: 3,
      convId: "conv-1",
      role: "user",
      content: "follow up",
      parentId: 2,
      children: [4],
    }),
    4: backendMessage({
      id: 4,
      convId: "conv-1",
      role: "assistant",
      content: "",
      status: "pending",
      parentId: 3,
    }),
  };
}

describe("ClientConversationManager streaming invariants", () => {
  it("keeps the assistant pending while tokens arrive", () => {
    const manager = new ClientConversationManager();
    seedConversation(manager, "conv-1", priorTurn());
    manager.addMessageOptimistically("conv-1", "next");
    const conv = manager.getConversation("conv-1")!;
    const assistant = conv.messages.find(
      (m) => m.role === "assistant" && m.status === "pending",
    )!;

    manager.updateMessageContent("conv-1", assistant.id, "Hel");
    manager.updateMessageContent("conv-1", assistant.id, "Hello");

    expect(assistant.content).toBe("Hello");
    expect(assistant.status).toBe("pending");
    expect(conv.pendingMessageIds.has(assistant.id)).toBe(true);
  });

  it("does not wipe live streamed content when a stale backend fetch arrives mid-stream", () => {
    const manager = new ClientConversationManager();
    const { assistant } = startInFlightFollowUp(manager);

    manager.updateWithChatResponse("conv-1", stalePendingTree());

    expect(assistant.content).toBe("partial streamed text");
    expect(assistant.reasoning).toBe("thinking...");
    expect(assistant.status).toBe("pending");
    expect(assistant.toolCalls).toEqual([
      { id: "tc-1", name: "search", args: '{"q":"x"}' },
    ]);
  });

  it("does not wipe live streamed content on a single-message SSE stub", () => {
    const manager = new ClientConversationManager();
    const { assistant } = startInFlightFollowUp(manager);

    manager.updateWithChatResponse("conv-1", {
      4: backendMessage({
        id: 4,
        convId: "conv-1",
        role: "assistant",
        content: "",
        status: "pending",
        parentId: 3,
      }),
    });

    expect(assistant.content).toBe("partial streamed text");
    expect(assistant.status).toBe("pending");
    expect(assistant.toolCalls?.[0]?.name).toBe("search");
  });

  it("preserves the in-flight assistant when rebuilding the visible branch", () => {
    const manager = new ClientConversationManager();
    const { conv, assistant } = startInFlightFollowUp(manager);

    manager.updateWithChatResponse("conv-1", stalePendingTree());

    const visible = conv.messages.find((m) => m.id === "4");
    expect(visible).toBe(assistant);
    expect(visible?.content).toBe("partial streamed text");
    expect(conv.pendingMessageIds.has("4")).toBe(true);
  });

  it("does not match a real-id in-flight assistant against another assistant row", () => {
    const manager = new ClientConversationManager();
    const { assistant } = startInFlightFollowUp(manager);

    manager.updateWithChatResponse("conv-1", {
      2: backendMessage({
        id: 2,
        convId: "conv-1",
        role: "assistant",
        content: "hey",
        parentId: 1,
      }),
    });

    expect(assistant.id).toBe("4");
    expect(assistant.content).toBe("partial streamed text");
    expect(assistant.status).toBe("pending");
  });

  it("merges tool call updates by id without dropping earlier fields", () => {
    const manager = new ClientConversationManager();
    const { assistant } = startInFlightFollowUp(manager);

    manager.addToolCall("conv-1", "4", {
      id: "tc-1",
      name: "search",
      tool_output: "result",
    } as ToolCall);

    expect(assistant.toolCalls).toEqual([
      { id: "tc-1", name: "search", args: '{"q":"x"}', tool_output: "result" },
    ]);
  });

  it("applies a completed backend message after the stream is no longer in-flight", () => {
    const manager = new ClientConversationManager();
    const { conv, assistant } = startInFlightFollowUp(manager);
    conv.pendingMessageIds.delete("4");
    assistant.status = "completed";

    manager.updateWithChatResponse("conv-1", {
      4: backendMessage({
        id: 4,
        convId: "conv-1",
        role: "assistant",
        content: "final saved reply",
        parentId: 3,
        speed: 12,
        tokenCount: 40,
      }),
    });

    const visible = conv.messages.find((m) => m.id === "4")!;
    expect(visible.content).toBe("final saved reply");
    expect(visible.status).toBe("completed");
  });
});

describe("recover / abandonInFlightStreams", () => {
  it("drops temp-id rows and clears pending ids", () => {
    const manager = new ClientConversationManager();
    const conv = manager.createConversation("new chat");
    const tempIds = conv.messages.map((m) => m.id);
    expect(tempIds.every((id) => isNaN(parseInt(id, 10)))).toBe(true);

    manager.abandonInFlightStreams();

    expect(conv.pendingMessageIds.size).toBe(0);
    expect(conv.messages).toEqual([]);
  });

  it("keeps real-id rows so a refetch can attach to them", () => {
    const manager = new ClientConversationManager();
    const { conv, assistant } = startInFlightFollowUp(manager);

    manager.abandonInFlightStreams();

    expect(conv.pendingMessageIds.size).toBe(0);
    expect(conv.messages.find((m) => m.id === "4")).toBe(assistant);
    expect(conv.messages.every((m) => !isNaN(parseInt(m.id, 10)))).toBe(true);
  });

  it("lets a completed backend save replace abandoned in-flight state", () => {
    const manager = new ClientConversationManager();
    startInFlightFollowUp(manager);
    manager.abandonInFlightStreams();

    manager.updateWithChatResponse("conv-1", {
      ...stalePendingTree(),
      4: backendMessage({
        id: 4,
        convId: "conv-1",
        role: "assistant",
        content: "full reply after disconnect",
        reasoning: "done",
        parentId: 3,
        tools: [{ id: "tc-1", name: "search", args: '{"q":"x"}' }],
      }),
    });

    const visible = manager.getConversation("conv-1")!.messages.find(
      (m) => m.id === "4",
    )!;
    expect(visible.content).toBe("full reply after disconnect");
    expect(visible.status).toBe("completed");
    expect(visible.reasoning).toBe("done");
  });

  it("shows backend pending after abandon instead of a stuck live buffer", () => {
    const manager = new ClientConversationManager();
    const { assistant } = startInFlightFollowUp(manager);
    manager.abandonInFlightStreams();

    manager.updateWithChatResponse("conv-1", stalePendingTree());

    const visible = manager.getConversation("conv-1")!.messages.find(
      (m) => m.id === "4",
    )!;
    expect(visible).not.toBe(assistant);
    expect(visible.content).toBe("");
    expect(visible.status).toBe("pending");
  });

  it("does not delete a conversation that still has pending messages", () => {
    const manager = new ClientConversationManager();
    const conv = manager.createConversation("draft");

    manager.loadBackendConversations([]);

    expect(manager.getConversation(conv.id)).toBe(conv);
  });

  it("can drop a local-only stub after abandon, once nothing is pending", () => {
    const manager = new ClientConversationManager();
    const conv = manager.createConversation("draft");
    manager.abandonInFlightStreams();

    manager.loadBackendConversations([]);

    expect(manager.getConversation(conv.id)).toBeUndefined();
  });

  it("rekey is a no-op if abandon + list refresh already removed the stub", () => {
    const manager = new ClientConversationManager();
    const conv = manager.createConversation("draft");
    const oldId = conv.id;
    manager.abandonInFlightStreams();
    manager.loadBackendConversations([]);

    manager.rekeyConversation(oldId, "real-uuid");

    expect(manager.getConversation(oldId)).toBeUndefined();
    expect(manager.getConversation("real-uuid")).toBeUndefined();
  });
});

describe("optimistic send / id swap", () => {
  it("createConversation starts with pending user and assistant placeholders", () => {
    const manager = new ClientConversationManager();
    const conv = manager.createConversation("hello world");

    expect(conv.messages).toHaveLength(2);
    expect(conv.messages[0]).toMatchObject({
      role: "user",
      content: "hello world",
      status: "pending",
    });
    expect(conv.messages[1]).toMatchObject({
      role: "assistant",
      content: "",
      status: "pending",
    });
    expect(conv.pendingMessageIds.size).toBe(2);
  });

  it("rekeyConversation keeps the same object under the backend id", () => {
    const manager = new ClientConversationManager();
    const conv = manager.createConversation("hello");
    const oldId = conv.id;

    manager.rekeyConversation(oldId, "uuid-1");

    expect(manager.getConversation(oldId)).toBeUndefined();
    expect(manager.getConversation("uuid-1")).toBe(conv);
    expect(conv.id).toBe("uuid-1");
  });

  it("markAssistantFailed completes the placeholder without deleting it", () => {
    const manager = new ClientConversationManager();
    const conv = manager.createConversation("hello");
    const assistant = conv.messages[1];

    manager.markAssistantFailed(conv.id, assistant.id, "network down");

    expect(assistant.status).toBe("completed");
    expect(assistant.error).toBe("network down");
    expect(conv.pendingMessageIds.has(assistant.id)).toBe(false);
  });

  it("hydrated messages stay stale until a network fetch marks them fresh", () => {
    const manager = new ClientConversationManager();
    seedConversation(manager, "conv-1", priorTurn());

    expect(manager.hasLoadedMessages("conv-1")).toBe(true);
    expect(manager.hasFreshMessages("conv-1")).toBe(false);

    manager.markMessagesFresh("conv-1");
    expect(manager.hasFreshMessages("conv-1")).toBe(true);

    manager.markMessagesStale("conv-1");
    expect(manager.hasFreshMessages("conv-1")).toBe(false);
  });

  it("persistable snapshot omits temp ids and client-only fields", () => {
    const manager = new ClientConversationManager();
    seedConversation(manager, "conv-1", priorTurn());
    manager.createConversation("unsaved");

    const snapshot = manager.getPersistableSnapshot();
    expect(snapshot.userId).toBe("user-1");
    expect(snapshot.conversations.map((c) => c.id)).toEqual(["conv-1"]);
    expect(snapshot.conversations[0].messages).toEqual({});
    expect(snapshot.messagesById["conv-1"][1].content).toBe("hi");
  });

  it("clear and delete drop freshness so SSE cannot patch a stale tree", () => {
    const manager = new ClientConversationManager();
    seedConversation(manager, "conv-1", priorTurn());
    manager.markMessagesFresh("conv-1");
    manager.handleExternalDelete("conv-1");
    expect(manager.hasFreshMessages("conv-1")).toBe(false);
  });
});
