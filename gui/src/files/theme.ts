// テーマ（theme.get・theme.changed）で、意味の名前を色にする。
// shj は色を fg にだけ入れる（同じ名前を文字にも背景にも使うため。設計書 3 章の B9 の細部）。

import type { Theme } from "../rpc/protocol.ts";
import type { Run } from "./layout.ts";

/** テーマがまだ無いときの色（モックの色）。 */
const FALLBACK: Record<string, string> = {
  "editor.bg": "#121418",
  "editor.fg": "#e6e8ec",
  lineno: "#7d8696",
  "lineno.current": "#e6e8ec",
  curline: "#1a1d23",
  selection: "#2c3a55",
  cursor: "#e6e8ec",
  "cursor.fg": "#121418",
};

/** name の色。テーマに無ければ控えの色、それも無ければ undefined。 */
export function colorOf(
  theme: Theme | null | undefined,
  name: string | undefined,
): string | undefined {
  if (!name) return undefined;
  return theme?.tokens[name]?.fg || FALLBACK[name];
}

/** 区間の CSS（文字の色・背景・太字など）。行の背景と標準の文字色は親が持つので、違うときだけ付ける。 */
export function runStyle(
  theme: Theme | null | undefined,
  r: Run,
): Record<string, string> {
  const st: Record<string, string> = { width: `${r.cells}ch` };
  if (r.cursor) {
    st.color = colorOf(theme, "cursor.fg") ?? "";
    st.background = colorOf(theme, "cursor") ?? "";
  } else {
    const fg = colorOf(theme, r.fg);
    const bg = colorOf(theme, r.bg);
    if (fg) st.color = fg;
    if (bg) st.background = bg;
  }
  if (r.bold) st["font-weight"] = "bold";
  if (r.italic) st["font-style"] = "italic";
  const deco = [r.underline && "underline", r.strike && "line-through"]
    .filter(Boolean)
    .join(" ");
  if (deco) st["text-decoration"] = deco;
  return st;
}
