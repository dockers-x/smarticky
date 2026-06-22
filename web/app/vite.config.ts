/// <reference types="vitest/config" />

import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [svelte()],
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
