// 左の一覧（D1）に出すものを、接続の状態とワークスペースの一覧から組み立てる。
// 画面の仕組み（Solid）には依存しない。描くのは Sidebar.tsx。
//
// 見た目は親 #271 のモック（「案：フラットな一覧」）に従う：マシンは見出し、ワークスペースは帯で、
// どちらも同じ左端にそろえる。ツリー（D2）・上下の固定（D3）・マシンごとの色（D4）は別の Issue。

import type { Workspace, WorkspaceId } from "../rpc/protocol.ts";
import { LOCAL } from "../ssh/api.ts";
import type { SessionState } from "../ssh/session.ts";

/** GUI がフォーカスしているワークスペース（どのマシンの、どれか）。 */
export interface WorkspaceFocus {
  target: string;
  id: WorkspaceId;
}

/** 見出しの横の丸。つながっている・つなぎ直している・それ以外。 */
export type Health = "up" | "pending" | "down";

export interface Band {
  id: WorkspaceId;
  /** 帯の名前。目印（.git / .shijimi-editor/）が無いワークスペースはルートのパス */
  title: string;
  /** title がパスなら true（等幅で出す） */
  path: boolean;
  /** 右に小さく出すもの（ブランチ、「git なし」） */
  detail: string;
  /** ツールチップに出すルートのパス */
  root: string;
  focused: boolean;
}

export interface MachineGroup {
  target: string;
  /** 見出しの文字（このマシン / ssh のホスト名） */
  label: string;
  health: Health;
  bands: Band[];
}

/** つなぎ先の表示名。 */
export const machineLabel = (t: string): string =>
  t === LOCAL ? "このマシン" : t;

/** 見出しの丸の色分け。 */
export function health(s: SessionState | undefined): Health {
  switch (s?.kind) {
    case "connected":
      return "up";
    case "connecting":
    case "reconnecting":
    case "installing":
      return "pending";
    default:
      return "down";
  }
}

/** ワークスペース 1 つを帯にする。 */
export function band(w: Workspace, focused: boolean): Band {
  if (w.kind === "cwd") {
    return {
      id: w.id,
      title: w.root,
      path: true,
      detail: "git なし",
      root: w.root,
      focused,
    };
  }
  return {
    id: w.id,
    title: w.name || w.root,
    path: !w.name,
    detail: w.branch ?? "",
    root: w.root,
    focused,
  };
}

/**
 * 一覧を組み立てる。targets はつないだ（つなごうとしている）つなぎ先で、この順に並べる。
 * 帯は shj の workspace.list と同じくルートのパスの順（増えても並びが飛ばないように）。
 */
export function buildSidebar(
  targets: string[],
  states: Record<string, SessionState | undefined>,
  workspaces: Record<string, Workspace[] | undefined>,
  focus: WorkspaceFocus | null,
): MachineGroup[] {
  return targets.map((target) => {
    const ws = [...(workspaces[target] ?? [])].sort((a, b) =>
      a.root < b.root ? -1 : a.root > b.root ? 1 : 0,
    );
    return {
      target,
      label: machineLabel(target),
      health: health(states[target]),
      bands: ws.map((w) =>
        band(w, focus?.target === target && focus.id === w.id),
      ),
    };
  });
}

/**
 * フォーカスがまだ無いときに、shj が教えてくれたフォーカスを使うか。
 * GUI のフォーカスは 1 つなので、先に分かったマシンのものを使い、あとから分かったものでは動かさない。
 */
export function adoptFocus(
  current: WorkspaceFocus | null,
  target: string,
  shjFocus: WorkspaceId | null,
): WorkspaceFocus | null {
  if (current) {
    // そのマシンでフォーカス先が消えたら、フォーカスを外す
    if (current.target === target && shjFocus !== current.id) {
      return shjFocus === null ? null : { target, id: shjFocus };
    }
    return current;
  }
  return shjFocus === null ? null : { target, id: shjFocus };
}

/** そのマシンの一覧が変わった・切ったあとに、フォーカス先がまだあるかを確かめる。 */
export function keepFocus(
  current: WorkspaceFocus | null,
  workspaces: Record<string, Workspace[] | undefined>,
): WorkspaceFocus | null {
  if (!current) return null;
  const ws = workspaces[current.target];
  return ws?.some((w) => w.id === current.id) ? current : null;
}
