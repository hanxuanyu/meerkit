import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(process.cwd(), "src") }
  },
  build: {
    outDir: "dist",
    emptyOutDir: true
  },
  server: {
    // Bind IPv4 as well as localhost so both localhost:5173 and 127.0.0.1:5173 work.
    host: "0.0.0.0",
    port: 5173,
    proxy: {
      "/api": { target: "http://127.0.0.1:8080", changeOrigin: false, ws: true },
      "/healthz": { target: "http://127.0.0.1:8080", changeOrigin: false }
    }
  }
});
