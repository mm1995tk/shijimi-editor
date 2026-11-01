import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

// Tauri は dev のとき devUrl（tauri.conf.json）の固定ポートに繋ぎに来るので、strictPort にする。
export default defineConfig({
  plugins: [solid()],
  clearScreen: false,
  server: { port: 1420, strictPort: true },
  build: { target: "es2022" },
});
