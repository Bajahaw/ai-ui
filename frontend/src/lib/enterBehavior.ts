import { isLargeScreen } from "./viewport";

export type EnterAction = "send" | "newline";
export type EnterBehaviorSetting = EnterAction | "dynamic";

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
