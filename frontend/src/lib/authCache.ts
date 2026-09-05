const AUTH_CACHE_KEY = "ai-ui-auth-cache";

export interface AuthCache {
  authenticated: boolean;
  userId?: string;
}

export function readAuthCache(): AuthCache | null {
  if (typeof localStorage === "undefined") {
    return null;
  }
  try {
    const raw = localStorage.getItem(AUTH_CACHE_KEY);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as AuthCache;
    if (typeof parsed?.authenticated !== "boolean") {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export function writeAuthCache(cache: AuthCache): void {
  if (typeof localStorage === "undefined") {
    return;
  }
  try {
    const prev = readAuthCache();
    const next: AuthCache = {
      authenticated: cache.authenticated,
      userId: cache.userId ?? prev?.userId,
    };
    localStorage.setItem(AUTH_CACHE_KEY, JSON.stringify(next));
  } catch {
    // Quota / private mode — ignore
  }
}

export function clearAuthCache(): void {
  if (typeof localStorage === "undefined") {
    return;
  }
  try {
    localStorage.removeItem(AUTH_CACHE_KEY);
  } catch {
    // ignore
  }
}
