export function wrapSandboxCode(code: string): string {
  const trimmed = code.trim();
  if (!trimmed) return "";
  if (trimmed.startsWith("<")) return trimmed;
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
