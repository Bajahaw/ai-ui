type Listener = (available: boolean) => void;

export type ApplyUpdateDeps = {
  sw?: Pick<ServiceWorkerContainer, "getRegistrations">;
  cacheStorage?: Pick<CacheStorage, "keys" | "delete">;
  reload: () => void;
};

let available = false;
let applying = false;
const listeners = new Set<Listener>();
let apply = () => {
  void forceApplyAppUpdate();
};

export function isNewerBuild(
  current: string,
  remote?: string | null,
): boolean {
  return Boolean(current && remote && remote !== current);
}

export function subscribeAppUpdate(listener: Listener): () => void {
  listeners.add(listener);
  listener(available);
  return () => {
    listeners.delete(listener);
  };
}

export function notifyAppUpdateAvailable(): void {
  if (available) return;
  available = true;
  for (const listener of listeners) listener(true);
}

export function setAppUpdateApplier(fn: () => void): void {
  apply = fn;
}

export function applyAppUpdate(): void {
  if (applying) return;
  applying = true;
  apply();
}

export function resetAppUpdateForTests(): void {
  available = false;
  applying = false;
  listeners.clear();
  apply = () => {
    void forceApplyAppUpdate();
  };
}

export async function forceApplyAppUpdate(
  deps: ApplyUpdateDeps = {
    sw: typeof navigator !== "undefined" ? navigator.serviceWorker : undefined,
    cacheStorage: typeof caches !== "undefined" ? caches : undefined,
    reload: () => window.location.reload(),
  },
): Promise<void> {
  try {
    if (deps.sw) {
      const registrations = await deps.sw.getRegistrations();
      await Promise.all(registrations.map((registration) => registration.unregister()));
    }
    if (deps.cacheStorage) {
      const keys = await deps.cacheStorage.keys();
      await Promise.all(keys.map((key) => deps.cacheStorage!.delete(key)));
    }
  } catch {
    // still drop the old page
  }
  deps.reload();
}

export async function checkRemoteBuild(
  currentBuild: string,
  fetchImpl: typeof fetch = fetch,
): Promise<boolean> {
  if (!currentBuild) return false;
  try {
    const response = await fetchImpl("/api/version", { cache: "no-store" });
    if (!response.ok) return false;
    const data = (await response.json()) as { build_id?: string };
    if (isNewerBuild(currentBuild, data.build_id)) {
      notifyAppUpdateAvailable();
      return true;
    }
  } catch {
    // ignore network errors
  }
  return false;
}
