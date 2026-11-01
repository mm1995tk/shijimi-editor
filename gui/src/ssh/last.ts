// 前回ワークスペースがあったマシンの記録（C5）。起動時にここへだけ自動でつなぐ。
// 記録の形は、つなぎ先（ssh のホスト名か LOCAL）の配列を JSON にしたもの。
// 置き場所は webview の localStorage（Tauri のアプリごとに残る。消えても自動でつながないだけ）。

/** 読み書きの窓口。localStorage と同じ形の最小限。テストで差し替える。 */
export interface KeyValueStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

export const LAST_KEY = "shijimi-editor.lastTargets";

/** 記録を読む。無い・壊れている・読めないときは空。 */
export function loadLastTargets(store: KeyValueStore | undefined): string[] {
  try {
    const raw = store?.getItem(LAST_KEY);
    if (!raw) return [];
    const v: unknown = JSON.parse(raw);
    if (!Array.isArray(v)) return [];
    return v.filter((x): x is string => typeof x === "string");
  } catch {
    return [];
  }
}

/**
 * 観測した結果を記録に反映する。ワークスペースがあれば加え、無くなれば外す。
 * つなげなかった・切れた・閉じたでは呼ばない（観測していないものは変えない）。
 */
export function recordWorkspaces(
  store: KeyValueStore | undefined,
  target: string,
  workspaceCount: number,
): void {
  const cur = loadLastTargets(store);
  const has = cur.includes(target);
  const next =
    workspaceCount > 0
      ? has
        ? cur
        : [...cur, target]
      : cur.filter((t) => t !== target);
  if (
    next === cur ||
    (next.length === cur.length && has === workspaceCount > 0)
  )
    return;
  try {
    store?.setItem(LAST_KEY, JSON.stringify(next));
  } catch {
    // 保存できなくても動く
  }
}

/** 起動時に自動でつなぐ先。記録にあり、いま選べるもの（LOCAL か一覧にあるホスト）だけ。 */
export function autoConnectTargets(
  recorded: string[],
  available: string[],
): string[] {
  const ok = new Set(available);
  return recorded.filter((t) => ok.has(t));
}

/** 本物の localStorage（使えない環境では undefined）。 */
export function browserStore(): KeyValueStore | undefined {
  try {
    return globalThis.localStorage;
  } catch {
    return undefined;
  }
}
