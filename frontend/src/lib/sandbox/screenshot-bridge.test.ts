import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createScreenshotBridge } from "./screenshot-bridge";

const { render } = vi.hoisted(() => ({ render: vi.fn() }));
vi.mock("@/lib/document-rendering/screenshot", () => ({
  renderDocumentScreenshot: render,
}));

beforeEach(() => {
  render
    .mockReset()
    .mockResolvedValue({ arrayBuffer: async () => new ArrayBuffer(4) });
});
afterEach(() => vi.restoreAllMocks());
const request = (requestId = 1) => ({
  type: "__sandbox_screenshot",
  requestId,
  data: new ArrayBuffer(8),
  filename: "report.docx",
  options: { page: 2, width: 800 },
});

describe("modular sandbox screenshot bridge", () => {
  it("renders only supplied bytes and transfers the resulting PNG", async () => {
    const reply = vi.fn();
    const controller = new AbortController();
    const handle = createScreenshotBridge(reply, controller.signal);
    const input = request();
    expect(handle(input)).toBe(true);
    await vi.waitFor(() => expect(reply).toHaveBeenCalledOnce());
    expect(render).toHaveBeenCalledWith(
      { data: input.data, filename: input.filename },
      input.options,
      controller.signal,
    );
    expect(reply.mock.calls[0][0]).toMatchObject({
      type: "__sandbox_screenshot_result",
      requestId: 1,
      data: expect.any(ArrayBuffer),
    });
    expect(reply.mock.calls[0][1]).toEqual([reply.mock.calls[0][0].data]);
    handle(input);
    expect(render).toHaveBeenCalledOnce();
    expect(handle({ type: "__sandbox_done" })).toBe(false);
  });

  it("rejects URLs, oversized requests, and invalid option envelopes", () => {
    const reply = vi.fn();
    const handle = createScreenshotBridge(reply, new AbortController().signal);
    handle({ ...request(1), data: "https://private.example/report.docx" });
    handle({ ...request(2), data: new ArrayBuffer(0) });
    handle({ ...request(3), options: [] });
    expect(render).not.toHaveBeenCalled();
    expect(reply).toHaveBeenCalledTimes(3);
    for (const [message] of reply.mock.calls)
      expect(message.error).toBe("Invalid screenshot request");
  });

  it("caps requests per run and reports renderer errors", async () => {
    const reply = vi.fn();
    render.mockRejectedValue(new Error("Page number is out of range"));
    const handle = createScreenshotBridge(reply, new AbortController().signal);
    for (let i = 1; i <= 7; i++) handle(request(i));
    await vi.waitFor(() => expect(reply).toHaveBeenCalledTimes(7));
    expect(render).toHaveBeenCalledTimes(6);
    expect(reply).toHaveBeenCalledWith(
      expect.objectContaining({
        requestId: 7,
        error: "Maximum 6 screenshots per sandbox run",
      }),
      undefined,
    );
    expect(reply).toHaveBeenCalledWith(
      expect.objectContaining({
        requestId: 1,
        error: "Page number is out of range",
      }),
      undefined,
    );
  });

  it("does not reply to a finished or cancelled sandbox", async () => {
    const reply = vi.fn();
    const controller = new AbortController();
    const handle = createScreenshotBridge(reply, controller.signal);
    handle(request());
    controller.abort();
    await Promise.resolve();
    await Promise.resolve();
    handle(request(2));
    expect(reply).not.toHaveBeenCalled();
    expect(render).toHaveBeenCalledOnce();
  });
});
