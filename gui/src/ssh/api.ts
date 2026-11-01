// Tauri の殻（src-tauri）とのやりとり。テストで差し替えられるよう、窓口は ShellApi にまとめる。

export interface AskpassRequest {
  requestId: number;
  conn: string;
  prompt: string;
}

export interface ClosedEvent {
  id: string;
  code: number | null;
  stderr: string;
}

/** リモートに入れるものの説明。ユーザーに聞くときに見せる。 */
export interface InstallPlan {
  /** GUI の版 */
  version: string;
  /** リモートに置く場所（`~/.cache/shijimi-editor/<版>/shj`。PATH の shj ではない） */
  path: string;
  /** 取ってくる GitHub Release のページ */
  release_url: string;
}

export interface InstallResult {
  path: string;
  asset: string;
  sha256: string;
}

/** shj が無いとき、`ssh host -- <shj> attach` がこの終了コードで終わる（殻の EXIT_SHJ_MISSING と同じ値）。 */
export const EXIT_SHJ_MISSING = 97;

export type Unlisten = () => void;

export interface ShellApi {
  listSshHosts(): Promise<string[]>;
  /** target は ssh のホスト名か LOCAL。id は呼ぶ側が決める（イベントの宛先） */
  connect(id: string, target: string): Promise<void>;
  send(id: string, text: string): Promise<void>;
  disconnect(id: string): Promise<void>;
  askpassReply(requestId: number, value: string | null): Promise<void>;
  /** 入れるものの説明（ユーザーに聞く前に見せる） */
  installPlan(): Promise<InstallPlan>;
  /**
   * リモートに shj を入れる。ユーザーが「入れる」と答えたあとにだけ呼ぶこと（ここでは聞かない）。
   * id は askpass の質問の宛先。取得した実行ファイルの SHA-256 と、リモートに置いた後の SHA-256 が
   * checksums と合わなければ、入れずに失敗する。
   */
  installRemote(id: string, target: string): Promise<InstallResult>;
  onLine(h: (e: { id: string; line: string }) => void): Promise<Unlisten>;
  onClosed(h: (e: ClosedEvent) => void): Promise<Unlisten>;
  onAskpass(h: (e: AskpassRequest) => void): Promise<Unlisten>;
  /** 答えが済んだ（時間切れ・接続が閉じた）質問の requestId */
  onAskpassDone(h: (requestId: number) => void): Promise<Unlisten>;
}

/** 「このマシン」。ssh を挟まず shj attach を直接起動する。ssh のホスト名とは取り違えない形にしてある。 */
export const LOCAL = "@local";
