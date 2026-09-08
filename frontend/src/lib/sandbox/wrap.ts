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

export function filesObjectLiteral(
  files: { name: string; data: string }[],
): string {
  if (files.length === 0) return "{}";
  return `{${files
    .map(
      (f) =>
        `${JSON.stringify(f.name)}:Uint8Array.from(atob(${JSON.stringify(f.data)}),c=>c.charCodeAt(0))`,
    )
    .join(",")}}`;
}
