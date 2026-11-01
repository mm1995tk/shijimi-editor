// 左の一覧（D1）：つないだマシンごとに見出しとワークスペースの帯を出す。帯を押すとフォーカスを移す。
// フォーカスしているワークスペースの帯の下にはツリーを出す（D2、描くのは tree/TreeView.tsx）。
// スクロールしても、上にフォーカスしているマシンの見出し → 帯 → 親フォルダを、下にそれより後ろの帯を固定する（D3）。
// マシンごとの色（D4）は section の --machine-color に入れ、見出しと帯の左端の線とアイコンに使う。
// 組み立ては model.ts、色の決め方は colors.ts、見た目は sidebar.css。
import { createMemo, For, onCleanup, onMount, Show, type JSX } from "solid-js";

import type { WorkspaceId } from "../rpc/protocol.ts";
import type { TreeRow } from "../tree/model.ts";
import { TreeView } from "../tree/TreeView.tsx";
import {
  splitAtFocus,
  type Band,
  type Health,
  type MachineGroup,
} from "./model.ts";
import "./sidebar.css";

const healthLabel: Record<Health, string> = {
  up: "つながっています",
  pending: "つないでいます",
  down: "つながっていません",
};

interface Handlers {
  onFocus: (target: string, id: WorkspaceId) => void;
  /** ツリーの行。まだ取っていなければ undefined */
  treeOf?: (target: string, id: WorkspaceId) => TreeRow[] | undefined;
  onToggle?: (target: string, id: WorkspaceId, path: string) => void;
}

/**
 * 左の一覧。つないだマシンごとに見出しとワークスペースの帯を出し、帯を押すと onFocus を呼ぶ。
 * フォーカスしている帯の下に treeOf のツリーを出し、フォルダを押すと onToggle を呼ぶ。
 *
 * 上下の固定（D3）は CSS の sticky で、要素の組み方で止まるところを決める:
 * フォーカスしているマシンの見出しはそのマシンの要素（帯はフォーカスしている帯まで）の中で、
 * 帯はツリーと一緒の要素の中で、フォルダの行は中身と一緒の要素の中で止まる。
 * それより後ろの帯とマシンは 1 つの要素にまとめ、一覧の下に固定する。
 */
export function Sidebar(
  props: Handlers & {
    groups: MachineGroup[];
    /** 一覧の要素（スクロールの位置を覚えて戻すのに使う） */
    ref?: (el: HTMLElement) => void;
  },
): JSX.Element {
  const split = createMemo(() => splitAtFocus(props.groups));
  const hasAfter = () =>
    split().after.rest.length > 0 || split().after.groups.length > 0;
  let nav!: HTMLElement;

  // 固定されているものに data-stuck を付ける（背景と影をモックに合わせる）。
  // 上に固定するものは、sticky の位置（一覧の上端から top）にいて、親の要素の上端より下にずれていれば
  // 固定されている（中身を通り過ぎて押し出された行は sticky の位置より上にいるので外れる）。
  // 下の要素は、一覧の終わりまでスクロールしていなければ固定されている。
  // 影（data-shadow）は、上に固定されたものの一番下と、下の要素に付ける
  onMount(() => {
    let frame = 0;
    const update = () => {
      frame = 0;
      let last: HTMLElement | null = null;
      for (const el of nav.querySelectorAll<HTMLElement>("[data-sticky]")) {
        const bottom = el.dataset.sticky === "bottom";
        let stuck: boolean;
        if (bottom) {
          stuck = nav.scrollTop + nav.clientHeight < nav.scrollHeight - 0.5;
        } else {
          const parent = el.parentElement;
          const top = el.getBoundingClientRect().top;
          const at =
            nav.getBoundingClientRect().top +
            nav.clientTop +
            (parseFloat(getComputedStyle(el).top) || 0);
          stuck =
            parent !== null &&
            Math.abs(top - at) < 0.5 &&
            top > parent.getBoundingClientRect().top + 0.5;
          if (stuck) last = el;
        }
        el.toggleAttribute("data-stuck", stuck);
        el.toggleAttribute("data-shadow", bottom && stuck);
      }
      last?.setAttribute("data-shadow", "");
    };
    const schedule = () => {
      if (frame === 0) frame = requestAnimationFrame(update);
    };
    nav.addEventListener("scroll", schedule, { passive: true });
    const resize = new ResizeObserver(schedule);
    resize.observe(nav);
    // 行が増減した・フォーカスが移った（属性は見ないので、data-stuck を付けても回らない）
    const mutation = new MutationObserver(schedule);
    mutation.observe(nav, { childList: true, subtree: true });
    schedule();
    onCleanup(() => {
      nav.removeEventListener("scroll", schedule);
      resize.disconnect();
      mutation.disconnect();
      if (frame !== 0) cancelAnimationFrame(frame);
    });
  });

  return (
    <nav
      class="sidebar"
      aria-label="マシンとワークスペース"
      ref={(el) => {
        nav = el;
        props.ref?.(el);
      }}
    >
      <For each={split().before}>
        {(g) => <Machine group={g} {...handlers(props)} />}
      </For>
      <Show when={split().focused}>
        {(g) => <Machine group={g()} sticky {...handlers(props)} />}
      </Show>
      <Show when={hasAfter()}>
        <div class="sidebar-after" data-sticky="bottom">
          <Show when={split().after.rest.length > 0}>
            {/* 見出しの無いマシンの要素にして、マシンの色（--machine-color）を同じにする */}
            <section
              class="machine machine-rest"
              style={{ "--machine-color": split().after.color }}
              data-target={split().after.target}
              aria-label={split().after.label}
            >
              <ul class="bands">
                <For each={split().after.rest}>
                  {(b) => (
                    <BandItem
                      target={split().after.target}
                      band={b}
                      {...handlers(props)}
                    />
                  )}
                </For>
              </ul>
            </section>
          </Show>
          <For each={split().after.groups}>
            {(g) => <Machine group={g} {...handlers(props)} />}
          </For>
        </div>
      </Show>
    </nav>
  );
}

