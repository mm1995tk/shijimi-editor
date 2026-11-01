// マシンごとの色（D4）。見出しと帯の左端の線、帯のアイコン、フォーカス中の帯の地に使う。
//
// 決め方：初めて一覧に出たマシンに、色の候補のうち使っている数がいちばん少ないもの（同じなら候補の順で先）を
// 割り当てる。ホスト名から計算すると、マシンが 2〜3 台でも同じ色になることがあり見分けられないため。
// 割り当ては webview の localStorage に残し、再起動しても同じ色にする（消えたら割り当て直すだけ）。
// 記録の形は、つなぎ先（ssh のホスト名か LOCAL）→ 候補の番号の JSON オブジェクト。

import type { KeyValueStore } from "../ssh/last.ts";

/**
 * 色の候補。最初の 3 つは親 #271 のモック（「案：フラットな一覧」）の色。
 * 残りは、暗い地（#181b21）の上で同じくらいの明るさに見えるものを足した。
 */
export const MACHINE_COLORS: readonly string[] = [
  "#8fb4ff", // 青
  "#e5b567", // 黄土
  "#c49cff", // 紫
  "#7cc4b0", // 青緑
  "#f28b82", // 赤
  "#f0a3d0", // 桃
  "#a5d6a7", // 緑
  "#6fd3e0", // 水色
];

/** localStorage のキー。値はつなぎ先 → 候補の番号の JSON オブジェクト。 */
export const COLORS_KEY = "shijimi-editor.machineColors";

/** 記録を読む。無い・壊れている・読めないときは空。範囲外の番号は捨てる。 */
export function loadMachineColors(
  store: KeyValueStore | undefined,
): Record<string, number> {
  try {
    const raw = store?.getItem(COLORS_KEY);
    if (!raw) return {};
    const v: unknown = JSON.parse(raw);
    if (typeof v !== "object" || v === null || Array.isArray(v)) return {};
    const out: Record<string, number> = {};
    for (const [k, n] of Object.entries(v)) {
      if (Number.isInteger(n) && n >= 0 && n < MACHINE_COLORS.length) {
        out[k] = n as number;
      }
    }
    return out;
  } catch {
    return {};
  }
}

/** 使っている数がいちばん少ない候補（同じなら番号の小さいほう）。 */
function leastUsed(assigned: Record<string, number>): number {
  const count = MACHINE_COLORS.map(() => 0);
  for (const n of Object.values(assigned)) count[n]++;
  let best = 0;
  for (let i = 1; i < count.length; i++) if (count[i] < count[best]) best = i;
  return best;
}

/**
 * targets の色を返す（つなぎ先 → 色）。記録に無いものには targets の順に割り当て、記録に足して保存する。
 * 記録にあるものは変えない（つないでいないマシンの割り当ても残し、次につないだとき同じ色にする）。
 */
export function machineColors(
  store: KeyValueStore | undefined,
  targets: string[],
): Record<string, string> {
  const assigned = loadMachineColors(store);
  let changed = false;
  for (const t of targets) {
    if (assigned[t] !== undefined) continue;
    assigned[t] = leastUsed(assigned);
    changed = true;
  }
  if (changed) {
    try {
      store?.setItem(COLORS_KEY, JSON.stringify(assigned));
    } catch {
      // 保存できなくても、今回は色が付く
    }
  }
  return Object.fromEntries(
    targets.map((t) => [t, MACHINE_COLORS[assigned[t]]]),
  );
}
