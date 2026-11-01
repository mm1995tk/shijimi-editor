// ファイルの欄（E1）。shj が送る Screen の files を描くだけで、編集の処理は shj がする。
// 1 行の並べ方は layout.ts、色は theme.ts、見た目は files.css（モック「案：フラットな一覧」に合わせる）。
//
// - 文字は等幅フォントのマス目に置く。区間ごとに幅をマス目（ch）で決めるので、全角の文字もずれない。
// - 欄の大きさが変わったら、入るマス目の桁数と行数を onResize で知らせる（呼ぶ側が view.resize を送る）。
// - キー・IME・マウス・貼り付けは onInput で shj に送る（E2）。キーは見えない textarea で受ける
//   （IME の変換と貼り付けを OS に任せるため）。変換中の文字は shj が下線付きで画面に入れて返す。
//   パラメータの作り方は input.ts、OS のクリップボードは clipboard.ts。
// - タイトル行（E3）、行番号の横の印・診断・インデントガイド（E5）、画像・プレビュー・差分（E6）はまだ描かない。
//   差分とプレビューのペインは、行の文字だけを出す。
import {
  createEffect,
  createMemo,
  For,
  Index,
  onCleanup,
  onMount,
  Show,
  type JSX,
  untrack,
} from "solid-js";

import type { Line, Pane, Requests, Screen, Theme } from "../rpc/protocol.ts";
import type { InputMethod } from "../ssh/session.ts";
import type { OsClipboard } from "./clipboard.ts";
import {
  cellAt,
  ClickCounter,
  commitParams,
  isMac,
  isPasteKey,
  keyParams,
  modsOf,
  runeOffset,
  WheelAccumulator,
} from "./input.ts";
import {
  cellsFor,
  gutterText,
  gutterWidth,
  LINE_HEIGHT,
  layoutLine,
  lineBg,
  lineKey,
  paneCells,
  type FilesSize,
  type LineOptions,
} from "./layout.ts";
import { colorOf, runStyle } from "./theme.ts";
import "./files.css";

export type { FilesSize };

/** 入力を shj に送る（seq は送る側が付ける）。 */
export type SendInput = <M extends InputMethod>(
  method: M,
  params: Omit<Requests[M]["params"], "seq">,
) => void;

const REGION = "files";

/**
 * ファイルの欄。screen（フォーカスしているマシンの画面）の files を、theme の色で描く。
 * screen が null のときと files が無いときは、空の欄だけを出す。
 * 欄の大きさが変わったら、入るマス目の桁数と行数を onResize で知らせる。
 */