/** 子に渡す処理だけを取り出す（getter にして、props の追跡を保つ）。 */
function handlers(props: Handlers): Handlers {
  return {
    get onFocus() {
      return props.onFocus;
    },
    get treeOf() {
      return props.treeOf;
    },
    get onToggle() {
      return props.onToggle;
    },
  };
}

/** マシン 1 台分。sticky なら見出しを上に固定する（フォーカスしているマシン）。 */
function Machine(
  props: Handlers & { group: MachineGroup; sticky?: boolean },
): JSX.Element {
  const g = () => props.group;
  return (
    <section
      class="machine"
      style={{ "--machine-color": g().color }}
      data-target={g().target}
      aria-label={g().label}
    >
      <h2
        class="machine-heading"
        data-sticky={props.sticky ? "top" : undefined}
      >
        <span class="machine-name">{g().label}</span>
        <span
          class="health"
          data-health={g().health}
          role="img"
          aria-label={healthLabel[g().health]}
          title={healthLabel[g().health]}
        />
      </h2>
      <Show
        when={g().bands.length > 0}
        fallback={<p class="no-workspace">ワークスペースはありません</p>}
      >
        <ul class="bands">
          <For each={g().bands}>
            {(b) => (
              <BandItem target={g().target} band={b} {...handlers(props)} />
            )}
          </For>
        </ul>
      </Show>
    </section>
  );
}

/** 帯 1 つ。フォーカスしていれば帯を上に固定し、下にツリーを出す。 */
function BandItem(
  props: Handlers & { target: string; band: Band },
): JSX.Element {
  const b = () => props.band;
  return (
    <li>
      <button
        class="band"
        aria-current={b().focused ? "true" : undefined}
        data-sticky={b().focused ? "top" : undefined}
        title={b().root}
        onClick={() => props.onFocus(props.target, b().id)}
      >
        <svg
          class="band-icon"
          width="16"
          height="16"
          viewBox="0 0 16 16"
          aria-hidden="true"
        >
          <path d="M8 2l6 3-6 3-6-3z" />
          <path d="M2 8l6 3 6-3" />
          <path d="M2 11l6 3 6-3" />
        </svg>
        <span class="band-title" classList={{ path: b().path }}>
          {b().title}
        </span>
        <Show when={b().detail}>
          <span class="band-detail">{b().detail}</span>
        </Show>
      </button>
      <Show when={b().focused && props.treeOf?.(props.target, b().id)}>
        {(rows) => (
          <TreeView
            rows={rows()}
            label={`${b().title} のファイル`}
            onToggle={(p) => props.onToggle?.(props.target, b().id, p)}
          />
        )}
      </Show>
    </li>
  );
}
