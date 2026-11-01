// 左の一覧（D1）に出すものを、接続の状態とワークスペースの一覧から組み立てる。
// 画面の仕組み（Solid）には依存しない。描くのは Sidebar.tsx。
//
// 見た目は親 #271 のモック（「案：フラットな一覧」）に従う：マシンは見出し、ワークスペースは帯で、
// どちらも同じ左端にそろえる。ツリー（D2）は tree/、上下の固定（D3）は splitAtFocus と Sidebar.tsx、
// マシンごとの色（D4）の決め方と保存は colors.ts。

import type { Workspace, WorkspaceId } from "../rpc/protocol.ts";
import { LOCAL } from "../ssh/api.ts";
import type { SessionState } from "../ssh/session.ts";
import { MACHINE_COLORS } from "./colors.ts";

/** GUI がフォーカスしているワークスペース（どのマシンの、どれか）。 */
export interface WorkspaceFocus {
  target: string;
  id: WorkspaceId;
}

/** 見出しの横の丸。つながっている・つなぎ直している・それ以外。 */
export type Health = "up" | "pending" | "down";

/** ワークスペース 1 つ分の帯に出すもの。 */
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

/** マシン 1 台分（見出しと、その下の帯）に出すもの。 */
export interface MachineGroup {
  target: string;
  /** 見出しの文字（このマシン / ssh のホスト名） */
  label: string;
  /** マシンの色（D4）。見出しと帯の左端の線などに使う */
  color: string;
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
 * colors はつなぎ先 → 色（colors.ts の machineColors）。無いつなぎ先は候補の最初の色にする。
 */
export function buildSidebar(
  targets: string[],
  states: Record<string, SessionState | undefined>,
  workspaces: Record<string, Workspace[] | undefined>,
  focus: WorkspaceFocus | null,
  colors: Record<string, string> = {},
): MachineGroup[] {
  return targets.map((target) => {
    const ws = [...(workspaces[target] ?? [])].sort((a, b) =>
      a.root < b.root ? -1 : a.root > b.root ? 1 : 0,
    );
    return {
      target,
      label: machineLabel(target),
      color: colors[target] ?? MACHINE_COLORS[0],
      health: health(states[target]),
      bands: ws.map((w) =>
        band(w, focus?.target === target && focus.id === w.id),
      ),
    };
  });
}

/**
 * shj が教えてくれたフォーカスを GUI のフォーカスに反映する。GUI のフォーカスは 1 つなので、
 * フォーカスが無ければそれを使い、同じマシンのものなら追い（null なら外す）、別のマシンのものでは動かさない。
 */
export function adoptFocus(
  current: WorkspaceFocus | null,
  target: string,
  shjFocus: WorkspaceId | null,
): WorkspaceFocus | null {
  if (current) {
    // 同じマシンで shj のフォーカスが変われば追い、消えれば外す
    if (current.target === target && shjFocus !== current.id) {
      return shjFocus === null ? null : { target, id: shjFocus };
    }
    return current;
  }
  return shjFocus === null ? null : { target, id: shjFocus };
}

/**
 * 一覧が変わった・切った・フォーカスが外れたあとに、フォーカス先がまだあるかを確かめる。
 * 無くなっていたら、ほかのマシンの shj のフォーカス（shjFocus の順）のうち一覧にあるものを使う
 * （起動直後の「先に分かったものを使う」に合わせ、印の無い状態を続けない）。どれも無ければ null。
 */
export function keepFocus(
  current: WorkspaceFocus | null,
  workspaces: Record<string, Workspace[] | undefined>,
  shjFocus: Record<string, WorkspaceId | null | undefined> = {},
): WorkspaceFocus | null {
  const exists = (f: WorkspaceFocus) =>
    workspaces[f.target]?.some((w) => w.id === f.id) ?? false;
  if (current && exists(current)) return current;
  for (const [target, id] of Object.entries(shjFocus)) {
    if (id == null) continue;
    const f = { target, id };
    if (exists(f)) return f;
  }
  return null;
}

/**
 * 上下の固定（D3）のために、一覧をフォーカスしている帯の前後で分けたもの。
 * フォーカスしている帯までは上から順に流し（そのマシンの見出しと帯は上に固定する）、
 * それより後ろの帯とマシンは、まとめて下に固定する。
 */
export interface SplitSidebar {
  /** フォーカスしている帯のマシンより前のマシン */
  before: MachineGroup[];
  /** フォーカスしている帯のマシン。帯はフォーカスしている帯まで。フォーカスが無ければ null */
  focused: MachineGroup | null;
  /** 下に固定するもの。rest はフォーカスしているマシンの残りの帯、groups はそれより後ろのマシン */
  after: {
    target: string;
    label: string;
    color: string;
    rest: Band[];
    groups: MachineGroup[];
  };
}

/** 一覧をフォーカスしている帯の前後で分ける。フォーカスが無ければ全部 before。 */
export function splitAtFocus(groups: MachineGroup[]): SplitSidebar {
  const gi = groups.findIndex((g) => g.bands.some((b) => b.focused));
  if (gi < 0) {
    return {
      before: groups,
      focused: null,
      after: { target: "", label: "", color: "", rest: [], groups: [] },
    };
  }
  const g = groups[gi];
  const bi = g.bands.findIndex((b) => b.focused);
  return {
    before: groups.slice(0, gi),
    focused: { ...g, bands: g.bands.slice(0, bi + 1) },
    after: {
      target: g.target,
      label: g.label,
      color: g.color,
      rest: g.bands.slice(bi + 1),
      groups: groups.slice(gi + 1),
    },
  };
}
