// Package screen は「画面に何を出すか」のデータ型だけを持つ（設計書 docs/design/gui-protocol.md の 1 章）。
//
// 中身（core）が作り、TUI の描画とプロトコルが読む。標準ライブラリ以外に依存しない。
// 色は RGB ではなくテーマ上の名前（Style）で持ち、名前 → 色の対応は Palette が表す。
// 位置は画面全体の座標ではなく「ペインの中の行と桁」で持つ。文言はローカライズ済み。
package screen

// Region は領域。Screen.Focus（今キーを受ける領域）などに使う。
type Region string

const (
	RegionFiles    Region = "files"
	RegionExplorer Region = "explorer"
	RegionChanges  Region = "changes"
	RegionAgent    Region = "agent"
	RegionTerminal Region = "terminal"
	RegionCmdline  Region = "cmdline"
	RegionOverlay  Region = "overlay"
)

// Screen は、ある時点で画面に出すもの全部。
type Screen struct {
	Rev     uint64    `json:"rev"`               // 作るたびに増える。GUI はこれで抜けを見つける
	AckSeq  uint64    `json:"ackSeq"`            // どの入力（Input.Seq）までがこの画面に入っているか
	Focus   Region    `json:"focus"`             // 今キーを受ける領域
	Files   *Files    `json:"files,omitempty"`   // ファイルの欄
	Status  Status    `json:"status"`            // 状態行
	Cmdline *Cmdline  `json:"cmdline,omitempty"` // ":" と "/" の入力行。無ければ nil
	Overlay []Overlay `json:"overlay,omitempty"` // 補完・ホバー・シグネチャ・確認・パレット・メニュー・レビューカード

	// TUI だけが使う領域。GUI は購読しない。
	Explorer *Explorer `json:"explorer,omitempty"`
	Changes  *Changes  `json:"changes,omitempty"`
	Agent    *Agent    `json:"agent,omitempty"`
}

// Files はファイルの欄。普段は Pane が 1 つ、必要なら左右に 2 つ。
type Files struct {
	Panes  []Pane `json:"panes"`
	Active int    `json:"active"`
}

// PaneKind は Pane の中身の種類。
type PaneKind string

const (
	KindText    PaneKind = "text"
	KindImage   PaneKind = "image"
	KindDiff    PaneKind = "diff"
	KindPreview PaneKind = "preview"
	KindWelcome PaneKind = "welcome"
)

// Pane はファイルの欄の 1 つの区画。差分は左右の Pane を 2 つ返す。
type Pane struct {
	ID         string    `json:"id"`
	Title      Title     `json:"title"`
	Kind       PaneKind  `json:"kind"`
	Viewport   Viewport  `json:"viewport"`
	Lines      []Line    `json:"lines,omitempty"`      // 見えている行だけ
	CurLine    int       `json:"curLine"`              // カーソルのある行（バッファの行番号）。行の強調と相対行番号の基準。Cursors の描く位置とは別（選択が前向きのとき、描く位置は 1 つ前の文字になる）
	Cursors    []Pos     `json:"cursors,omitempty"`    // 描くカーソルの位置。先頭が主のカーソルで、残りが複数カーソルの追加分
	Selections []Range   `json:"selections,omitempty"` // Cursors と同じ並び。選択が無いカーソルは Start == End の空の範囲
	Image      *ImageRef `json:"image,omitempty"`      // Kind が KindImage のとき。実体は blob.get で取る
	Diff       *DiffInfo `json:"diff,omitempty"`       // Kind が KindDiff のとき
}

// DiffInfo は差分の Pane (Kind が KindDiff) の追加の項目。
//
// 差分は、変更の前（SideOld）と後（SideNew）の Pane を左右に 2 つ返す。統合表示のときと、相手が無い
// （新しいファイル）ときは、前後を行ごとに並べた Pane（SideUnified）を 1 つ返す。
// どの Pane も、Lines は見えている「表示行」を上から並べたもので、左右の Pane は同じ表示行を同じ
// 並びで持つ（相手側にだけ行がある表示行は Line.Blank）。統合表示で消えた行は Marks に MarkDeleted を
// 持ち、No は変更前のファイルの行番号になる。変更後の行は、変更の一部なら MarkAdded か MarkModified
// を持つ。表示行は変更前と後の両方の行を数えるので、バッファの行番号とは数えが違う。
type DiffInfo struct {
	Side  DiffSide `json:"side"`
	Top   int      `json:"top"`   // Lines[0] の表示行
	Rows  int      `json:"rows"`  // 表示行の全部の数
	Hunks []Hunk   `json:"hunks"` // 変更のある表示行の範囲。Start の小さい順で、重ならない
}

