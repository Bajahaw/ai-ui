import { beforeEach, describe, expect, it } from "vitest";
import type { Conversation, Message } from "./api/types";
import {
  conversationCache,
  resetConversationCacheForTests,
} from "./conversationCache";

function conv(id: string, userId = "user-1"): Conversation {
  return {
    id,
    userId,
    title: `Chat ${id}`,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T00:00:00Z",
    messages: {},
  };
}

function msg(id: number, convId: string): Message {
  return {
    id,
    convId,
    role: id % 2 === 1 ? "user" : "assistant",
    content: `m${id}`,
    status: "completed",
    children: [],
  };
}

describe("conversationCache", () => {
  beforeEach(async () => {
    resetConversationCacheForTests();
    await conversationCache.clear();
  });

  it("stores list metadata and messages separately", async () => {
    await conversationCache.replaceList("user-1", [conv("a"), conv("b")]);
    await conversationCache.saveMessages("user-1", "a", {
      1: msg(1, "a"),
      2: msg(2, "a"),
    });
    await conversationCache.saveStats("user-1", {
      totalTokens: 10,
      totalInputTokens: 4,
      totalConversations: 2,
      totalMessages: 2,
    });

    const snapshot = await conversationCache.load("user-1");
    expect(snapshot?.conversations.map((c) => c.id).sort()).toEqual(["a", "b"]);
    expect(snapshot?.messagesById.a[1].content).toBe("m1");
    expect(snapshot?.messagesById.b).toBeUndefined();
    expect(snapshot?.stats?.totalConversations).toBe(2);
  });

  it("drops conversations removed from a later list replace", async () => {
    await conversationCache.replaceAll({
      userId: "user-1",
      conversations: [conv("a"), conv("b")],
      messagesById: {
        a: { 1: msg(1, "a") },
        b: { 1: msg(1, "b") },
      },
    });
    await conversationCache.replaceList("user-1", [conv("a")]);

    const snapshot = await conversationCache.load("user-1");
    expect(snapshot?.conversations.map((c) => c.id)).toEqual(["a"]);
    expect(snapshot?.messagesById.b).toBeUndefined();
    expect(snapshot?.messagesById.a[1].content).toBe("m1");
  });

  it("does not leak another user's conversations", async () => {
    await conversationCache.replaceList("user-1", [conv("a", "user-1")]);
    await conversationCache.replaceList("user-2", [conv("z", "user-2")]);

    const first = await conversationCache.load("user-1");
    const second = await conversationCache.load("user-2");
    expect(first?.conversations.map((c) => c.id)).toEqual(["a"]);
    expect(second?.conversations.map((c) => c.id)).toEqual(["z"]);
  });

  it("clears a single user without wiping the other", async () => {
    await conversationCache.replaceList("user-1", [conv("a", "user-1")]);
    await conversationCache.replaceList("user-2", [conv("z", "user-2")]);
    await conversationCache.clear("user-1");

    expect(await conversationCache.load("user-1")).toBeNull();
    expect((await conversationCache.load("user-2"))?.conversations[0].id).toBe(
      "z",
    );
  });
});
