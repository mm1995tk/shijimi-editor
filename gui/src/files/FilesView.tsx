// ファイルの欄（E1）。shj が送る Screen の files を描くだけで、編集の処理は shj がする。
// 1 行の並べ方は layout.ts、色は theme.ts、見た目は files.css（モック「案：フラットな一覧」に合わせる）。
//
// - 文字は等幅フォントのマス目に置く。区間ごとに幅をマス目（ch）で決めるので、全角の文字もずれない。
// - 欄の大きさが変わったら、入るマス目の桁数と行数を onResize で知らせる（呼ぶ側が view.resize を送る）。
// - タイトル行（E3）、行番号の横の印・診断・インデントガイド（E5）、画像・プレビュー・差分（E6）はまだ描かない。
//   差分とプレビューのペインは、行の文字だけを出す。
import {
  createMemo,
  createSignal,
  For,
  onCleanup,
  onMount,
  Show,
  type JSX,
} from "solid-js";

import type { Line, Pane, Screen, Theme } from "../rpc/protocol.ts";
import {
  cellsFor,
  gutterText,
  layoutLine,
  lineBg,
  paneCells,
  type FilesSize,
  type LineOptions,
} from "./layout.ts";
import { colorOf, runStyle } from "./theme.ts";
import "./files.css";

export type { FilesSize };

export function FilesView(props: {
  screen: Screen | null;
  theme: Theme | null;
  onResize: (size: FilesSize) => void;
}): JSX.Element {
  let body: HTMLDivElement | undefined;
  let measure: HTMLSpanElement | undefined;
  let last: FilesSize | undefined;

  function report() {
    if (!body || !measure) return;
    const cw =
      measure.getBoundingClientRect().width / measure.textContent!.length;
    const size = cellsFor(body.clientWidth, body.clientHeight, cw);
    if (last && last.cols === size.cols && last.rows === size.rows) return;
    last = size;
    props.onResize(size);
  }
  onMount(() => {
    const ro = new ResizeObserver(report);
    ro.observe(body!);
    onCleanup(() => ro.disconnect());
    // 等幅フォントを読み込み終えたら、マス目の幅が変わることがある
    void document.fonts?.ready.then(() => {
      last = undefined;
      report();
    });
  });

  const files = () => props.screen?.files;
  const color = (name: string) => colorOf(props.theme, name);

  return (
    <section
      class="files"
      aria-label="ファイル"
      style={{
        "--files-bg": color("editor.bg"),
        "--files-fg": color("editor.fg"),
        "--files-lineno": color("lineno"),
        "--files-lineno-current": color("lineno.current"),
      }}
    >
      <div class="files-body" ref={body}>
        <span class="files-measure" ref={measure} aria-hidden="true">
          {"0".repeat(64)}
        </span>
        <Show when={files()}>
          {(f) => (
            <For each={f().panes}>
              {(p, i) => (
                <>
                  <Show when={i() > 0}>
                    <div class="files-split" aria-hidden="true" />
                  </Show>
                  <PaneView
                    pane={p}
                    active={i() === f().active}
                    theme={props.theme}
                  />
                </>
              )}
            </For>
          )}
        </Show>
      </div>
    </section>
  );
}

function PaneView(props: {
  pane: Pane;
  active: boolean;
  theme: Theme | null;
}): JSX.Element {
  const opts = createMemo<LineOptions>(() => ({ cursor: props.active }));
  const text = () => props.pane.kind === "text";
  return (
    <div
      class="files-pane"
      role="region"
      aria-label={props.pane.title?.text || props.pane.id}
      data-kind={props.pane.kind}
      data-active={props.active ? "" : undefined}
      style={{ width: `${paneCells(props.pane)}ch` }}
    >
      <Show when={props.pane.kind === "image" && props.pane.caption}>
        <div class="files-caption">{String(props.pane.caption)}</div>
      </Show>
      <For each={props.pane.lines ?? []}>
        {(line) => (
          <LineView
            line={line}
            pane={props.pane}
            opts={opts()}
            gutter={text()}
            theme={props.theme}
          />
        )}
      </For>
    </div>
  );
}

function LineView(props: {
  line: Line;
  pane: Pane;
  opts: LineOptions;
  gutter: boolean;
  theme: Theme | null;
}): JSX.Element {
  const runs = createMemo(() => layoutLine(props.line, props.pane, props.opts));
  const bg = () => colorOf(props.theme, lineBg(props.line, props.pane));
  const current = () =>
    props.line.no >= 0 && props.line.no === props.pane.curLine;
  return (
    <div class="files-line" style={{ background: bg() }}>
      <Show when={props.gutter}>
        <span
          class="files-gutter"
          data-current={current() ? "" : undefined}
          aria-hidden="true"
        >
          {gutterText(props.line.no, props.pane.viewport.total)}
        </span>
      </Show>
      <For each={runs()}>
        {(r) => (
          <span
            class="files-run"
            data-cursor={r.cursor ? "" : undefined}
            style={runStyle(props.theme, r)}
          >
            {r.text}
          </span>
        )}
      </For>
    </div>
  );
}
