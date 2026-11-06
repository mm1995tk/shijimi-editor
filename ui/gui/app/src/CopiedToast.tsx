import { createSignal, Show } from "solid-js";

import { CopiedNotice } from "./copiedNotice.ts";

const [visible, setVisible] = createSignal(false);

/** 画面のどこからでも呼べる「コピーしました」（コピーに成功したときだけ呼ぶ）。 */
export const copiedNotice = new CopiedNotice(setVisible);

export function CopiedToast() {
  return (
    <Show when={visible()}>
      <div class="copied-toast" role="status">
        コピーしました
      </div>
    </Show>
  );
}
