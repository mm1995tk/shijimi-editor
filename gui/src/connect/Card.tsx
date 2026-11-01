// 窓のカード（モック Connect.dc.html の「ホストを開く」と同じ決まり）。
// ホストを開く窓・shj を入れるかの確認・ssh の質問の窓は、真ん中に出して後ろを触らせないもの（ModalCard）。
// つながらない・引き継がれた・新しい版の知らせは、右下に積んで後ろも触れるもの（.card-notice。Notices.tsx）。
// 寸法と色は card.css。
import { onMount, type JSX } from "solid-js";

import "./card.css";

/**
 * 真ん中に出すカード。出したら showModal で後ろを触らせず、Esc で onCancel を呼ぶ
 * （dialog は自分では閉じない。閉じるのは呼んだ側が出すのをやめたとき）。
 */
export function ModalCard(props: {
  label: string;
  onCancel: () => void;
  /** 幅（px）。省くとモックの 560px */
  width?: number;
  children: JSX.Element;
}): JSX.Element {
  let el!: HTMLDialogElement;
  onMount(() => {
    // 取れない WebView では開いたまま（open）の素の窓になる
    if (!el.open) el.showModal?.();
    if (!el.open) el.setAttribute("open", "");
  });
  return (
    <dialog
      ref={el}
      class="card card-modal"
      style={props.width ? { width: `${props.width}px` } : undefined}
      aria-label={props.label}
      onCancel={(e) => {
        e.preventDefault();
        props.onCancel();
      }}
    >
      {props.children}
    </dialog>
  );
}
