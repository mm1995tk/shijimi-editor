import { createEffect, createSignal, on, onCleanup, type JSX } from "solid-js";

import type { ExtraPane } from "../rpc/protocol.ts";
import { formatBytes } from "./transfer.ts";
import {
  baseName,
  loadVideo,
  MAX_VIDEO_BYTES,
  seekTo,
  tooLarge,
  videoKey,
  type MediaFetch,
} from "./media.ts";

type Load =
  | { kind: "loading"; ratio: number }
  | { kind: "ready"; url: string }
  | { kind: "error"; text: string };

/**
 * 動画の欄（#555）。ターミナルの欄と同じ升目に並ぶ（TerminalGrid）。shj が頼んだ動画を media.read で取って
 * <video> で再生する。見出しの × で閉じる。キーは、この欄にキーが向いているときだけ受ける：
 * 空白と k は再生・停止、← → は 5 秒、j l は 10 秒、m は消音。GUI のショートカット（Ctrl+Shift+矢印など）は
 * ここで止めず、いつも通り GUI が受ける。
 */
export function VideoView(props: {
  pane: ExtraPane;
  /** 中身を取る関数（欄の id は束ねてある） */
  read: MediaFetch;
  /** 今キーを向けている欄か（並べたときに印を付ける） */
  active?: boolean;
  /** 隠すか（IDE モードで別の欄を見せているとき・ほかの欄を隠しているとき） */
  hidden?: boolean;
  /** 欄の位置（CSS grid の grid-area） */
  area?: string;
  /** 欄がフォーカスを受けた */
  onFocus?: () => void;
  onClose: () => void;
  /** この欄にキーを向ける関数を受け取る（欄が無くなるとき undefined で呼ぶ） */
  focusRef?: (focus: (() => void) | undefined) => void;
}): JSX.Element {
  let cell!: HTMLDivElement;
  let video: HTMLVideoElement | undefined;
  const [load, setLoad] = createSignal<Load>({ kind: "loading", ratio: 0 });

  const focus = () => (video ?? cell).focus();
  props.focusRef?.(focus);
  onCleanup(() => props.focusRef?.(undefined));

  // 欄の動画（id）が替わったら、取り直す
  createEffect(
    on(
      () => props.pane.id,
      () => {
        const pane = props.pane;
        const ac = new AbortController();
        let url: string | undefined;
        setLoad({ kind: "loading", ratio: 0 });
        onCleanup(() => {
          ac.abort();
          if (url) URL.revokeObjectURL(url);
        });
        if (tooLarge(pane.size)) {
          setLoad({
            kind: "error",
            text: `動画が大きすぎて開けません（${formatBytes(pane.size)}。上限は ${formatBytes(MAX_VIDEO_BYTES)}）`,
          });
          return;
        }
        loadVideo(props.read, ac.signal, (ratio) =>
          setLoad({ kind: "loading", ratio }),
        ).then(
          (blob) => {
            if (ac.signal.aborted) return;
            url = URL.createObjectURL(blob);
            setLoad({ kind: "ready", url });
          },
          (e) => {
            if (ac.signal.aborted) return;
            setLoad({
              kind: "error",
              text: `動画を取れませんでした：${e instanceof Error ? e.message : e}`,
            });
          },
        );
      },
    ),
  );

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.isComposing || !video) return;
    const k = videoKey(e);
    if (!k) return;
    e.preventDefault();
    e.stopPropagation();
    if (k.kind === "toggle") void (video.paused ? video.play() : video.pause());
    else if (k.kind === "mute") video.muted = !video.muted;
    else video.currentTime = seekTo(video.currentTime, k.dt, video.duration);
  };

  return (
    <div
      class="terminal-cell video-cell"
      data-active={props.active ? "" : undefined}
      hidden={props.hidden}
      style={props.area ? { "grid-area": props.area } : undefined}
      tabindex="-1"
      ref={cell}
      onFocusIn={() => props.onFocus?.()}
      onKeyDown={onKeyDown}
      onPointerDown={() => props.onFocus?.()}
    >
      <div class="video-head">
        <span class="video-name" title={props.pane.path}>
          {baseName(props.pane.path)}
        </span>
        <button
          type="button"
          class="video-close"
          aria-label="動画の欄を閉じる"
          title="閉じる"
          onClick={() => props.onClose()}
        >
          ×
        </button>
      </div>
      <div class="video-body">
        {(() => {
          const l = load();
          if (l.kind === "ready")
            return (
              <video
                ref={(el) => {
                  video = el;
                  onCleanup(() => {
                    if (video === el) video = undefined;
                  });
                }}
                src={l.url}
                controls
                autoplay
                playsinline
                tabindex="0"
                onError={() =>
                  setLoad({
                    kind: "error",
                    text: "この動画は再生できません（形式に対応していません）",
                  })
                }
              />
            );
          return (
            <p class="video-note" role="status">
              {l.kind === "error"
                ? l.text
                : `動画を取っています… ${Math.round((l.kind === "loading" ? l.ratio : 1) * 100)}%`}
            </p>
          );
        })()}
      </div>
    </div>
  );
}
