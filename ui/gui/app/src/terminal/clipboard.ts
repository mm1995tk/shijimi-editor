import type { ClipboardItem, ClipboardLocation } from "coneko-term";

/**
 * 公式の clipboard-manager プラグインが OS に書けるのは、テキストか画像のどちらか 1 つ。
 * 両方あるときはテキストを選ぶ（どのアプリにも貼れる）。
 */
export type WritePlan =
  { kind: "text"; text: string } | { kind: "image"; bytes: Uint8Array };

function isText(mime: string): boolean {
  return mime.split(";")[0]!.trim().toLowerCase() === "text/plain";
}

function isImage(mime: string): boolean {
  return mime.split(";")[0]!.trim().toLowerCase().startsWith("image/");
}

/** 書き込みの項目から、OS に書くものを選ぶ。書けるものが無ければ undefined（対応しない）。 */
export function planWrite(items: ClipboardItem[]): WritePlan | undefined {
  const text = items.find((i) => isText(i.mime));
  if (text) {
    return {
      kind: "text",
      text: new TextDecoder("utf-8").decode(text.data),
    };
  }
  const image = items.find((i) => isImage(i.mime));
  if (image) return { kind: "image", bytes: image.data };
  return undefined;
}

/** OS に読ませられる種類。読み取りで答えるのはこの 2 つだけ。 */
export const READ_TEXT = "text/plain";
export const READ_IMAGE = "image/png";

/** 頼まれた種類（先に書いたものほど優先）のうち、渡せる最初のもの。 */
export function pickReadMime(
  wanted: string[],
  available: string[],
): string | undefined {
  for (const w of wanted) {
    const base = w.split(";")[0]!.trim().toLowerCase();
    if (available.includes(base)) return base;
  }
  return undefined;
}

/**
 * primary selection は OS のクリップボード機能（プラグイン）では扱えない。
 * 扱えない場所は、書き込みも読み取りも「対応しない」と答える。
 */
export function supportsLocation(location: ClipboardLocation): boolean {
  return location === "clipboard";
}
