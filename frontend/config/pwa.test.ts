import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { describe, expect, it } from "vitest";
import { appShellFiles, woff2Only } from "./pwa.ts";

describe("app shell precache", () => {
  it("includes static imports and their CSS, without lazy chunks or fonts", () => {
    expect(appShellFiles({
      "index.html": {
        file: "assets/app.js",
        isEntry: true,
        imports: ["_shared.js"],
        dynamicImports: ["language.ts"],
        css: ["assets/app.css"],
        assets: ["assets/KaTeX_Main.woff2"],
      },
      "_shared.js": {
        file: "assets/shared.js",
        imports: ["_runtime.js"],
        css: ["assets/shared.css"],
      },
      "_runtime.js": { file: "assets/runtime.js", imports: ["_shared.js"] },
      "language.ts": { file: "assets/python.js", css: ["assets/lazy.css"] },
    })).toEqual(new Set([
      "assets/app.js",
      "assets/app.css",
      "assets/shared.js",
      "assets/shared.css",
      "assets/runtime.js",
    ]));
  });

  it("fails the build rather than silently omitting missing startup chunks", () => {
    expect(() => appShellFiles({})).toThrow("Missing app shell chunk: index.html");
    expect(() => appShellFiles({
      "index.html": { file: "assets/app.js", imports: ["missing"] },
    })).toThrow("Missing app shell chunk: missing");
  });

  it("includes the dynamic PWA registration helper and its static imports", () => {
    expect(appShellFiles({
      "index.html": {
        file: "assets/app.js",
        dynamicImports: ["../node_modules/workbox-window/build/workbox-window.prod.es5.mjs"],
      },
      "../node_modules/workbox-window/build/workbox-window.prod.es5.mjs": {
        file: "assets/workbox-window.js",
        imports: ["_runtime.js"],
      },
      "_runtime.js": { file: "assets/runtime.js" },
    })).toEqual(new Set([
      "assets/app.js",
      "assets/workbox-window.js",
      "assets/runtime.js",
    ]));
  });
});

describe("KaTeX WOFF2-only CSS", () => {
  it("keeps WOFF2 and font properties while dropping legacy formats", () => {
    const css = '@font-face { font-family: "KaTeX_Main"; src: url(font.woff2) format("woff2"), url(font.woff) format("woff"), url(font.ttf) format("truetype"); font-weight: bold; }';
    expect(woff2Only(css)).toBe('@font-face { font-family: "KaTeX_Main"; src:url(font.woff2) format("woff2"); font-weight: bold; }');
  });

  it("handles minified CSS with no final semicolon", () => {
    expect(woff2Only("@font-face{src:url(font.woff) format('woff'),url(font.woff2) format('woff2')}"))
      .toBe("@font-face{src:url(font.woff2) format('woff2')}");
  });

  it("supports every font in the installed KaTeX stylesheet", () => {
    const require = createRequire(import.meta.url);
    const original = readFileSync(require.resolve("katex/dist/katex.min.css"), "utf8");
    const result = woff2Only(original);
    const faces = original.match(/@font-face/g)?.length;
    expect(faces).toBeGreaterThan(0);
    expect(result.match(/@font-face/g)).toHaveLength(faces!);
    expect(result.match(/format\(["']woff2["']\)/g)).toHaveLength(faces!);
    expect(result).not.toMatch(/\.(?:woff|ttf)["')]/);
  });

  it("fails explicitly if a future KaTeX font has no WOFF2 source", () => {
    expect(() => woff2Only("@font-face{src:url(font.ttf) format('truetype')}"))
      .toThrow("KaTeX font is missing a WOFF2 source");
  });
});
