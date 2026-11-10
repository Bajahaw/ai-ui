import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";
import { readFile } from "node:fs/promises";
import path from "path";
import { fileURLToPath } from "node:url";
import { viteStaticCopy } from "vite-plugin-static-copy";
import packageJson from "./package.json" with { type: "json" };
import pdfjsPackageJson from "pdfjs-dist/package.json" with { type: "json" };
import { appShellFiles, katexWoff2Only } from "./config/pwa.ts";
import type { Manifest } from "vite";

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(packageJson.version),
    __APP_BUILD__: JSON.stringify(process.env.VITE_BUILD_ID?.trim() ?? ""),
  },
  plugins: [
    react(),
    katexWoff2Only(),
    viteStaticCopy({
      targets: [
        ...["cmaps", "standard_fonts", "wasm"].map((directory) => ({
          src: `${fileURLToPath(new URL(`../${directory}`, import.meta.resolve("pdfjs-dist"))).replaceAll("\\", "/")}/*`,
          dest: `pdfjs/${directory}`,
          rename: { stripBase: true as const },
        })),
        {
          src: fileURLToPath(
            new URL("../LICENSE", import.meta.resolve("pdfjs-dist")),
          ).replaceAll("\\", "/"),
          dest: "pdfjs",
          rename: { stripBase: true },
        },
      ],
    }),
    VitePWA({
      registerType: "prompt",
      injectRegister: false,
      manifest: false,
      includeAssets: [
        "favicon.ico",
        "favicon-32x32.png",
        "favicon-192x192.png",
        "favicon.svg",
      ],
      workbox: {
        cleanupOutdatedCaches: true,
        globPatterns: ["index.html", "manifest.json", "assets/**/*.{js,css}"],
        // Vite's manifest describes the static startup graph. Dynamic imports
        // (languages, diagrams, viewers) and fonts are fetched only when used.
        manifestTransforms: [
          async (entries) => {
            const manifest = JSON.parse(
              await readFile(
                path.resolve(import.meta.dirname, "dist/.vite/manifest.json"),
                "utf8",
              ),
            ) as Manifest;
            const shell = appShellFiles(manifest);
            return {
              manifest: entries.filter(
                (entry) => !entry.url.startsWith("assets/") || shell.has(entry.url),
              ),
              warnings: [],
            };
          },
        ],
        navigateFallback: "/index.html",
        navigateFallbackDenylist: [
          /^\/api\//,
          /^\/data\//,
          /^\/sandbox\.html$/,
        ],
        maximumFileSizeToCacheInBytes: 6 * 1024 * 1024,
        runtimeCaching: [
          {
            urlPattern: /\/api\//,
            handler: "NetworkOnly",
          },
          {
            urlPattern: ({ url, sameOrigin }) =>
              sameOrigin && url.pathname.startsWith("/assets/"),
            handler: "CacheFirst",
            options: {
              cacheName: "app-assets-v1",
              cacheableResponse: { statuses: [200] },
              expiration: { maxEntries: 512, maxAgeSeconds: 60 * 24 * 60 * 60 },
            },
          },
          {
            urlPattern: ({ url, sameOrigin }) =>
              sameOrigin && url.pathname.startsWith("/pdfjs/"),
            handler: "CacheFirst",
            options: {
              // These URLs aren't hashed, so isolate them by PDF.js version.
              cacheName: `pdfjs-assets-${pdfjsPackageJson.version}`,
              cacheableResponse: { statuses: [200] },
              expiration: { maxEntries: 256, maxAgeSeconds: 60 * 24 * 60 * 60 },
            },
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  server: {
    port: 3000,
    host: true,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/data/resources": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    manifest: true,
    sourcemap: true,
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts", "config/**/*.test.ts"],
  },
});
