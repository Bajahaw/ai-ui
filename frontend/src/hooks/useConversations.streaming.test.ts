import { describe, expect, it } from "vitest";
import { ClientConversationManager } from "@/lib/clientConversationManager";
import { buildMessageTimeline } from "@/lib/messageTimeline";
import { StreamingState, createStreamingHandlers } from "./useConversations";

function setup() {
  const manager = new ClientConversationManager();
  const conv = manager.createConversation("hello");
  const assistant = conv.messages[1];
  const state = new StreamingState();
  const handlers = createStreamingHandlers(
    manager,
    conv.id,
    { current: assistant.id },
    state,
    () => {},
  );
  return { assistant, state, handlers };
}

describe("streaming rounds", () => {
  it("moves each round's text/reasoning onto its first tool call", () => {
    const { assistant, state, handlers } = setup();

    // Round 1: reasoning + text, two parallel calls.
    handlers.onReasoning("R1");
    handlers.onChunk("A");
    handlers.onToolCall({ id: "t1", name: "search", args: "{}" });
    handlers.onToolCall({ id: "t2", name: "fetch", args: "{}" });
    // Backend echoes executed calls (first carries the stored round text).
    handlers.onToolCall({ id: "t1", name: "search", tool_output: "o1", text: "A", reasoning: "R1" });
    handlers.onToolCall({ id: "t2", name: "fetch", tool_output: "o2" });

    // Round 2: reasoning only, one call.
    handlers.onReasoning("R2");
    handlers.onToolCall({ id: "t3", name: "search", args: "{}" });
    handlers.onToolCall({ id: "t3", name: "search", tool_output: "o3", reasoning: "R2" });

    // Final round.
    handlers.onChunk("C");

    expect(assistant.toolCalls).toEqual([
      { id: "t1", name: "search", args: "{}", tool_output: "o1", text: "A", reasoning: "R1" },
      { id: "t2", name: "fetch", args: "{}", tool_output: "o2" },
      { id: "t3", name: "search", args: "{}", tool_output: "o3", reasoning: "R2" },
    ]);
    expect(assistant.content).toBe("C");
    expect(assistant.reasoning).toBe("");
    expect(state.content).toBe("C");
    expect(state.reasoning).toBe("");

    const layout = buildMessageTimeline(assistant).map((s) =>
      s.kind === "steps"
        ? s.steps.map((st) => (st.kind === "tool" ? st.toolCall.id : "r")).join(",")
        : `text:${s.text}`,
    );
    expect(layout).toEqual(["r", "text:A", "t1,t2,r,t3", "text:C"]);
  });

  it("does not attach anything when a round has no text or reasoning", () => {
    const { assistant, handlers } = setup();
    handlers.onToolCall({ id: "t1", name: "search", args: "{}" });
    expect(assistant.toolCalls).toEqual([{ id: "t1", name: "search", args: "{}" }]);
  });
});
