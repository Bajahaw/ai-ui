/** The app's mobile breakpoint (Tailwind `md`); also used for sidebar/touch handling. */
export const LARGE_SCREEN_QUERY = "(min-width: 768px)";

export function isLargeScreen(): boolean {
  return typeof window !== "undefined" && typeof window.matchMedia === "function"
    ? window.matchMedia(LARGE_SCREEN_QUERY).matches
    : true;
}