export function FilesView(props: {
  screen: Screen | null;
  theme: Theme | null;
  onResize: (size: FilesSize) => void;
  /** 入力の送り先。省くと送らない */
  onInput?: SendInput;
  /** OS のクリップボード（貼り付けのキーで読む）。省くと貼り付けない */
  clipboard?: OsClipboard;
}): JSX.Element {
  let body: HTMLDivElement | undefined;
  let measure: HTMLSpanElement | undefined;
  let ime: HTMLTextAreaElement | undefined;
  let last: FilesSize | undefined;
  const mac = isMac(navigator.platform || navigator.userAgent);

  /** 等幅のマス目 1 つの幅（px）。 */
  function cellWidth(): number {
    if (!measure) return 8;
    return measure.getBoundingClientRect().width / measure.textContent!.length;
  }

  function report() {
    if (!body || !measure) return;
    const size = cellsFor(body.clientWidth, body.clientHeight, cellWidth());
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
    // ホイールで欄そのものがスクロールしないよう、passive にしない
    body!.addEventListener("wheel", onWheel, { passive: false });
    onCleanup(() => {
      body?.removeEventListener("wheel", onWheel);
      endDrag();
    });
  });

  const files = () => props.screen?.files;
  const color = (name: string) => colorOf(props.theme, name);

  // ---- 入力（E2） ----

  const send: SendInput = (method, params) => props.onInput?.(method, params);
  const clicks = new ClickCounter();
  const wheel = new WheelAccumulator();
  let composing = false;

  // 画面が出たとき、ほかに何もフォーカスしていなければキーを受ける
  createEffect(() => {
    if (!files()) return;
    if (!document.activeElement || document.activeElement === document.body)
      ime?.focus({ preventScroll: true });
  });

  /** IME の候補の窓がカーソルの近くに出るよう、textarea をカーソルのマス目に置く。 */
  function placeIme() {
    if (!ime || !body) return;
    const cur = body.querySelector<HTMLElement>(
      ".files-pane[data-active] [data-cursor]",
    );
    if (!cur) return;
    const b = body.getBoundingClientRect();
    const c = cur.getBoundingClientRect();
    ime.style.left = `${c.left - b.left}px`;
    ime.style.top = `${c.top - b.top}px`;
  }

  function onKeyDown(e: KeyboardEvent) {
    void props.clipboard?.flush();
    if (composing || e.isComposing || e.keyCode === 229) return;
    if (isPasteKey(e, mac)) {
      // macOS は OS の貼り付け（paste のイベント）に任せる。ほかは読んで送る
      if (mac) return;
      e.preventDefault();
      void props.clipboard?.read().then((text) => {
        if (text) send("input.paste", { region: REGION, text });
      });
      return;
    }
    const p = keyParams(e, mac);
    if (!p) return;
    // Tab でフォーカスが移ったり、文字が textarea に入ったりしないように
    e.preventDefault();
    send("input.key", { region: REGION, ...p });
  }

  function onPaste(e: ClipboardEvent) {
    e.preventDefault();
    // OS のクリップボードに書けていない、shj でコピーした文字のほうが新しい
    const text =
      props.clipboard?.unwritten ?? e.clipboardData?.getData("text/plain");
    if (text) send("input.paste", { region: REGION, text });
  }

  function sendComposition() {
    if (!ime) return;
    const v = ime.value;
    send("input.composition", {
      region: REGION,
      text: v,
      cursor: [
        runeOffset(v, ime.selectionStart ?? v.length),
        runeOffset(v, ime.selectionEnd ?? v.length),
      ],
    });
  }

  function onCompositionStart() {
    composing = true;
    placeIme();
  }

  function onCompositionEnd(e: CompositionEvent) {
    composing = false;
    const text = e.data ?? ime?.value ?? "";
    send("input.composition", { region: REGION, text: "", cursor: null });
    if (text) send("input.key", { region: REGION, ...commitParams(text) });
    if (ime) ime.value = "";
  }

  function onImeInput(e: InputEvent) {
    if (composing || e.isComposing) {
      sendComposition();
      return;
    }
    const text = ime?.value ?? "";
    if (ime) ime.value = "";
    // WebKit は compositionend のあとに確定の input を出すことがある（compositionend で送り済み）
    if (e.inputType?.includes("Composition")) return;
    // 変換を経ずに入った文字（キーで送らなかったもの）は、確定した文字として送る
    if (text) send("input.key", { region: REGION, ...commitParams(text) });
  }

  /** 点（画面の px）の下にあるペインと、その中のマス目。 */
  function hit(
    el: HTMLElement,
    x: number,
    y: number,
  ): { pane: string; row: number; col: number } | null {
    const id = el.dataset.paneId;
    const p = files()?.panes.find((q) => q.id === id);
    if (!id || !p) return null;
    const r = el.getBoundingClientRect();
    const gutter = p.kind === "text" ? gutterWidth(p.viewport.total) : 0;
    return {
      pane: id,
      ...cellAt(x - r.left, y - r.top, cellWidth(), LINE_HEIGHT, gutter),
    };
  }

  function paneAt(target: EventTarget | null): HTMLElement | null {
    return target instanceof Element
      ? target.closest<HTMLElement>(".files-pane")
      : null;
  }

  /** 押したままのボタンと、押したペイン（外へ出ても、そのペインのマス目で送る） */
  let drag:
    { el: HTMLElement; button: number; row: number; col: number } | undefined;

  function onMouseDown(e: MouseEvent) {
    void props.clipboard?.flush();
    // 文字の選択を始めず、キーは textarea に残す
    e.preventDefault();
    ime?.focus({ preventScroll: true });
    const el = paneAt(e.target);
    if (!el || drag) return;
    const h = hit(el, e.clientX, e.clientY);
    if (!h) return;
    send("input.mouse", {
      region: REGION,
      pane: h.pane,
      kind: "down",
      button: e.button,
      clicks: clicks.press(e.timeStamp, e.button, h.pane, h.row, h.col),
      row: h.row,
      col: h.col,
      mods: modsOf(e),
    });
    drag = { el, button: e.button, row: h.row, col: h.col };
    window.addEventListener("mousemove", onDragMove);
    window.addEventListener("mouseup", onDragUp);
  }

  function onDragMove(e: MouseEvent) {
    if (!drag) return;
    const h = hit(drag.el, e.clientX, e.clientY);
    if (!h || (h.row === drag.row && h.col === drag.col)) return;
    drag.row = h.row;
    drag.col = h.col;
    send("input.mouse", {
      region: REGION,
      pane: h.pane,
      kind: "move",
      button: drag.button,
      row: h.row,
      col: h.col,
      mods: modsOf(e),
    });
  }

  function onDragUp(e: MouseEvent) {
    if (!drag || e.button !== drag.button) return;
    const h = hit(drag.el, e.clientX, e.clientY);
    const d = drag;
    endDrag();
    send("input.mouse", {
      region: REGION,
      pane: h?.pane ?? d.el.dataset.paneId,
      kind: "up",
      button: d.button,
      row: h?.row ?? d.row,
      col: h?.col ?? d.col,
      mods: modsOf(e),
    });
  }

  function endDrag() {
    drag = undefined;
    window.removeEventListener("mousemove", onDragMove);
    window.removeEventListener("mouseup", onDragUp);
  }

  function onWheel(e: WheelEvent) {
    e.preventDefault();
    const el = paneAt(e.target);
    const h = el && hit(el, e.clientX, e.clientY);
    if (!h) return;
    const p = files()?.panes.find((q) => q.id === h.pane);
    const d = wheel.add(e.deltaX, e.deltaY, e.deltaMode, {
      cellWidth: cellWidth(),
      lineHeight: LINE_HEIGHT,
      rows: p?.viewport.rows ?? 1,
      cols: p?.viewport.cols ?? 1,
    });
    if (d.dx === 0 && d.dy === 0) return;
    send("input.mouse", {
      region: REGION,
      pane: h.pane,
      kind: "wheel",
      row: h.row,
      col: h.col,
      dx: d.dx,
      dy: d.dy,
      mods: modsOf(e),
    });
  }

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
      <div
        class="files-body"
        ref={body}
        onMouseDown={onMouseDown}
        onContextMenu={(e) => e.preventDefault()}
      >
        <span class="files-measure" ref={measure} aria-hidden="true">
          {"0".repeat(64)}
        </span>
        <textarea
          class="files-ime"
          ref={ime}
          aria-label="ファイルの欄への入力"
          autocomplete="off"
          autocapitalize="off"
          spellcheck={false}
          onKeyDown={onKeyDown}
          onPaste={onPaste}
          onCompositionStart={onCompositionStart}
          onCompositionEnd={onCompositionEnd}
          onInput={onImeInput}
          onFocus={() => {
            placeIme();
            send("input.focus", { region: REGION });
          }}
        />
        <Show when={files()}>
          {(f) => (
            // 位置で使い回す（ペインが新しいオブジェクトになっても PaneView と変わらない行の DOM を残す）
            <Index each={f().panes}>
              {(p, i) => (
                <>
                  <Show when={i > 0}>
                    <div class="files-split" aria-hidden="true" />
                  </Show>
                  <PaneView
                    pane={p()}
                    active={i === f().active}
                    theme={props.theme}
                  />
                </>
              )}
            </Index>
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
      data-pane-id={props.pane.id}
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
  // ペインが変わっても、この行を描くのに使う項目が同じなら描き直さない（key は文字列なので、同じなら先へ伝わらない）
  const key = createMemo(() => lineKey(props.line, props.pane, props.opts));
  const runs = createMemo(() => {
    key();
    return untrack(() => layoutLine(props.line, props.pane, props.opts));
  });
  const bg = createMemo(() => {
    key();
    return untrack(() => lineBg(props.line, props.pane));
  });
  const total = createMemo(() => props.pane.viewport.total);
  const current = () =>
    props.line.no >= 0 && props.line.no === props.pane.curLine;
  return (
    <div class="files-line" style={{ background: colorOf(props.theme, bg()) }}>
      <Show when={props.gutter}>
        <span
          class="files-gutter"
          data-current={current() ? "" : undefined}
          aria-hidden="true"
        >
          {gutterText(props.line.no, total())}
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