// DiffSide は差分の Pane がどちら側か。
type DiffSide string

const (
	SideOld     DiffSide = "old"
	SideNew     DiffSide = "new"
	SideUnified DiffSide = "unified"
)

// Hunk は変更のある表示行の範囲 [Start, End)。Kind は MarkAdded（追加だけ）、MarkDeleted（削除だけ）、
// MarkModified（両方）のどれか。
type Hunk struct {
	Start int      `json:"start"`
	End   int      `json:"end"`
	Kind  MarkKind `json:"kind"`
}

// Title はペインのタイトル行。
type Title struct {
	Text     string `json:"text"` // ファイル名など
	Dirty    bool   `json:"dirty,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
	Peer     string `json:"peer,omitempty"` // 差分の相手など

	// 差分の Pane（変更後の側）のタイトル。
	Added    int    `json:"added,omitempty"`    // 足した行の数
	Deleted  int    `json:"deleted,omitempty"`  // 消した行の数
	Note     string `json:"note,omitempty"`     // レビューの注記
	NoteWarn bool   `json:"noteWarn,omitempty"` // 注記が警告（PR の先頭と作業ツリーが食い違っている）
}

// Viewport は見えている範囲。行数と桁数は GUI が送り、スクロールは shj が決める。
type Viewport struct {
	Top   int `json:"top"`   // 先頭の行（バッファの行番号）
	Rows  int `json:"rows"`  // 見える行数
	Cols  int `json:"cols"`  // 見える桁数
	Left  int `json:"left"`  // 横スクロール量（桁）
	Total int `json:"total"` // バッファの全行数
}

// ImageRef は画像の参照。
type ImageRef struct {
	Blob   string `json:"blob"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Pos はバッファの行番号 + その行の中の rune の位置（どちらも 0 始まり）。
type Pos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

// Range は [Start, End) の範囲。
type Range struct {
	Start Pos `json:"start"`
	End   Pos `json:"end"`
}

// Line は表示する 1 行。
type Line struct {
	No     int          `json:"no"` // バッファの行番号（0 始まり）
	Spans  []Span       `json:"spans"`
	Marks  []Mark       `json:"marks,omitempty"` // 行番号の横の印
	Diag   []Diagnostic `json:"diag,omitempty"`
	Guides []int        `json:"guides,omitempty"` // インデントガイドを引く桁（表示上の桁、小さい順）。文字の上でも、空行の行末より右でも引く
	Blank  bool         `json:"blank,omitempty"`  // 差分で、相手側にだけ行がある表示行の空き。No は -1
	Words  []Cols       `json:"words,omitempty"`  // 差分で、行の中の変わった部分（rune の位置）
	Indent int          `json:"indent,omitempty"` // ガイドが及ぶ幅（桁）。空行は前後の行の深いほうから決まる。ガイドを引かないときは 0
}

// Cols は行の中の rune の範囲 [Start, End)。
type Cols struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Span は同じスタイルで描く文字列の区間。
type Span struct {
	Text  string `json:"text"`
	Style Style  `json:"style,omitempty"` // 空はテーマの標準の文字色
}

// Mark は行番号の横の印。
type Mark struct {
	Kind  MarkKind `json:"kind"`
	Style Style    `json:"style,omitempty"`
}

// MarkKind は印の種類。
type MarkKind string

const (
	MarkAdded    MarkKind = "added"
	MarkModified MarkKind = "modified"
	MarkDeleted  MarkKind = "deleted"
	MarkDiag     MarkKind = "diag"
)

// Severity は診断の重さ（LSP の DiagnosticSeverity と同じ数）。
type Severity int

const (
	SeverityError Severity = iota + 1
	SeverityWarning
	SeverityInfo
	SeverityHint
)

// Diagnostic は 1 つの診断。範囲は行の中の rune の位置。
type Diagnostic struct {
	Start    int      `json:"start"`
	End      int      `json:"end"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Cont     bool     `json:"cont,omitempty"` // 前の行から続く診断。下線だけ引き、行末の文言と行番号の横の印には使わない
}

// Status は状態行。文言はローカライズ済み。左（モード・領域・ブランチ・診断・メッセージ）と
// 右（位置・字下げ・言語・版）に分かれる。右は Position が空なら（テキスト以外のとき）出さない。
type Status struct {
	Mode    string `json:"mode"`              // モードの表示（"NOR" など）。空ならモードの欄を出さない
	Pending string `json:"pending,omitempty"` // 途中まで押されたキー（"space" など）
	Style   Style  `json:"style"`             // モードの色（StyleModeNormal など）

	Pane        string `json:"pane,omitempty"`        // キーを受けているエディタ以外の領域の名前
	Branch      string `json:"branch,omitempty"`      // git のブランチ
	Errors      int    `json:"errors,omitempty"`      // 診断の数
	Warnings    int    `json:"warnings,omitempty"`    // 診断の数
	Suggestions string `json:"suggestions,omitempty"` // エージェントの提案の数（提案の欄が見えていないときだけ）

	Msg      string `json:"msg,omitempty"`      // メッセージ（保存の結果、同期の状況など）
	MsgErr   bool   `json:"msgErr,omitempty"`   // Msg がエラー
	Question string `json:"question,omitempty"` // 確認の問い。あれば Msg より優先し、右側を隠す

	Cursor   Pos    `json:"cursor"`             // カーソルの位置
	Position string `json:"position,omitempty"` // 「行:桁」に複数カーソルと選択の文字数を足したもの
	Indent   string `json:"indent,omitempty"`   // 字下げの種類
	Language string `json:"language,omitempty"` // 言語と言語サーバーの状態
	Unsent   string `json:"unsent,omitempty"`   // 未送信のレビューコメントの数
	Version  string `json:"version,omitempty"`  // shj の版
}

// Cmdline は ":" と "/" の入力行。検索バー（Ctrl+F）も同じ形で、ラベルと案内が付く。
type Cmdline struct {
	Prefix string `json:"prefix"` // ":" か "/" か "?"（検索バーはラベル）
	Text   string `json:"text"`
	Cursor int    `json:"cursor"`         // rune の位置
	Hint   string `json:"hint,omitempty"` // 検索バーの右の案内
}

// OverlayKind は Overlay の種類。
type OverlayKind string

const (
	OverlayCompletion OverlayKind = "completion"
	OverlayHover      OverlayKind = "hover"
	OverlaySignature  OverlayKind = "signature"
	OverlayConfirm    OverlayKind = "confirm"
	OverlayPalette    OverlayKind = "palette"
	OverlayMenu       OverlayKind = "menu"
	OverlayReviewCard OverlayKind = "reviewCard"
)

// Overlay はペインの上に重ねるもの。At はペインの中の位置（無ければ中央など描く側が決める）。
type Overlay struct {
	Kind   OverlayKind `json:"kind"`
	Pane   string      `json:"pane,omitempty"` // 重ねる先の Pane.ID
	At     *Pos        `json:"at,omitempty"`
	Title  string      `json:"title,omitempty"`
	Lines  []Line      `json:"lines,omitempty"`
	Items  []Item      `json:"items,omitempty"`
	Active int         `json:"active"` // Items の選択位置
}

// Item は一覧の 1 行（補完の候補、パレットの項目、メニューなど）。
type Item struct {
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
	Kind   string `json:"kind,omitempty"`
}

// Explorer, Changes, Agent は TUI だけが使う領域。中身は後の手順で決める。
type (
	Explorer struct {
		Rows   []Line `json:"rows"`
		Active int    `json:"active"`
	}
	Changes struct {
		Rows   []Line `json:"rows"`
		Active int    `json:"active"`
	}
	Agent struct {
		Rows []Line `json:"rows"`
	}
)
