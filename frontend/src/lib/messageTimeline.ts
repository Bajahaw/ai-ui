import type { FrontendMessage, ToolCall } from "@/lib/api/types";

export type TimelineStep =
  | {
      kind: "reasoning";
      text: string;
      duration?: number;
      isStreaming: boolean;
    }
  | { kind: "tool"; toolCall: ToolCall };

export type TimelineSegment =
  | { kind: "steps"; steps: TimelineStep[]; isStreaming: boolean }
  | { kind: "text"; text: string; isFinal: boolean };

const hasText = (s?: string): s is string => Boolean(s?.trim());

/**
 * Lays an assistant message out in generation order. Each round is
 * reasoning -> text -> tool calls; consecutive reasoning/tool steps share one
 * bar and assistant text splits bars. The last segment is always the final
 * (editable) text, even when empty.
 */
export function buildMessageTimeline(
  message: FrontendMessage,
): TimelineSegment[] {
  const segments: TimelineSegment[] = [];
  let steps: TimelineStep[] = [];

  const flush = () => {
    if (steps.length > 0) {
      segments.push({ kind: "steps", steps, isStreaming: false });
      steps = [];
    }
  };

  for (const toolCall of message.toolCalls ?? []) {
    if (hasText(toolCall.reasoning)) {
      steps.push({
        kind: "reasoning",
        text: toolCall.reasoning,
        duration: message.roundReasoningDurations?.[toolCall.id],
        isStreaming: false,
      });
    }
    if (hasText(toolCall.text)) {
      flush();
      segments.push({ kind: "text", text: toolCall.text, isFinal: false });
    }
    steps.push({ kind: "tool", toolCall });
  }

  if (hasText(message.reasoning)) {
    steps.push({
      kind: "reasoning",
      text: message.reasoning,
      duration: message.reasoningDuration,
      isStreaming: false,
    });
  }

  // Only the bar right before a still-empty final text can be live.
  const live = message.status === "pending" && !hasText(message.content);
  if (live && steps.length > 0) {
    const last = steps[steps.length - 1];
    if (last.kind === "reasoning") last.isStreaming = true;
    segments.push({ kind: "steps", steps, isStreaming: true });
    steps = [];
  }
  flush();

  segments.push({ kind: "text", text: message.content, isFinal: true });
  return segments;
}

/** All assistant text of a message in display order (copy, read-aloud). */
export function getMessageFullText(message: FrontendMessage): string {
  return [
    ...(message.toolCalls ?? []).map((tc) => tc.text),
    message.content,
  ]
    .filter(hasText)
    .join("\n\n");
}
