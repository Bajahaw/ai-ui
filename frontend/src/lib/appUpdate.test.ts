import { afterEach, describe, expect, it, vi } from "vitest";
import {
  applyAppUpdate,
  checkRemoteBuild,
  forceApplyAppUpdate,
  isNewerBuild,
  notifyAppUpdateAvailable,
  resetAppUpdateForTests,
  setAppUpdateApplier,
  subscribeAppUpdate,
} from "./appUpdate";

afterEach(() => {
  resetAppUpdateForTests();
});

describe("isNewerBuild", () => {
  it("ignores empty current or remote builds", () => {
    expect(isNewerBuild("", "1")).toBe(false);
    expect(isNewerBuild("1", "")).toBe(false);
    expect(isNewerBuild("1", undefined)).toBe(false);
  });

  it("is true only when both exist and differ", () => {
    expect(isNewerBuild("1", "1")).toBe(false);
    expect(isNewerBuild("1", "2")).toBe(true);
  });
});

describe("app update prompt", () => {
  it("does not apply until the subscriber asks", () => {
    const apply = vi.fn();
    setAppUpdateApplier(apply);
    const seen: boolean[] = [];
    const unsubscribe = subscribeAppUpdate((value) => seen.push(value));

    notifyAppUpdateAvailable();
    notifyAppUpdateAvailable();
    expect(seen).toEqual([false, true]);
    expect(apply).not.toHaveBeenCalled();

    applyAppUpdate();
    applyAppUpdate();
    expect(apply).toHaveBeenCalledOnce();
    unsubscribe();
  });

  it("notifies from a newer remote build_id without applying", async () => {
    const apply = vi.fn();
    setAppUpdateApplier(apply);
    const seen: boolean[] = [];
    subscribeAppUpdate((value) => seen.push(value));

    const fetchImpl = vi.fn(async () => ({
      ok: true,
      json: async () => ({ build_id: "new", build: "new" }),
    })) as unknown as typeof fetch;

    await expect(checkRemoteBuild("old", fetchImpl)).resolves.toBe(true);
    expect(seen).toEqual([false, true]);
    expect(apply).not.toHaveBeenCalled();
  });

  it("ignores the legacy build field so old loops cannot restart", async () => {
    const fetchImpl = vi.fn(async () => ({
      ok: true,
      json: async () => ({ build: "new" }),
    })) as unknown as typeof fetch;

    await expect(checkRemoteBuild("old", fetchImpl)).resolves.toBe(false);
  });

  it("clears the service worker and caches before reloading", async () => {
    const unregister = vi.fn(async () => true);
    const deleteCache = vi.fn(async () => true);
    const reload = vi.fn();

    await forceApplyAppUpdate({
      sw: {
        getRegistrations: async () =>
          [{ unregister } as unknown as ServiceWorkerRegistration],
      },
      cacheStorage: {
        keys: async () => ["workbox-precache"],
        delete: deleteCache,
      },
      reload,
    });

    expect(unregister).toHaveBeenCalledOnce();
    expect(deleteCache).toHaveBeenCalledWith("workbox-precache");
    expect(reload).toHaveBeenCalledOnce();
  });

  it("reloads even if cache teardown fails", async () => {
    const reload = vi.fn();
    await forceApplyAppUpdate({
      sw: {
        getRegistrations: async () => {
          throw new Error("blocked");
        },
      },
      reload,
    });
    expect(reload).toHaveBeenCalledOnce();
  });
});
