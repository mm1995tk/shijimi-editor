// ツリーの右クリックのメニュー（項目の出し分け）と、プライマリターミナルに送る cd の文字、パスの組み立て。
// DOM を使わないので node --test で確かめられる。

import type { TreeRow } from "./model.ts";

/** メニューの項目。 */
export type TreeMenuAction =
  | "copy-name"
  | "copy-relative"
  | "copy-absolute"
  | "new-file"
  | "new-dir"
  | "rename"
  | "delete"
  | "cd";

/** 項目の文言。 */
export const treeMenuLabels: Record<TreeMenuAction, string> = {
  "copy-name": "名前をコピー",
  "copy-relative": "相対パスをコピー",
  "copy-absolute": "絶対パスをコピー",
  "new-file": "新しいファイル",
  "new-dir": "新しいフォルダ",
  rename: "名前の変更",
  delete: "削除",
  cd: "ここへ移動",
};

/** 行に出す項目（上から順）。ファイルはパスのコピーと名前の変更・削除、フォルダは新規作成と cd も。 */
export function menuItems(row: Pick<TreeRow, "dir">): TreeMenuAction[] {
  const common: TreeMenuAction[] = [
    "copy-name",
    "copy-relative",
    "copy-absolute",
  ];
  return row.dir
    ? [...common, "new-file", "new-dir", "rename", "delete", "cd"]
    : [...common, "rename", "delete"];
}

/** ルートのフォルダ（絶対パス）と、ルートからの相対パスをつなぐ。 */
export function absolutePath(root: string, rel: string): string {
  if (rel === "") return root;
  return `${root.endsWith("/") ? root : `${root}/`}${rel}`;
}

/** 相対パスの親（ルート直下なら ""）。 */
export function parentPath(rel: string): string {
  const i = rel.lastIndexOf("/");
  return i < 0 ? "" : rel.slice(0, i);
}

/** 新しいものの名前を入れる欄の初期値：フォルダの中を指す "dir/"（ルートなら ""）。 */
export function newEntryDefault(dir: string): string {
  return dir === "" ? "" : `${dir}/`;
}

/**
 * 新しいファイル・フォルダの入力を、ルートからの相対パスにする。空・"/" で終わる（名前が無い）ときは null。
 * 先頭の "./" と末尾の空白は落とす。
 */
export function newEntryPath(input: string): string | null {
  const v = input.trim().replace(/^(\.\/)+/, "");
  if (v === "" || v.endsWith("/")) return null;
  return v;
}

/** 名前の変更の入力を、新しい相対パスにする。空・変わっていないときは null。 */
export function renamedPath(old: string, input: string): string | null {
  const v = input.trim().replace(/^(\.\/)+/, "");
  if (v === "" || v === old || v.endsWith("/")) return null;
  return v;
}

/** シェルに渡す 1 語にする（単引用符で囲み、中の ' は '\'' にする。bash・zsh・fish で同じ意味）。 */
export function shellQuote(s: string): string {
  return `'${s.replaceAll("'", "'\\''")}'`;
}

/**
 * プライマリターミナルに送る cd。入力の途中の行に続けて実行されないよう、行末へ行って行頭まで消してから打つ
 * （Ctrl+E・Ctrl+U。bash・zsh・fish で効く。vim など、シェルでないものが前面にいるときは分からないので送ってしまう）。
 * path が undefined なら親のフォルダへ（cd ..）。
 */
export function cdInput(path?: string): string {
  const cmd = path === undefined ? "cd .." : `cd -- ${shellQuote(path)}`;
  return `\x05\x15${cmd}\r`;
}
