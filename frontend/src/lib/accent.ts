export const ACCENTS = [
  { id: "neutral", label: "Neutral" },
  { id: "blue", label: "Blue" },
  { id: "green", label: "Green" },
  { id: "yellow", label: "Yellow" },
  { id: "pink", label: "Pink" },
  { id: "orange", label: "Orange" },
  { id: "purple", label: "Purple" },
] as const;

export type AccentId = (typeof ACCENTS)[number]["id"];

export const DEFAULT_ACCENT: AccentId = "neutral";
export const ACCENT_SETTING_KEY = "accentColor";
export const ACCENT_STORAGE_KEY = "ai-ui-accent";

export function isAccentId(value: unknown): value is AccentId {
  return ACCENTS.some((a) => a.id === value);
}

/** Resolve a stored/user-provided value to a known accent, falling back to the default. */
export function resolveAccent(value: unknown): AccentId {
  return isAccentId(value) ? value : DEFAULT_ACCENT;
}

/** Apply an accent to the document root. Neutral is the stylesheet default, so it clears the attribute. */
export function applyAccentToDocument(
  accent: AccentId,
  root: HTMLElement = document.documentElement,
) {
  if (accent === DEFAULT_ACCENT) delete root.dataset.accent;
  else root.dataset.accent = accent;
}
