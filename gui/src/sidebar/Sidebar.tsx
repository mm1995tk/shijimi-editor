// 左の一覧（D1）：つないだマシンごとに見出しとワークスペースの帯を出す。帯を押すとフォーカスを移す。
// 組み立ては model.ts、見た目は sidebar.css。
import { For, Show, type JSX } from "solid-js";

import type { WorkspaceId } from "../rpc/protocol.ts";
import type { Health, MachineGroup } from "./model.ts";
import "./sidebar.css";

const healthLabel: Record<Health, string> = {
  up: "つながっています",
  pending: "つないでいます",
  down: "つながっていません",
};

/** 左の一覧。つないだマシンごとに見出しとワークスペースの帯を出し、帯を押すと onFocus を呼ぶ。 */
export function Sidebar(props: {
  groups: MachineGroup[];
  onFocus: (target: string, id: WorkspaceId) => void;
}): JSX.Element {
  return (
    <nav class="sidebar" aria-label="マシンとワークスペース">
      <For each={props.groups}>
        {(g) => (
          <section class="machine" data-target={g.target} aria-label={g.label}>
            <h2 class="machine-heading">
              <span class="machine-name">{g.label}</span>
              <span
                class="health"
                data-health={g.health}
                role="img"
                aria-label={healthLabel[g.health]}
                title={healthLabel[g.health]}
              />
            </h2>
            <Show
              when={g.bands.length > 0}
              fallback={<p class="no-workspace">ワークスペースはありません</p>}
            >
              <ul class="bands">
                <For each={g.bands}>
                  {(b) => (
                    <li>
                      <button
                        class="band"
                        aria-current={b.focused ? "true" : undefined}
                        title={b.root}
                        onClick={() => props.onFocus(g.target, b.id)}
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
                        <span class="band-title" classList={{ path: b.path }}>
                          {b.title}
                        </span>
                        <Show when={b.detail}>
                          <span class="band-detail">{b.detail}</span>
                        </Show>
                      </button>
                    </li>
                  )}
                </For>
              </ul>
            </Show>
          </section>
        )}
      </For>
    </nav>
  );
}
