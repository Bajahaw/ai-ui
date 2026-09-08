import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  deleteFile,
  getFileBytes,
  resetFileBytesCache,
  retainFileBytes,
  uploadFile,
} from "./files";

function meta(id: string, name = `${id}.txt`) {
  return {
    id,
    name,
    type: "text/plain",
    size: 3,
    path: `/files/${id}`,
    content: "",
    createdAt: "",
  };
}

describe("file bytes cache", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    resetFileBytesCache();
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
    fetchMock.mockImplementation(async (url: RequestInfo | URL) => {
      const u = String(url);
      if (u.startsWith("/api/files/") && !u.includes("upload") && !u.includes("delete")) {
        const id = u.slice("/api/files/".length);
        return {
          ok: true,
          json: async () => meta(id),
        };
      }
      if (u.startsWith("/api/files/delete/")) {
        return { ok: true };
      }
      if (u === "/api/files/upload") {
        return {
          ok: true,
          json: async () => meta("up1", "local.txt"),
        };
      }
      return {
        ok: true,
        arrayBuffer: async () =>
          new TextEncoder().encode(u).buffer as ArrayBuffer,
      };
    });
  });

  afterEach(() => {
    resetFileBytesCache();
    vi.unstubAllGlobals();
  });

  it("fetches bytes once and reuses the cached buffer", async () => {
    const first = await getFileBytes("a");
    const second = await getFileBytes("a");
    expect(first.data).toBe(second.data);
    expect(fetchMock.mock.calls.filter((c) => String(c[0]) === "/files/a")).toHaveLength(
      1,
    );
  });

  it("drops bytes that are not retained", async () => {
    await getFileBytes("a");
    retainFileBytes(["b"]);
    fetchMock.mockClear();
    await getFileBytes("a");
    expect(fetchMock.mock.calls.some((c) => String(c[0]) === "/files/a")).toBe(
      true,
    );
  });

  it("caches upload bytes so sandbox does not re-download", async () => {
    const local = new File(["hello"], "local.txt", { type: "text/plain" });
    await uploadFile(local);
    fetchMock.mockClear();
    const cached = await getFileBytes("up1");
    expect(new TextDecoder().decode(cached.data)).toBe("hello");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("evicts bytes on delete", async () => {
    await getFileBytes("a");
    await deleteFile("a");
    fetchMock.mockClear();
    await getFileBytes("a");
    expect(fetchMock.mock.calls.some((c) => String(c[0]) === "/files/a")).toBe(
      true,
    );
  });
});
