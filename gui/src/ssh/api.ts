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

export type Unlisten = () => void;

export interface ShellApi {
  listSshHosts(): Promise<string[]>;
  /** target は ssh のホスト名か LOCAL。id は呼ぶ側が決める（イベントの宛先） */
  connect(id: string, target: string): Promise<void>;
  send(id: string, text: string): Promise<void>;
  disconnect(id: string): Promise<void>;
  askpassReply(requestId: number, value: string | null): Promise<void>;
  onLine(h: (e: { id: string; line: string }) => void): Promise<Unlisten>;
  onClosed(h: (e: ClosedEvent) => void): Promise<Unlisten>;
  onAskpass(h: (e: AskpassRequest) => void): Promise<Unlisten>;
  /** 答えが済んだ（時間切れ・接続が閉じた）質問の requestId */
  onAskpassDone(h: (requestId: number) => void): Promise<Unlisten>;
}

/** 「このマシン」。ssh を挟まず shj attach を直接起動する。ssh のホスト名とは取り違えない形にしてある。 */
export const LOCAL = "@local";
