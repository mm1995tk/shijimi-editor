// ssh の質問（パスワード・認証コード・ホスト鍵の確認）を出す窓。
import { createSignal, Show, type JSX } from "solid-js";

import type { AskpassRequest } from "./ssh/api.ts";

/**
 * 質問が無ければ何も出さない。答えは onReply に渡す（null は取り消し）。
 * prompt は接続先のサーバーが決められる文字列なので、伏せ字にするかは prompt で決めない
 * （既定は常に伏せ、「表示する」でユーザーが外す）。
 */
export function AskpassDialog(props: {
  ask: AskpassRequest | undefined;
  onReply: (requestId: number, value: string | null) => void;
}): JSX.Element {
  const [answer, setAnswer] = createSignal("");
  const [reveal, setReveal] = createSignal(false);

  function reply(ask: AskpassRequest, value: string | null) {
    setAnswer("");
    setReveal(false);
    props.onReply(ask.requestId, value);
  }

  return (
    <Show when={props.ask} keyed>
      {(ask) => (
        <dialog open aria-modal="true" aria-label="ssh の質問">
          <form
            method="dialog"
            onSubmit={(e) => {
              e.preventDefault();
              reply(ask, answer());
            }}
          >
            <p>{ask.prompt}</p>
            <input
              type={reveal() ? "text" : "password"}
              value={answer()}
              onInput={(e) => setAnswer(e.currentTarget.value)}
              autofocus
            />
            <label>
              <input
                type="checkbox"
                checked={reveal()}
                onChange={(e) => setReveal(e.currentTarget.checked)}
              />
              表示する（host key の確認は yes / no と打つ）
            </label>
            <button type="submit">OK</button>
            <button type="button" onClick={() => reply(ask, null)}>
              取り消し
            </button>
          </form>
        </dialog>
      )}
    </Show>
  );
}
