export type SandboxInputFile = {
  name: string;
  id?: string;
  data: string;
};

/** Keys a mounted file can be read by: original name, file id, and id+extension. */
export function fileAliases(name: string, id?: string): string[] {
  const keys: string[] = [];
  const seen = new Set<string>();
  const push = (k: string) => {
    if (!k) return;
    if (seen.has(k)) return;
    seen.add(k);
    keys.push(k);
  };
  if (name) {
    push(name);
    // Defensive: strip any directory components if a full path slipped in.
    const base = name.split(/[\\/]/).pop();
    if (base && base !== name) push(base);
  }
  const trimmedId = (id || "").trim();
  if (trimmedId) {
    push(trimmedId);
    const dot = name.lastIndexOf(".");
    if (dot > 0 && dot < name.length - 1) {
      const ext = name.slice(dot);
      if (!trimmedId.toLowerCase().endsWith(ext.toLowerCase())) {
        push(trimmedId + ext);
      }
    }
  }
  return keys;
}
