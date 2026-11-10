import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { render, upload, cached } = vi.hoisted(() => ({
  render: vi.fn(),
  upload: vi.fn(),
  cached: vi.fn(),
}));
vi.mock("./screenshot", () => ({ renderDocumentScreenshot: render }));
vi.mock("@/lib/api/files", () => ({
  fileThumbnailUrl: (path: string) => `/thumbs/${path}.jpg`,
  fileResourceUrl: (path: string) => `/${path}`,
  uploadFileThumbnail: upload,
  cachedFileBytes: cached,
}));
vi.mock("@/lib/api/headers", () => ({ getSessionId: () => "test-session" }));
const file = {
  id: "one",
  path: "one.docx",
  name: "Report.docx",
  type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
};
let fetchFile: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.resetModules();
  render
    .mockReset()
    .mockResolvedValue(new Blob(["png"], { type: "image/png" }));
  upload.mockReset().mockResolvedValue(undefined);
  cached.mockReset().mockReturnValue(undefined);
  fetchFile = vi.fn(async (_url: string, options?: RequestInit) =>
    options?.method === "HEAD"
      ? { ok: false, status: 404 }
      : {
          ok: true,
          headers: new Headers(),
          arrayBuffer: async () => new ArrayBuffer(8),
        },
  );
  vi.stubGlobal("fetch", fetchFile);
});
afterEach(() => vi.unstubAllGlobals());

describe("externally persisted Office thumbnails", () => {
  it("serves an existing thumbnail without fetching or rendering the document", async () => {
    fetchFile.mockResolvedValue({ ok: true });
    const { acquireOfficeThumbnail } = await import("./office-thumbnail");
    const job = acquireOfficeThumbnail(file);
    await expect(job.promise).resolves.toBe("/thumbs/one.docx.jpg");
    job.release();
    expect(fetchFile).toHaveBeenCalledOnce();
    expect(render).not.toHaveBeenCalled();
    expect(upload).not.toHaveBeenCalled();
  });

  it("generates once on a cache miss, uploads separately, and deduplicates cards", async () => {
    const { acquireOfficeThumbnail } = await import("./office-thumbnail");
    const first = acquireOfficeThumbnail(file);
    const second = acquireOfficeThumbnail(file);
    expect(first.promise).toBe(second.promise);
    first.release();
    await second.promise;
    second.release();
    expect(render).toHaveBeenCalledOnce();
    expect(render).toHaveBeenCalledWith(
      expect.objectContaining({ filename: file.name }),
      { width: 360, height: 360 },
      expect.any(AbortSignal),
    );
    expect(upload).toHaveBeenCalledWith(
      file.id,
      expect.any(Blob),
      expect.any(AbortSignal),
    );
    const third = acquireOfficeThumbnail(file);
    await third.promise;
    third.release();
    expect(render).toHaveBeenCalledOnce();
    expect(fetchFile).toHaveBeenCalledTimes(2);
  });

  it("reuses upload bytes without downloading the original again", async () => {
    cached.mockReturnValue({ data: new ArrayBuffer(8) });
    const { acquireOfficeThumbnail } = await import("./office-thumbnail");
    const job = acquireOfficeThumbnail(file);
    await job.promise;
    job.release();
    expect(fetchFile).toHaveBeenCalledOnce();
    expect(render).toHaveBeenCalledOnce();
  });

  it("does not render on authorization or server failures and suppresses repeated failures", async () => {
    fetchFile.mockResolvedValue({ ok: false, status: 403 });
    const { acquireOfficeThumbnail } = await import("./office-thumbnail");
    for (let i = 0; i < 2; i++) {
      const job = acquireOfficeThumbnail(file);
      await expect(job.promise).rejects.toThrow("Could not check");
      job.release();
    }
    expect(fetchFile).toHaveBeenCalledOnce();
    expect(render).not.toHaveBeenCalled();
  });

  it("cancels a queued job when its final consumer leaves", async () => {
    const { acquireOfficeThumbnail } = await import("./office-thumbnail");
    const job = acquireOfficeThumbnail(file);
    const rejected = expect(job.promise).rejects.toThrow();
    job.release();
    await rejected;
    expect(fetchFile).not.toHaveBeenCalled();
    expect(render).not.toHaveBeenCalled();
  });
});
