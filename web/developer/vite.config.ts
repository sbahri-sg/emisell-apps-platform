import { defineConfig } from "vite";
import vinext from "vinext";
import tailwindcss from "@tailwindcss/postcss";
import { fileURLToPath } from "node:url";

export default defineConfig({
  resolve: {
    alias: { "@": fileURLToPath(new URL("../dashboard", import.meta.url)) },
    dedupe: ["react", "react-dom", "@base-ui/react"],
  },
  css: { postcss: { plugins: [tailwindcss()] } },
  server: {
    host: "localhost",
    port: 4319,
    strictPort: true,
    fs: { allow: [fileURLToPath(new URL("..", import.meta.url))] },
    proxy: { "/api/v1/developer": { target: "http://127.0.0.1:8087", changeOrigin: false } },
  },
  plugins: [vinext()],
});
