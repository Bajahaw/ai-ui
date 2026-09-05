import { ApiErrorHandler } from "@/lib/api/errorHandler";

export function isBrowserOffline(): boolean {
  return typeof navigator !== "undefined" && navigator.onLine === false;
}

export function isOfflineError(error: unknown): boolean {
  if (isBrowserOffline()) {
    return true;
  }
  return ApiErrorHandler.isNetworkError(error);
}

export function shouldSurfaceConversationsError(
  error: unknown,
  hasLocalData: boolean,
): boolean {
  if (hasLocalData || isOfflineError(error)) {
    return false;
  }
  return true;
}
