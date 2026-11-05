import { init } from "coneko-term";
import { render } from "solid-js/web";
import "./fonts.ts";
import { App } from "./App";

// ターミナル（coneko-term）は wasm を読み終えてから作る。読み終えるまで画面は出さない
await init();
render(() => <App />, document.getElementById("root")!);
