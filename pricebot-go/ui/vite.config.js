import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

// /admin/ altinda servis edilir; derleme ciktisi Go'ya gomulur. Ortak tasarim sistemi: ../ui-kit (@kit)
export default defineConfig({
  plugins: [react()],
  base: "/admin/",
  resolve: { alias: { "@kit": path.resolve(__dirname, "../ui-kit/src") } },
  server: { proxy: { "/api": "http://localhost:3000", "/health": "http://localhost:3000", "/docs": "http://localhost:3000" }, fs: { allow: [path.resolve(__dirname, "..")] } },
  build: { outDir: "dist", emptyOutDir: true },
});
