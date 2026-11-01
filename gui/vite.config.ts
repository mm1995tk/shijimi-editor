import { defineConfig, type Plugin } from "vite";
import solid from "vite-plugin-solid";

// @fontsource の CSS は woff2 と、古いブラウザ向けの woff を並べて書いている。
// WebView（WebKitGTK・WKWebView・WebView2）はどれも woff2 を読むので、woff は使われないのに同梱されて重くなる。
// woff の方を CSS から消し、dist に入らないようにする。
function dropWoffFallback(): Plugin {
  return {
    name: "drop-woff-fallback",
    enforce: "pre",
    transform(code, id) {
      if (!/[\\/]@fontsource[\\/].*\.css$/.test(id)) return;
      return code.replace(/,\s*url\([^)]*\.woff\)\s*format\('woff'\)/g, "");
    },
  };
}

// Tauri は dev のとき devUrl（tauri.conf.json）の固定ポートに繋ぎに来るので、strictPort にする。
export default defineConfig({
  plugins: [dropWoffFallback(), solid()],
  clearScreen: false,
  server: { port: 1420, strictPort: true },
  build: { target: "es2022" },
});
