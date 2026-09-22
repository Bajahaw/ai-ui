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

  it("does not re-prompt for a remote build we already tried to apply", async () => {
    const apply = vi.fn();
    setAppUpdateApplier(apply);
    const fetchImpl = vi.fn(async () => ({
      ok: true,
      json: async () => ({ build_id: "new" }),
    })) as unknown as typeof fetch;

    await expect(checkRemoteBuild("old", fetchImpl)).resolves.toBe(true);
    applyAppUpdate();
    expect(apply).toHaveBeenCalledOnce();

    // Simulate the page that comes back after the reload, still on "old".
    resetAppUpdateForTests();
    sessionStorage.setItem("ai-ui:update-attempted-build", "new");
    const seen: boolean[] = [];
    subscribeAppUpdate((value) => seen.push(value));
    await expect(checkRemoteBuild("old", fetchImpl)).resolves.toBe(false);
    expect(seen).toEqual([false]);

    // A genuinely different build prompts again.
    const newer = vi.fn(async () => ({
      ok: true,
      json: async () => ({ build_id: "newer" }),
    })) as unknown as typeof fetch;
    await expect(checkRemoteBuild("old", newer)).resolves.toBe(true);
    expect(seen).toEqual([false, true]);
  });

  it("promotes a waiting worker instead of wiping caches", async () => {
    const unregister = vi.fn(async () => true);
    const deleteCache = vi.fn(async () => true);
    const reload = vi.fn();
    const listeners = new Set<() => void>();
    const postMessage = vi.fn(() => {
      // Worker skips waiting and takes control.
      queueMicrotask(() => listeners.forEach((fn) => fn()));
    });

    await forceApplyAppUpdate({
      sw: {
        getRegistrations: async () => [{ unregister, waiting: { postMessage } }],
        addEventListener: (_type: string, fn: unknown) => {
          listeners.add(fn as () => void);
        },
        removeEventListener: (_type: string, fn: unknown) => {
          listeners.delete(fn as () => void);
        },
      },
      cacheStorage: { keys: async () => ["workbox-precache"], delete: deleteCache },
      reload,
    });

    expect(postMessage).toHaveBeenCalledWith({ type: "SKIP_WAITING" });
    expect(unregister).not.toHaveBeenCalled();
    expect(deleteCache).not.toHaveBeenCalled();
    expect(reload).toHaveBeenCalledOnce();
    expect(listeners.size).toBe(0);
  });

  it("falls back to unregister and cache wipe if the waiting worker never takes control", async () => {
    const unregister = vi.fn(async () => true);
    const deleteCache = vi.fn(async () => true);
    const reload = vi.fn();
    const postMessage = vi.fn();

    await forceApplyAppUpdate({
      sw: {
        getRegistrations: async () => [{ unregister, waiting: { postMessage } }],
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      },
      cacheStorage: { keys: async () => ["workbox-precache"], delete: deleteCache },
      reload,
      controllerTimeoutMs: 10,
    });

    expect(postMessage).toHaveBeenCalledOnce();
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
