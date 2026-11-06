// ブラウザの欄（#556）の、Tauri の殻（src-tauri/src/browser.rs）の命令と通知。

import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";

import type { Bounds } from "./browser.ts";

/** 子の webview（欄の中身）を作る。リモートのマシンの localhost 系は、殻が ssh のポート転送越しに開く。 */
export function openBrowser(
  key: string,
  target: string,
  url: string,
  b: Bounds,
): Promise<void> {
  return invoke("browser_open", { key, target, url, ...b });
}

export function moveBrowser(key: string, b: Bounds): Promise<void> {
  return invoke("browser_bounds", { key, ...b });
}

export function showBrowser(key: string, visible: boolean): Promise<void> {
  return invoke("browser_visible", { key, visible });
}

export function goBrowser(
  key: string,
  target: string,
  url: string,
): Promise<void> {
  return invoke("browser_go", { key, target, url });
}

export function navBrowser(
  key: string,
  action: "back" | "forward" | "reload",
): Promise<void> {
  return invoke("browser_nav", { key, action });
}

export function closeBrowser(key: string): Promise<void> {
  return invoke("browser_close", { key });
}

/** 欄の中のページが変わったとき（読み込み開始・終了）の URL。返す関数で聞くのをやめる。 */
export async function onBrowserUrl(
  key: string,
  h: (url: string) => void,
): Promise<() => void> {
  return listen<{ key: string; url: string }>("browser-url", (e) => {
    if (e.payload.key === key) h(e.payload.url);
  });
}
