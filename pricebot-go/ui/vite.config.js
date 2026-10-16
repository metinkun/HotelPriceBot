import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// /admin/ altinda servis edilir; derleme ciktisi Go'ya gomulur.
export default defineConfig({
  plugins: [react()],
  base: "/admin/",
  build: { outDir: "dist", emptyOutDir: true },
  server: { proxy: { "/api": "http://localhost:3000" } },
});
