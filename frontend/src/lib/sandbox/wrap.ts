const HTML_TAG_PATTERN =
  /^<(?:!doctype|html|head|body|div|span|p|a|button|canvas|table|ul|ol|li|section|article|header|footer|main|form|input|img|svg|link|meta|style|script|h[1-6])[\s>/!]/i;

export function isSandboxHtml(code: string): boolean {
  return HTML_TAG_PATTERN.test(code.trim());
}

export function wrapSandboxCode(code: string): string {
  const trimmed = code.trim();
  if (!trimmed) return "";
  // Bare JS starting with `<` (e.g. `a < b`) must not be mistaken for HTML;
  // only real markup passes through unwrapped.
  if (isSandboxHtml(trimmed)) return trimmed;
  const safe = trimmed.replace(/<\/script/gi, "<\\/script");
  return `<script>
(async () => {
  try {
    ${safe}
    await Promise.resolve();
  } catch (e) {
    sandbox.done(String(e && e.message ? e.message : e));
    return;
  }
  if (!sandbox._done) sandbox.done();
})();
</script>`;
}

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

export function filesObjectLiteral(files: SandboxInputFile[]): string {
  if (files.length === 0) return "{}";
  const decls: string[] = [];
  const entries: string[] = [];
  files.forEach((f, i) => {
    const v = `__d${i}`;
    decls.push(
      `var ${v}=Uint8Array.from(atob(${JSON.stringify(f.data)}),c=>c.charCodeAt(0))`,
    );
    const keys = fileAliases(f.name, f.id);
    if (keys.length === 0) {
      // Fallback: never emit an empty entry set for a file.
      entries.push(`${JSON.stringify(f.name)}:${v}`);
    } else {
      for (const key of keys) {
        entries.push(`${JSON.stringify(key)}:${v}`);
      }
    }
  });
  return `(function(){${decls.join(";")};return {${entries.join(",")}}})()`;
}
