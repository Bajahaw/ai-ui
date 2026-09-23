export type EnterAction = "send" | "newline";
export type EnterBehaviorSetting = EnterAction | "dynamic";

/** Matches the app's mobile breakpoint (see App.tsx sidebar/touch handling). */
export const LARGE_SCREEN_QUERY = "(min-width: 768px)";

export function isLargeScreen(): boolean {
  return typeof window !== "undefined" && typeof window.matchMedia === "function"
    ? window.matchMedia(LARGE_SCREEN_QUERY).matches
    : true;
}

/**
 * Resolve the configured Enter key setting into a concrete action.
 * "dynamic" sends on large screens and inserts a newline on small ones.
 */
export function resolveEnterAction(
  setting: string | undefined,
  largeScreen: boolean = isLargeScreen(),
): EnterAction {
  if (setting === "newline") return "newline";
  if (setting === "dynamic") return largeScreen ? "send" : "newline";
  return "send";
}
