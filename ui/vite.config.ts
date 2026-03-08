import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/v1": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/healthz": { target: "http://localhost:8080", changeOrigin: true },
      "/readyz": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./test/setup.ts"],
    exclude: ["e2e/**", "node_modules/**"],
  },
});
