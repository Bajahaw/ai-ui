import type { NavigateFunction } from "react-router-dom";

export function historyIdx(): number {
  return typeof window.history.state?.idx === "number"
    ? window.history.state.idx
    : 0;
}

export function goToNewChat(navigate: NavigateFunction) {
  if (historyIdx() > 0) {
    navigate(-1);
    return;
  }
  navigate("/", { replace: true });
}

/**
 * Put "/" at the bottom of the history stack, under whatever entry the app was
 * opened on. Run once at mount; it is a no-op once the stack has depth.
 *
 * - Opening /c/:id directly (reload, deep link) should still let Back return
 *   to the new-chat screen rather than leave the app.
 * - Chrome's standalone PWA shell only registers its Android back handler once
 *   the tab can go back. If that first happens via pushState while the soft
 *   keyboard is open, the handler is registered above the IME's and the next
 *   Back press navigates instead of closing the keyboard. Seeding at launch
 *   means the handler exists before any keyboard can open.
 */
export function seedHomeUnderCurrentEntry(): void {
  if (historyIdx() > 0) {
    return;
  }
  const { pathname, search, hash } = window.location;
  const state = window.history.state ?? {};
  window.history.replaceState({ ...state, idx: 0 }, "", "/");
  window.history.pushState(
    { ...state, idx: 1 },
    "",
    `${pathname}${search}${hash}`,
  );
}
