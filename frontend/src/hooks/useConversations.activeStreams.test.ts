import { describe, expect, it } from "vitest";
import {
  type ActiveStream,
  endActiveStream,
  startActiveStream,
} from "./useConversations";

describe("active streams", () => {
  it("keeps streams in other conversations running", () => {
    const streams = new Map<string, ActiveStream>();
    const a = startActiveStream(streams, "a");
    const b = startActiveStream(streams, "b");

    expect(a.abort.signal.aborted).toBe(false);
    expect(b.abort.signal.aborted).toBe(false);
    expect(streams.get("a")).toBe(a);
    expect(streams.get("b")).toBe(b);
  });

  it("aborts the previous stream in the same conversation", () => {
    const streams = new Map<string, ActiveStream>();
    const first = startActiveStream(streams, "a");
    const second = startActiveStream(streams, "a");

    expect(first.abort.signal.aborted).toBe(true);
    expect(second.abort.signal.aborted).toBe(false);
    expect(streams.get("a")).toBe(second);
  });

  it("ending a superseded stream leaves the newer one registered", () => {
    const streams = new Map<string, ActiveStream>();
    const first = startActiveStream(streams, "a");
    const second = startActiveStream(streams, "a");

    endActiveStream(streams, "a", first);
    expect(streams.get("a")).toBe(second);

    endActiveStream(streams, "a", second);
    expect(streams.has("a")).toBe(false);
  });
});
