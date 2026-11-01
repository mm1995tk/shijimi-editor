// 1 台のマシン（1 つの Session）の、ワークスペースごとのツリー（D2）。状態は model.ts、ここは送り受け。
//
// - フォーカスしているワークスペースだけ tree.watch で開いているフォルダを教え、tree.changed で取り直す。
//   フォーカスを外したら監視をやめる（tree.watch に空を送る）が、開閉・取った中身・スクロールの位置は覚えておく。
//   戻ったら、覚えている中身をすぐ出しつつ、開いているフォルダと git の印を取り直す。
// - tree.watch は、取り直す前に送る（送ったときの中身が監視の基準になるので、間の変化を取りこぼさない）。
// - つなぎ直したら（reconnected）、今のワークスペースを同じように取り直す。
// Solid には依存しない。変わったら onChange を呼ぶ。

import { ErrorCode, RpcError } from "../rpc/message.ts";
import type { Notifications, Requests, WorkspaceId } from "../rpc/protocol.ts";
import type { TreeMethod } from "../ssh/session.ts";
import {
  applyGitChanged,
  applyGitStatus,
  applyList,
  applyTreeChanged,
  dropDir,
  emptyTree,
  forgetGit,
  ROOT,
  toggle,
  watchPaths,
  type TreeState,
} from "./model.ts";

/** ツリーの要求を送る先（Session.request）。 */
export type TreeRequest = <M extends TreeMethod>(
  method: M,
  params: Requests[M]["params"],
) => Promise<Requests[M]["result"]>;

export class WorkspaceTrees {
  private readonly send: TreeRequest;
  private readonly onChange: () => void;
  private readonly onError: (message: string) => void;
  private readonly states = new Map<WorkspaceId, TreeState>();
  private active: WorkspaceId | null = null;
  /** tree.watch を 1 つずつ送る（前に送ったものと入れ違わないように） */
  private watchChain: Promise<void> = Promise.resolve();

  constructor(opts: {
    request: TreeRequest;
    onChange: () => void;
    /** 取れなかったときの知らせ（ログ用）。省くと何もしない */
    onError?: (message: string) => void;
  }) {
    this.send = opts.request;
    this.onChange = opts.onChange;
    this.onError = opts.onError ?? (() => {});
  }

  /** そのワークスペースのツリー。まだ一度もフォーカスしていなければ undefined。 */
  get(id: WorkspaceId): TreeState | undefined {
    return this.states.get(id);
  }

  /** 今のワークスペース（このマシンにフォーカスが無ければ null）。 */
  get activeId(): WorkspaceId | null {
    return this.active;
  }

  /** フォーカスしているワークスペースを変える（このマシンのものでなければ null）。 */
  activate(id: WorkspaceId | null): void {
    if (id === this.active) return;
    const prev = this.active;
    this.active = id;
    if (prev !== null && this.states.has(prev)) this.watch(prev);
    if (id === null) return;
    if (!this.states.has(id)) {
      this.states.set(id, emptyTree());
      this.onChange();
    }
    void this.refresh(id);
  }

  /** つなぎ直した。今のワークスペースを取り直す（git の版は shj ごとなので忘れる）。 */
  reconnected(): void {
    for (const [id, s] of this.states) this.states.set(id, forgetGit(s));
    this.watchChain = Promise.resolve();
    if (this.active !== null) void this.refresh(this.active);
  }

  /** 一覧から消えたワークスペースを忘れる。 */
  retain(ids: WorkspaceId[]): void {
    const keep = new Set(ids);
    let changed = false;
    for (const id of [...this.states.keys()]) {
      if (!keep.has(id)) {
        this.states.delete(id);
        changed = true;
      }
    }
    if (this.active !== null && !keep.has(this.active)) this.active = null;
    if (changed) this.onChange();
  }

  /** フォルダを開く・閉じる。開いたら監視に足して中身を取る。 */
  toggle(id: WorkspaceId, dir: string): void {
    const s = this.states.get(id);
    if (!s) return;
    const next = toggle(s, dir);
    this.update(id, next);
    if (id !== this.active) return;
    const opened = !!next.expanded[dir];
    void this.watch(id).then(() => {
      if (opened) return this.list(id, dir);
    });
  }

  /** スクロールの位置を覚える。 */
  setScroll(id: WorkspaceId, top: number): void {
    const s = this.states.get(id);
    if (s && s.scrollTop !== top) this.states.set(id, { ...s, scrollTop: top });
  }

  handleTreeChanged(n: Notifications["tree.changed"]): void {
    const s = this.states.get(n.workspaceId);
    if (!s) return;
    const { state, refetch } = applyTreeChanged(s, n.paths, n.rev);
    this.states.set(n.workspaceId, state);
    if (n.workspaceId !== this.active) return;
    for (const p of refetch) void this.list(n.workspaceId, p);
  }

  handleGitChanged(n: Notifications["git.changed"]): void {
    const s = this.states.get(n.workspaceId);
    if (!s) return;
    this.update(n.workspaceId, applyGitChanged(s, n));
  }

  private update(id: WorkspaceId, next: TreeState): void {
    if (this.states.get(id) === next) return;
    this.states.set(id, next);
    this.onChange();
  }

  /** 監視を教え直してから、開いているフォルダ全部と git の印を取り直す。 */
  private async refresh(id: WorkspaceId): Promise<void> {
    await this.watch(id);
    if (id !== this.active) return;
    const s = this.states.get(id);
    if (!s) return;
    await Promise.all([
      ...watchPaths(s).map((p) => this.list(id, p)),
      this.gitStatus(id),
    ]);
    // 消えていたフォルダを除いたので、監視を教え直す
    const after = this.states.get(id);
    if (
      after &&
      id === this.active &&
      watchPaths(after).length !== watchPaths(s).length
    ) {
      await this.watch(id);
    }
  }

  /**
   * tree.watch を送る。今のワークスペースなら開いているフォルダを、そうでなければ空を送る
   * （送る直前の状態で決める）。
   */
  private watch(id: WorkspaceId): Promise<void> {
    const run = async () => {
      const s = this.states.get(id);
      if (!s) return;
      const paths = id === this.active ? watchPaths(s) : [];
      try {
        await this.send("tree.watch", { workspaceId: id, paths });
      } catch (e) {
        this.onError(`tree.watch failed: ${message(e)}`);
      }
    };
    const p = this.watchChain.then(run);
    this.watchChain = p;
    return p;
  }

  private async list(id: WorkspaceId, dir: string): Promise<void> {
    try {
      const r = await this.send("tree.list", { workspaceId: id, path: dir });
      const s = this.states.get(id);
      if (s) this.update(id, applyList(s, dir, r));
    } catch (e) {
      // フォルダが消えた・入れない：開いていた印ごと忘れる（tree.watch が消えたパスで断られないように）
      if (dir !== ROOT && isGone(e)) {
        const s = this.states.get(id);
        if (s) this.update(id, dropDir(s, dir));
        return;
      }
      this.onError(`tree.list ${dir || "/"} failed: ${message(e)}`);
    }
  }

  private async gitStatus(id: WorkspaceId): Promise<void> {
    try {
      const r = await this.send("git.status", { workspaceId: id });
      const s = this.states.get(id);
      if (s) this.update(id, applyGitStatus(s, r));
    } catch (e) {
      // git のリポジトリでなければ印は出さない（tree.list の git をそのまま使う）
      this.onError(`git.status failed: ${message(e)}`);
    }
  }
}

function isGone(e: unknown): boolean {
  return (
    e instanceof RpcError &&
    (e.code === ErrorCode.NotFound || e.code === ErrorCode.Denied)
  );
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}
