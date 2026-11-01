// ホストを開く窓（モック Connect.dc.html）。左の一覧の一番下の「ホストを開く」で出す。
// 候補はこのマシンと ~/.ssh/config のホスト。名前で絞り、行を選んで「つなぐ」（行のダブルクリック・Enter でも）。
// フォルダを選んで開くところ（モックの下半分）は shj の側に無いので、まだ作らない。
import { createMemo, createSignal, For, Show, type JSX } from "solid-js";

import { health, machineLabel } from "../sidebar/model.ts";
import type { SessionState } from "../ssh/session.ts";
import { ModalCard } from "./Card.tsx";
import { canConnect, stateLabel } from "./status.ts";

export function HostPicker(props: {
  /** 候補（LOCAL と ~/.ssh/config のホスト）。この順に並べる */
  targets: string[];
  stateOf: (target: string) => SessionState | undefined;
  onConnect: (target: string) => void;
  onClose: () => void;
}): JSX.Element {
  const [query, setQuery] = createSignal("");
  const [picked, setPicked] = createSignal<string | undefined>();
  const shown = createMemo(() => {
    const q = query().trim().toLowerCase();
    return q === ""
      ? props.targets
      : props.targets.filter((t) => machineLabel(t).toLowerCase().includes(q));
  });
  // 選んだ行が絞り込みで消えたら、残りのうちつなげる最初の行（無ければ最初の行）
  const selected = createMemo(() => {
    const p = picked();
    if (p !== undefined && shown().includes(p)) return p;
    return shown().find((t) => canConnect(props.stateOf(t))) ?? shown()[0];
  });
  const ready = () => {
    const t = selected();
    return t !== undefined && canConnect(props.stateOf(t));
  };

  function go(t: string | undefined = selected()) {
    if (t === undefined || !canConnect(props.stateOf(t))) return;
    props.onConnect(t);
    props.onClose();
  }

  function move(step: number) {
    const list = shown();
    if (list.length === 0) return;
    const i = list.indexOf(selected() ?? "");
    setPicked(list[(i + step + list.length) % list.length]);
  }

  return (
    <ModalCard label="ホストを開く" onCancel={props.onClose}>
      <div class="card-body">
        <h1 class="card-title">ホストを開く</h1>
        <label class="card-field">
          ホスト名で探す
          <input
            class="card-input"
            type="text"
            placeholder="~/.ssh/config のホスト"
            value={query()}
            onInput={(e) => setQuery(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.isComposing) return;
              if (e.key === "ArrowDown" || e.key === "ArrowUp") {
                e.preventDefault();
                move(e.key === "ArrowDown" ? 1 : -1);
              } else if (e.key === "Enter") {
                e.preventDefault();
                go();
              }
            }}
            autofocus
          />
        </label>
      </div>
      <div class="card-section" id="host-picker-list">
        このマシンと ~/.ssh/config から
      </div>
      <ul class="card-rows" role="listbox" aria-labelledby="host-picker-list">
        <For
          each={shown()}
          fallback={<li class="card-empty">合うホストはありません</li>}
        >
          {(t) => (
            <li>
              <button
                type="button"
                class="card-row"
                role="option"
                aria-selected={selected() === t ? "true" : "false"}
                onClick={() => setPicked(t)}
                onDblClick={() => go(t)}
              >
                <span
                  class="state-dot"
                  data-health={health(props.stateOf(t))}
                  aria-hidden="true"
                />
                <span class="card-row-name">{machineLabel(t)}</span>
                <Show when={stateLabel(props.stateOf(t))}>
                  {(l) => <span class="card-row-state">{l()}</span>}
                </Show>
              </button>
            </li>
          )}
        </For>
      </ul>
      <div class="card-actions">
        <button type="button" class="card-button" onClick={props.onClose}>
          閉じる
        </button>
        <button
          type="button"
          class="card-button primary"
          disabled={!ready()}
          onClick={() => go()}
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            aria-hidden="true"
          >
            <path d="M5 12h14M13 6l6 6-6 6" />
          </svg>
          つなぐ
        </button>
      </div>
    </ModalCard>
  );
}
