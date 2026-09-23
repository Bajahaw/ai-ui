import { beforeEach, describe, expect, it, vi } from "vitest";
import type { NavigateFunction } from "react-router-dom";
import { goToNewChat, historyIdx, seedHomeUnderCurrentEntry } from "./history";

function resetHistory(path: string, state: unknown = null) {
  window.history.replaceState(state, "", path);
}

describe("historyIdx", () => {
  it("returns 0 when state has no idx", () => {
    resetHistory("/");
    expect(historyIdx()).toBe(0);
  });

  it("returns the idx stored in history state", () => {
    resetHistory("/c/abc", { idx: 3 });
    expect(historyIdx()).toBe(3);
  });
});

describe("seedHomeUnderCurrentEntry", () => {
  let lengthBefore: number;

  beforeEach(() => {
    lengthBefore = window.history.length;
  });

  it("seeds / under a conversation opened at the bottom of the stack", () => {
    resetHistory("/c/abc", { idx: 0, key: "k1", usr: null });

    seedHomeUnderCurrentEntry();

    expect(window.location.pathname).toBe("/c/abc");
    expect(window.history.state).toMatchObject({ idx: 1, key: "k1" });
    expect(window.history.length).toBe(lengthBefore + 1);
  });

  it("seeds / under / so the new-chat screen is never the bottom entry", () => {
    resetHistory("/", { idx: 0 });

    seedHomeUnderCurrentEntry();

    expect(window.location.pathname).toBe("/");
    expect(window.history.state.idx).toBe(1);
    expect(window.history.length).toBe(lengthBefore + 1);
  });

  it("preserves search and hash of the current entry", () => {
    resetHistory("/c/abc?x=1#frag", { idx: 0 });

    seedHomeUnderCurrentEntry();

    expect(window.location.pathname).toBe("/c/abc");
    expect(window.location.search).toBe("?x=1");
    expect(window.location.hash).toBe("#frag");
  });

  it("is a no-op when the stack already has depth", () => {
    resetHistory("/c/abc", { idx: 2 });

    seedHomeUnderCurrentEntry();

    expect(window.history.state.idx).toBe(2);
    expect(window.history.length).toBe(lengthBefore);
  });

  it("is idempotent across repeated mounts", () => {
    resetHistory("/", { idx: 0 });

    seedHomeUnderCurrentEntry();
    seedHomeUnderCurrentEntry();

    expect(window.history.state.idx).toBe(1);
    expect(window.history.length).toBe(lengthBefore + 1);
  });
});

describe("goToNewChat", () => {
  it("goes back one entry when there is history below", () => {
    resetHistory("/c/abc", { idx: 1 });
    const navigate = vi.fn() as unknown as NavigateFunction;

    goToNewChat(navigate);

    expect(navigate).toHaveBeenCalledWith(-1);
  });

  it("replaces with / when at the bottom of the stack", () => {
    resetHistory("/c/abc", { idx: 0 });
    const navigate = vi.fn() as unknown as NavigateFunction;

    goToNewChat(navigate);

    expect(navigate).toHaveBeenCalledWith("/", { replace: true });
  });
});
