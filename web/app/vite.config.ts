/// <reference types="vitest/config" />

import { svelte } from "@sveltejs/vite-plugin-svelte";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";

const cspSafeLodashRoot = fileURLToPath(
  new URL("./src/lib/vendor/lodashRoot.ts", import.meta.url),
);
const cspSafeExcalidrawSubset = fileURLToPath(
  new URL("./src/lib/vendor/excalidrawSubset.ts", import.meta.url),
);
const cspSafeExcalidrawSubsetWorker = fileURLToPath(
  new URL("./src/lib/vendor/excalidrawSubsetWorker.ts", import.meta.url),
);

const cspGlobalProbePatterns = [
  /Function\("return this"\)\(\)/g,
  /Function\('return this'\)\(\)/g,
];

function isDependencyWithGlobalProbe(id: string): boolean {
  return (
    id.includes("/node_modules/mermaid/") ||
    id.includes("/node_modules/cytoscape/") ||
    id.includes("/node_modules/@excalidraw/excalidraw/") ||
    id.includes("/node_modules/lodash-es/")
  );
}

export default defineConfig({
  plugins: [
    {
      name: "smarticky-csp-safe-lodash-root",
      enforce: "pre",
      resolveId(source, importer) {
        if (source !== "./_root.js") return null;
        if (!importer?.includes("/node_modules/lodash-es/")) return null;
        return cspSafeLodashRoot;
      },
    },
    {
      name: "smarticky-csp-safe-vendor-probes",
      enforce: "pre",
      resolveId(source, importer) {
        if (!importer?.includes("/node_modules/@excalidraw/excalidraw/")) {
          return null;
        }
        if (source === "./subset-shared.chunk.js") {
          return cspSafeExcalidrawSubset;
        }
        if (source === "./subset-worker.chunk.js") {
          return cspSafeExcalidrawSubsetWorker;
        }
        return null;
      },
      transform(code, id) {
        if (!isDependencyWithGlobalProbe(id)) return null;

        let nextCode = code;
        for (const pattern of cspGlobalProbePatterns) {
          nextCode = nextCode.replace(pattern, "globalThis");
        }
        if (nextCode === code) return null;

        return {
          code: nextCode,
          map: null,
        };
      },
    },
    svelte(),
  ],
  base: "/static/app/",
  optimizeDeps: {
    include: ["mermaid", "@markdown-viewer/drawio2svg"],
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts"],
  },
  build: {
    outDir: "../static/app",
    emptyOutDir: true,
    rollupOptions: {
      output: {
        entryFileNames: "assets/index.js",
        chunkFileNames: "assets/[name].js",
        assetFileNames: (assetInfo) => {
          if (assetInfo.name?.endsWith(".css")) return "assets/index.css";
          return "assets/[name][extname]";
        },
        manualChunks: (id) => {
          if (
            id.includes("/node_modules/@codemirror/") ||
            id.includes("/node_modules/@lezer/") ||
            id.includes("/node_modules/crelt/") ||
            id.includes("/node_modules/style-mod/")
          ) {
            return "editor";
          }
        },
      },
    },
  },
});
