// Package screen は「画面に何を出すか」のデータ型だけを持つ（設計書 docs/design/gui-protocol.md の 1 章）。
//
// 中身（core）が作り、TUI の描画とプロトコルが読む。標準ライブラリと msg（キーと値）以外に依存しない。
// 色は RGB ではなくテーマ上の名前（Style）で持ち、名前 → 色の対応は Palette が表す。
// 位置は画面全体の座標ではなく「ペインの中の行と桁」で持つ。文言はキーと値の msg.Msg で持ち、訳すのは UI（TUI は描くとき、server は送る直前）。
package screen

import "github.com/mm1995tk/shijimi-editor/core/msg"

// Region は領域。Screen.Focus（今キーを受ける領域）などに使う。
type Region string

const (
	RegionFiles    Region = "files"
	RegionExplorer Region = "explorer"
	RegionChanges  Region = "changes"
	RegionAgent    Region = "agent"
	RegionTerminal Region = "terminal"
	RegionStatus   Region = "status"
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
	ID         string       `json:"id"`
	Title      Title        `json:"title"`
	Kind       PaneKind     `json:"kind"`
	Viewport   Viewport     `json:"viewport"`
	Lines      []Line       `json:"lines,omitempty"`      // 見えている行だけ
	CurLine    int          `json:"curLine"`              // カーソルのある行（バッファの行番号）。行の強調と相対行番号の基準。Cursors の描く位置とは別（選択が前向きのとき、描く位置は 1 つ前の文字になる）
	Cursors    []Pos        `json:"cursors,omitempty"`    // 描くカーソルの位置。先頭が主のカーソルで、残りが複数カーソルの追加分
	Selections []Range      `json:"selections,omitempty"` // Cursors と同じ並び。選択が無いカーソルは Start == End の空の範囲
	Image      *ImageRef    `json:"image,omitempty"`      // Kind が KindImage のとき。実体は blob.get で取る。読めなかったときは nil
	Caption    msg.Msg      `json:"caption,omitzero"`     // Kind が KindImage のとき、画像の説明の行（形式・大きさ・容量。読めなかったときはその理由）
	Diff       *DiffInfo    `json:"diff,omitempty"`       // Kind が KindDiff のとき
	Preview    *PreviewInfo `json:"preview,omitempty"`    // Kind が KindPreview のとき
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

// PreviewInfo は Markdown プレビューの Pane (Kind が KindPreview) の追加の項目。
//
// プレビューは文書を Pane の幅（Viewport.Cols）で折り返して作った「表示行」を上から並べたもので、Lines は
// そのうち見えている分（Top から Viewport.Rows 行まで）だけ。Line.No は文書の中の表示行の番号で、バッファの
// 行番号ではない。画像（ローカルの画像と mermaid の図）は Lines に埋め込まず、場所だけ Images で指す。
// 実体は blob.get で取る。
type PreviewInfo struct {
	Top    int            `json:"top"`    // Lines[0] の表示行
	Rows   int            `json:"rows"`   // 表示行の全部の数
	Images []PreviewImage `json:"images"` // 画像の場所。Row の小さい順で、重ならない
}

// PreviewImage はプレビューの中の画像 1 つ。表示行 [Row, Row+Rows) を使う。この範囲の Lines は空の行。
// 画像を作れていないとき（読み込み中・失敗）は画像ではなく説明の文字の行になり、ここには入らない。
type PreviewImage struct {
	Row   int      `json:"row"`
	Rows  int      `json:"rows"`
	Col   int      `json:"col,omitempty"` // 左に空ける桁（引用の中の画像）。この桁の分は Lines の文字が見える
	Image ImageRef `json:"image"`
}

// Title はペインのタイトル行。
type Title struct {
	Text     string `json:"text"` // ファイル名など
	Dirty    bool   `json:"dirty,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
	Peer     string `json:"peer,omitempty"` // 差分の相手など

	// 差分の Pane（変更後の側）のタイトル。
	Added    int     `json:"added,omitempty"`    // 足した行の数
	Deleted  int     `json:"deleted,omitempty"`  // 消した行の数
	Note     msg.Msg `json:"note,omitzero"`      // レビューの注記
	NoteWarn bool    `json:"noteWarn,omitempty"` // 注記が警告（PR の先頭と作業ツリーが食い違っている）
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

	Format string `json:"format,omitempty"` // 画像の形式（png など）。Pane.Kind が KindImage のとき
	Size   int64  `json:"size,omitempty"`   // ファイルの大きさ（バイト）。同上
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
	Band   Style        `json:"band,omitempty"`   // 行の背景の色（ホバーのコードブロック）。空は標準
	Rule   bool         `json:"rule,omitempty"`   // 水平線（ホバー）。幅いっぱいに引く。Spans は空
}

// Cols は行の中の rune の範囲 [Start, End)。
type Cols struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Span は同じスタイルで描く文字列の区間。
type Span struct {
	Text      string `json:"text"`
	Style     Style  `json:"style,omitempty"` // 文字の色。空はテーマの標準の文字色
	Bg        Style  `json:"bg,omitempty"`    // 背景の色（Style の色をそのまま背景に使う）。空は行の背景
	Bold      bool   `json:"bold,omitempty"`
	Italic    bool   `json:"italic,omitempty"`
	Underline bool   `json:"underline,omitempty"`
	Strike    bool   `json:"strike,omitempty"`
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

// Status は状態行。文言はキーと値の msg.Msg で持ち、訳すのは UI（TUI は描くとき、server は送る直前）。左（モード・領域・ブランチ・診断・メッセージ）と
// 右（位置・字下げ・言語・版）に分かれる。右は Position が空なら（テキスト以外のとき）出さない。
type Status struct {
	Mode    string `json:"mode"`              // モードの表示（"NOR" など）。空ならモードの欄を出さない
	Pending string `json:"pending,omitempty"` // 途中まで押されたキー（"space" など）
	Style   Style  `json:"style"`             // モードの色（StyleModeNormal など）

	Pane        msg.Msg `json:"pane,omitzero"`        // キーを受けているエディタ以外の領域の名前
	Branch      string  `json:"branch,omitempty"`     // git のブランチ
	Errors      int     `json:"errors,omitempty"`     // 診断の数
	Warnings    int     `json:"warnings,omitempty"`   // 診断の数
	Suggestions msg.Msg `json:"suggestions,omitzero"` // エージェントの提案の数（提案の欄が見えていないときだけ）

	Msg    msg.Msg `json:"msg,omitzero"`     // メッセージ（保存の結果、同期の状況など）
	MsgErr bool    `json:"msgErr,omitempty"` // Msg がエラー

	Cursor   Pos     `json:"cursor"`            // カーソルの位置
	Position msg.Msg `json:"position,omitzero"` // 「行:桁」に複数カーソルと選択の文字数を足したもの
	Indent   msg.Msg `json:"indent,omitzero"`   // 字下げの種類
	Language msg.Msg `json:"language,omitzero"` // 言語と言語サーバーの状態
	Unsent   msg.Msg `json:"unsent,omitzero"`   // 未送信のレビューコメントの数
	Version  string  `json:"version,omitempty"` // shj の版
}

// Cmdline は ":" と "/" の入力行。検索バー（Ctrl+F）も同じ形で、ラベルと案内が付く。
type Cmdline struct {
	Prefix msg.Msg `json:"prefix"` // ":" か "/" か "?"（検索バーはラベル）
	Text   string  `json:"text"`
	Cursor int     `json:"cursor"`        // rune の位置
	Hint   msg.Msg `json:"hint,omitzero"` // 検索バーの右の案内
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
	OverlaySearch     OverlayKind = "search"
	OverlayProposal   OverlayKind = "proposal"
)

// Overlay はペインの上に重ねるもの。At はペインの中の位置（無ければ中央など描く側が決める）。
type Overlay struct {
	Kind   OverlayKind `json:"kind"`
	Pane   string      `json:"pane,omitempty"` // 重ねる先の Pane.ID
	At     *Pos        `json:"at,omitempty"`
	Title  msg.Msg     `json:"title,omitzero"`
	Lines  []Line      `json:"lines,omitempty"`
	Items  []Item      `json:"items,omitempty"`
	Active int         `json:"active"` // Items の選択位置

	// パレットが使う。入力欄。Title は見出し（プロンプトの文言や選ぶものの名前）、Hint は候補が無いときの案内。
	Field *Field `json:"field,omitempty"`

	// ホバーとレビューカードが使う。Lines には見えている行だけが入り、View がその位置を表す
	// （Top は先頭の行、Rows は窓の高さ、Cols は文字の幅、Total は全体の行数）。
	View      *Viewport `json:"view,omitempty"`
	Hint      msg.Msg   `json:"hint,omitzero"`      // 枠の下の案内
	CopyLabel msg.Msg   `json:"copyLabel,omitzero"` // 枠の上の右端の「コピー」の押すところ

	// レビューカードが使う。Images は Lines の中の画像。Row は Line.No と同じ表示行の番号（Lines のインデックスではない）で、見える範囲に掛かるものだけ入る。Compose は返信・コメントの編集のときだけ。このとき View.Rows はラベルを含むプレビューの高さ（Lines は最大 Rows-1 行）で、Total は使わない。
	Images  []PreviewImage `json:"images,omitempty"`
	Compose *Compose       `json:"compose,omitempty"`

	// エージェントの変更の承認の窓（OverlayProposal）が使う。Title は見出し（待っている数を含む）、Lines は見えている差分の行
	// （追加・削除の行は Band に背景を付ける）、View がその位置と窓の幅（Cols）を表す。差分に行が無いときは Lines の代わりに Empty が入る。
	Empty msg.Msg `json:"empty,omitzero"`

	// 検索の窓（プロジェクトの検索・置換）が使う。Title は窓の見出し、Hint は状態の行の右に出すキーの一覧。
	Search *Search `json:"search,omitempty"`
}

// Search は検索の窓（OverlaySearch）の状態。上に 4 つの入力欄（検索・置換・含める・除く）とオプション、
// 状態の行があり、下の左に一致した行の一覧、右に選んだ行のプレビューが並ぶ。窓は欄いっぱい（周りに 1 マスの余白）に広げる。
type Search struct {
	Fields  [4]SearchField  `json:"fields"`  // 検索・置換・含める・除くの順（Tab の順）
	Field   int             `json:"field"`   // 今の欄（Fields の番号）
	Options [4]SearchOption `json:"options"` // 大文字小文字・単語・正規表現・大文字小文字を保つの順
	Status  msg.Msg         `json:"status"`  // 状態の行（件数、検索中、エラーなど）
	Err     bool            `json:"err,omitempty"`
	Busy    bool            `json:"busy,omitempty"` // 検索中

	// 一覧とプレビューの高さ（行）。Hits は Scroll から見えている分だけで、Cursor は全体の中の選択位置。
	BodyRows int         `json:"bodyRows"`
	Hits     []SearchHit `json:"hits,omitempty"`
	Scroll   int         `json:"scroll"`
	Cursor   int         `json:"cursor"`
	Total    int         `json:"total"`          // 一致した行の全部の数
	Empty    msg.Msg     `json:"empty,omitzero"` // 一致した行が無いときの文言

	Preview *SearchPreview `json:"preview,omitempty"` // 選んだ行があるときだけ
}

// SearchField は検索の窓の入力欄。
type SearchField struct {
	Label       msg.Msg `json:"label"`
	Text        string  `json:"text"`
	Cursor      int     `json:"cursor"` // rune の位置
	Placeholder msg.Msg `json:"placeholder,omitzero"`
}

// SearchOption は検索のオプション（切り替え）。
type SearchOption struct {
	Label msg.Msg `json:"label"`
	On    bool    `json:"on"`
}

// SearchHit は一致した行の一覧の 1 行。Line は、一致した部分に背景（StyleMatch）を付けた、見せる最初の桁からの行。
type SearchHit struct {
	Label string `json:"label"` // 「相対パス:行番号」
	Line  Line   `json:"line"`
	Skip  bool   `json:"skip,omitempty"` // 置換から外した行
}

// SearchPreview は選んだ行のプレビュー。Lines はファイルの中の見える行（No は 0 始まりの行番号）で、
// Hit の行には一致した部分に背景を付ける。置換のあとの行が違うときは Del と Add に前と後の行を入れる（Hit の行の下に出す）。
type SearchPreview struct {
	Title string `json:"title"` // 「相対パス:行番号」
	Lines []Line `json:"lines"`
	Hit   int    `json:"hit"`   // 選んだ行の No
	Total int    `json:"total"` // ファイルの全行数（行番号の桁数に使う）
	Del   *Line  `json:"del,omitempty"`
	Add   *Line  `json:"add,omitempty"`
}

// Compose はレビューカードの編集（返信・コメント・レビューの本文）の入力欄。Overlay の Lines は、その本文を
// Markdown として描いたもの（プレビュー）になる。
type Compose struct {
	Text         string  `json:"text"`
	Cursor       Pos     `json:"cursor"`                // 入力欄の中の行と桁（rune）
	PreviewLabel msg.Msg `json:"previewLabel,omitzero"` // プレビューの見出し。置き場所が無いときは空
}

// Item は一覧の 1 行（補完の候補、パレットの項目、メニューなど）。
type Item struct {
	Label   msg.Msg `json:"label"`
	Detail  msg.Msg `json:"detail,omitzero"`
	Kind    msg.Msg `json:"kind,omitzero"`
	Matched []int   `json:"matched,omitempty"` // Label の中で検索に合った文字の位置（rune、小さい順）
}

// Field は Overlay の 1 行の入力欄（パレット）。
type Field struct {
	Prefix      string  `json:"prefix"` // 見出しの後ろの印。":"（文言を聞く）か ">"（探す・選ぶ）
	Text        string  `json:"text"`
	Cursor      int     `json:"cursor"`               // rune の位置
	Placeholder msg.Msg `json:"placeholder,omitzero"` // Text が空のときに見せる薄い文言
}

// Size は領域の大きさ（等幅のマス目で数える）。
type Size struct {
	Cols int `json:"cols"`
	Rows int `json:"rows,omitempty"`
}

// View は GUI が view.resize で送る、領域ごとの大きさ。Files.Rows はペインの本文の行数で、タイトル行は含めない。
// ペインを 2 つ並べるときの割り方は core が Files の中で決める。
type View struct {
	Files   Size `json:"files"`
	Status  Size `json:"status"`
	Cmdline Size `json:"cmdline"`
	Overlay Size `json:"overlay"`
	// DiffPct と PreviewPct は、ユーザーがドラッグで決めた左右の境界の位置。左側（差分の元の側・プレビューの横の
	// テキスト）が、間の 1 桁を除いた幅の何 % か。0 なら半分。
	DiffPct    int `json:"diffPct,omitempty"`
	PreviewPct int `json:"previewPct,omitempty"`
}

// DefaultView は GUI が view.resize を送るまでの大きさ。
var DefaultView = View{
	Files:   Size{Cols: 80, Rows: 22},
	Status:  Size{Cols: 80},
	Cmdline: Size{Cols: 80},
	Overlay: Size{Cols: 80, Rows: 22},
}
