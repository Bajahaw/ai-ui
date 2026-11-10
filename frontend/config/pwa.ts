import type { Manifest, Plugin } from "vite";

/** Keep static startup imports/CSS, but not optional dynamic imports or fonts. */
export function appShellFiles(manifest: Manifest): Set<string> {
  const files = new Set<string>();
  const visited = new Set<string>();

  const visit = (key: string) => {
    if (visited.has(key)) return;
    visited.add(key);
    const chunk = manifest[key];
    if (!chunk) throw new Error(`Missing app shell chunk: ${key}`);
    files.add(chunk.file);
    for (const css of chunk.css ?? []) files.add(css);
    for (const dependency of chunk.imports ?? []) visit(dependency);
  };

  visit("index.html");
  // The PWA registration helper is dynamically imported, but still needed at
  // startup (including an offline reload), unlike optional feature chunks.
  for (const key of Object.keys(manifest)) {
    if (/(?:^|\/)workbox-window\//.test(key)) visit(key);
  }
  return files;
}

export function woff2Only(css: string): string {
  return css.replace(/src:\s*([^;{}]+)/g, (_declaration, sources: string) => {
    const woff2 = sources.split(",").find((source) =>
      /format\(["']?woff2["']?\)/.test(source),
    );
    if (!woff2) throw new Error("KaTeX font is missing a WOFF2 source");
    return `src:${woff2.trim()}`;
  });
}

/** Remove legacy font references before Vite emits the referenced assets. */
export function katexWoff2Only(): Plugin {
  return {
    name: "katex-woff2-only",
    enforce: "pre",
    transform(code, id) {
      const normalizedId = id.replaceAll("\\", "/");
      if (!/\/katex\/dist\/katex(?:\.min)?\.css(?:\?|$)/.test(normalizedId)) {
        return;
      }
      return { code: woff2Only(code), map: null };
    },
  };
}
