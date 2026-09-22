type Listener = (available: boolean) => void;

type WaitingWorker = Pick<ServiceWorker, "postMessage">;

type Registration = Pick<ServiceWorkerRegistration, "unregister"> & {
  waiting?: WaitingWorker | null;
};

export type ApplyUpdateDeps = {
  sw?: {
    getRegistrations: () => Promise<readonly Registration[]>;
    addEventListener?: ServiceWorkerContainer["addEventListener"];
    removeEventListener?: ServiceWorkerContainer["removeEventListener"];
  };
  cacheStorage?: Pick<CacheStorage, "keys" | "delete">;
  reload: () => void;
  /** How long to wait for a promoted worker to take control before falling back. */
  controllerTimeoutMs?: number;
};

const ATTEMPTED_BUILD_KEY = "ai-ui:update-attempted-build";

let available = false;
let applying = false;
let pendingRemoteBuild: string | null = null;
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
  if (pendingRemoteBuild) rememberAttemptedBuild(pendingRemoteBuild);
  apply();
}

export function resetAppUpdateForTests(): void {
  available = false;
  applying = false;
  pendingRemoteBuild = null;
  listeners.clear();
  apply = () => {
    void forceApplyAppUpdate();
  };
  try {
    sessionStorage.removeItem(ATTEMPTED_BUILD_KEY);
  } catch {
    // no storage in this environment
  }
}

function rememberAttemptedBuild(build: string): void {
  try {
    sessionStorage.setItem(ATTEMPTED_BUILD_KEY, build);
  } catch {
    // storage unavailable, we just may prompt again
  }
}

function alreadyAttemptedBuild(build: string): boolean {
  try {
    return sessionStorage.getItem(ATTEMPTED_BUILD_KEY) === build;
  } catch {
    return false;
  }
}

function waitForController(
  sw: NonNullable<ApplyUpdateDeps["sw"]>,
  waiting: WaitingWorker,
  timeoutMs: number,
): Promise<boolean> {
  if (!sw.addEventListener || !sw.removeEventListener) {
    return Promise.resolve(false);
  }
  return new Promise((resolve) => {
    const finish = (took: boolean) => {
      clearTimeout(timer);
      sw.removeEventListener!("controllerchange", onChange);
      resolve(took);
    };
    const onChange = () => finish(true);
    const timer = setTimeout(() => finish(false), timeoutMs);
    sw.addEventListener!("controllerchange", onChange);
    waiting.postMessage({ type: "SKIP_WAITING" });
  });
}

export async function forceApplyAppUpdate(
  deps: ApplyUpdateDeps = {
    sw:
      typeof navigator !== "undefined" && navigator.serviceWorker
        ? {
            getRegistrations: () => navigator.serviceWorker.getRegistrations(),
            addEventListener: navigator.serviceWorker.addEventListener.bind(
              navigator.serviceWorker,
            ),
            removeEventListener:
              navigator.serviceWorker.removeEventListener.bind(
                navigator.serviceWorker,
              ),
          }
        : undefined,
    cacheStorage: typeof caches !== "undefined" ? caches : undefined,
    reload: () => window.location.reload(),
  },
): Promise<void> {
  try {
    if (deps.sw) {
      const registrations = await deps.sw.getRegistrations();
      // Preferred path: a newer worker is already installed and waiting. Promoting
      // it lets its activate step prune the old precache, and the reload is then
      // served by a worker whose cache is complete. Nuking caches while the old
      // worker can still be resurrected is what produced ERR_FAILED navigations.
      const waiting = registrations
        .map((registration) => registration.waiting)
        .find((worker): worker is WaitingWorker => Boolean(worker));
      if (
        waiting &&
        (await waitForController(deps.sw, waiting, deps.controllerTimeoutMs ?? 5000))
      ) {
        deps.reload();
        return;
      }
      await Promise.all(
        registrations.map((registration) => registration.unregister()),
      );
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
      // We already reloaded for this exact remote build and still run the old
      // bundle (stale proxy/CDN, mixed replicas). Prompting again would only loop.
      if (alreadyAttemptedBuild(data.build_id!)) return false;
      pendingRemoteBuild = data.build_id!;
      notifyAppUpdateAvailable();
      return true;
    }
  } catch {
    // ignore network errors
  }
  return false;
}
