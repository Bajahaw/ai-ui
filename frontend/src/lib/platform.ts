/** Human-readable modifier label: "⌘" on Apple platforms, otherwise "Ctrl+". */
export const modKeyLabel =
  typeof navigator !== "undefined" &&
  /Mac|iPhone|iPad/.test(navigator.userAgent)
    ? "⌘"
    : "Ctrl+";
