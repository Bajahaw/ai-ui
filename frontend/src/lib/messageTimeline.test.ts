import { describe, expect, it } from "vitest";
import type { FrontendMessage, ToolCall } from "@/lib/api/types";
import {
  buildMessageTimeline,
  getMessageFullText,
  type TimelineSegment,
} from "./messageTimeline";

const msg = (overrides: Partial<FrontendMessage>): FrontendMessage => ({
  id: "1",
  role: "assistant",
  content: "",
  status: "completed",
  timestamp: 0,
  ...overrides,
});

const call = (id: string, extra: Partial<ToolCall> = {}): ToolCall => ({
  id,
  name: `tool_${id}`,
  args: "{}",
  tool_output: "ok",
  ...extra,
});

// Compact shape: "bar[r,t1]" / "text:A" / "final:C" (+ "*" when live).
const shape = (segments: TimelineSegment[]) =>
  segments.map((s) =>
    s.kind === "steps"
      ? `bar[${s.steps
          .map((st) =>
            st.kind === "tool"
              ? st.toolCall.id
              : `r${st.isStreaming ? "*" : ""}`,
          )
          .join(",")}]${s.isStreaming ? "*" : ""}`
      : `${s.isFinal ? "final" : "text"}:${s.text}`,
  );

describe("buildMessageTimeline", () => {
  it("keeps the classic single bar for a message without tools", () => {
    expect(
      shape(buildMessageTimeline(msg({ reasoning: "R", content: "C" }))),
    ).toEqual(["bar[r]", "final:C"]);
    expect(shape(buildMessageTimeline(msg({ content: "C" })))).toEqual([
      "final:C",
    ]);
  });

  it("opens a new bar after each text and merges text-less rounds", () => {
    const message = msg({
      content: "C",
      reasoning: "R3",
      toolCalls: [
        call("t1", { text: "A", reasoning: "R1" }),
        call("t2"),
        call("t3", { reasoning: "R2" }), // round without text: same bar
        call("t4", { text: "B" }),
      ],
    });
    expect(shape(buildMessageTimeline(message))).toEqual([
      "bar[r]",
      "text:A",
      "bar[t1,t2,r,t3]",
      "text:B",
      "bar[t4,r]",
      "final:C",
    ]);
  });

  it("renders legacy messages (merged content, no per-call text) as before", () => {
    const message = msg({
      content: "everything merged",
      reasoning: "R",
      toolCalls: [call("t1"), call("t2")],
    });
    expect(shape(buildMessageTimeline(message))).toEqual([
      "bar[t1,t2,r]",
      "final:everything merged",
    ]);
  });

  it("marks only the trailing bar live while the final text is empty", () => {
    const thinking = msg({
      status: "pending",
      reasoning: "R2",
      toolCalls: [call("t1", { text: "A" })],
    });
    expect(shape(buildMessageTimeline(thinking))).toEqual([
      "text:A",
      "bar[t1,r*]*",
      "final:",
    ]);

    const calling = msg({
      status: "pending",
      toolCalls: [call("t1", { text: "A", tool_output: undefined })],
    });
    expect(shape(buildMessageTimeline(calling))).toEqual([
      "text:A",
      "bar[t1]*",
      "final:",
    ]);

    const writing = msg({
      status: "pending",
      content: "C",
      toolCalls: [call("t1", { text: "A" })],
    });
    expect(shape(buildMessageTimeline(writing))).toEqual([
      "text:A",
      "bar[t1]",
      "final:C",
    ]);
  });

  it("attaches live round durations to their reasoning step", () => {
    const [bar] = buildMessageTimeline(
      msg({
        content: "C",
        roundReasoningDurations: { t1: 4 },
        toolCalls: [call("t1", { reasoning: "R1" })],
      }),
    );
    expect(bar.kind === "steps" && bar.steps[0]).toMatchObject({
      kind: "reasoning",
      duration: 4,
    });
  });
});

describe("getMessageFullText", () => {
  it("joins round texts and the final content in order", () => {
    expect(
      getMessageFullText(
        msg({
          content: "C",
          toolCalls: [call("t1", { text: "A" }), call("t2"), call("t3", { text: "B" })],
        }),
      ),
    ).toBe("A\n\nB\n\nC");
    expect(getMessageFullText(msg({ content: "only" }))).toBe("only");
  });
});
