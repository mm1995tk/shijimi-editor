package ui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/diff"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// Suggestions in the right panel. They come from rules over git, LSP and
// the terminal panel, never from the AI: that keeps them instant and free.
// Choosing one hands a prompt to the agent (agent.go).

type suggestKind int

const (
	sgApplyFix   suggestKind = iota // apply the fix the failure analysis proposed
	sgConflict                      // conflict markers are left in files
	sgCreatePR                      // the branch has commits and no pull request
	sgReviewMode                    // the branch has a pull request
	sgFailure                       // a command in the terminal panel failed
	sgLSPErrors                     // LSP reports errors
	sgAIReview                      // a pull request is shown in diff mode
)

// offer is one row of the panel. key names the condition: dismissing
// hides the suggestion until the condition goes away.
type offer struct {
	kind   suggestKind
	key    string
	title  string
	detail string
}

// suggestState is what the rules look at, gathered in the background.
type suggestState struct {
	sel       int
	dismissed map[string]bool
	repo      repoProbe
	probing   bool
	ticks     int // watch ticks since the last probe
	prTicks   int // watch ticks since pull requests were last asked for
	fails     []termFailure
	failSeq   int
	fixOffer  bool // the last failure analysis ended with a fix to apply
	fixSeq    int
}

// repoProbe is the state of the repository the rules need.
type repoProbe struct {
	branch    string
	head      string
	base      string   // the default branch, e.g. origin/main
	ahead     int      // commits on HEAD that base lacks
	conflicts []string // files with conflict markers, from the repository top
	prKnown   bool     // gh answered for branch and head
	prs       []prSummary
}

// termFailure is a failed command seen in the terminal panel.
type termFailure struct {
	seq    int
	tab    int // termTab.id
	n      int // number shown on the terminal tab
	code   int // exit code, or -1 when a test reported FAIL
	line   string
	output []string // the end of the terminal at the time
}

const (
	probeTicks   = 5  // watch ticks between repository probes
	prProbeTicks = 60 // watch ticks between pull request lookups
	failTail     = 200
)

// failLine matches the summary lines test runners print on failure.
var failLine = regexp.MustCompile(`^\s*(--- FAIL|FAIL\b|FAILED\b|test result: FAILED)|^=+ .*\b\d+ failed\b`)

// ---- gathering ----

type repoProbedMsg struct{ probe repoProbe }

// probeRepo looks at the repository in the background. Pull requests are
// asked for only when the branch or HEAD moved, or every prProbeTicks.
func (a *App) probeRepo() tea.Cmd {
	if a.gitTop == "" || a.sg.probing {
		return nil
	}
	a.sg.probing = true
	a.sg.ticks = 0
	root, prev := a.root, a.sg.repo
	askPR := a.sg.prTicks >= prProbeTicks || prev.head == ""
	if askPR {
		a.sg.prTicks = 0
	}
	return func() tea.Msg { return repoProbedMsg{probeRepoState(root, prev, askPR)} }
}

func probeRepoState(root string, prev repoProbe, askPR bool) repoProbe {
	var p repoProbe
	if out, err := run(root, nil, "git", "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		p.branch = strings.TrimSpace(string(out))
	}
	if out, err := run(root, nil, "git", "rev-parse", "HEAD"); err == nil {
		p.head = strings.TrimSpace(string(out))
	}
	p.conflicts = conflictFiles(root)
	p.base = defaultBranch(root)
	if p.base != "" && p.head != "" {
		if out, err := run(root, nil, "git", "rev-list", "--count", p.base+"..HEAD"); err == nil {
			p.ahead, _ = strconv.Atoi(strings.TrimSpace(string(out)))
		}
	}
	if p.branch == "" || p.branch == "HEAD" {
		return p
	}
	if !askPR && prev.branch == p.branch && prev.head == p.head {
		p.prKnown, p.prs = prev.prKnown, prev.prs
		return p
	}
	if prs, err := branchPRs(root, p.branch); err == nil {
		p.prKnown, p.prs = true, prs
	}
	return p
}

// conflictFiles lists unmerged files and files holding both ends of a
// conflict block, from the repository top.
func conflictFiles(root string) []string {
	var files []string
	if out, err := run(root, nil, "git", "diff", "--name-only", "--diff-filter=U"); err == nil {
		files = append(files, strings.Fields(string(out))...)
	}
	if out, err := run(root, nil, "git", "grep", "-l", "-I", "--full-name", "--all-match", "-E",
		"-e", "^<{7}( |$)", "-e", "^>{7}( |$)", "--", ":/"); err == nil {
		files = append(files, strings.Fields(string(out))...)
	}
	slices.Sort(files)
	return slices.Compact(files)
}

// defaultBranch is the branch pull requests usually go to, as a revision.
func defaultBranch(root string) string {
	if out, err := run(root, nil, "git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimSpace(string(out))
	}
	for _, r := range []string{"origin/main", "origin/master", "main", "master"} {
		if _, err := run(root, nil, "git", "rev-parse", "--verify", "-q", r); err == nil {
			return r
		}
	}
	return ""
}

func (a *App) handleRepoProbed(m repoProbedMsg) {
	a.sg.probing = false
	a.sg.repo = m.probe
	a.pruneDismissed()
}

// suggestTick runs on every watch tick: it reads the terminal panel and
// probes the repository now and then.
func (a *App) suggestTick() tea.Cmd {
	a.scanTerminals()
	a.pruneDismissed()
	a.sg.ticks++
	a.sg.prTicks++
	if a.sg.ticks >= probeTicks {
		return a.probeRepo()
	}
	return nil
}

// scanTerminals records failed commands: exit codes the shell reports
// (OSC 133;D, see shellIntegration) and FAIL lines of test runners.
func (a *App) scanTerminals() {
	alive := map[int]bool{}
	for _, t := range a.term.tabs {
		alive[t.id] = true
		for _, code := range t.vt.TakeCommandEnds() {
			switch code {
			case 0:
				a.clearFailure(t.id)
			case 130:
				// Ctrl+C: the user stopped it on purpose.
			default:
				a.addFailure(t, code, "")
			}
		}
		lines, next := t.vt.LinesSince(t.scanned)
		t.scanned = next
		for _, l := range lines {
			if failLine.MatchString(l) {
				a.addFailure(t, -1, strings.TrimSpace(l))
				break
			}
		}
	}
	a.sg.fails = slices.DeleteFunc(a.sg.fails, func(f termFailure) bool { return !alive[f.tab] })
}

// addFailure records a failure in tab t. A tab still failing keeps its
// record (and the key a dismissal refers to); only what is shown changes.
func (a *App) addFailure(t *termTab, code int, line string) {
	f := termFailure{tab: t.id, n: t.n, code: code, line: line, output: t.vt.Tail(failTail)}
	for i := range a.sg.fails {
		if a.sg.fails[i].tab == t.id {
			f.seq = a.sg.fails[i].seq
			a.sg.fails[i] = f
			return
		}
	}
	a.sg.failSeq++
	f.seq = a.sg.failSeq
	a.sg.fails = append(a.sg.fails, f)
}

func (a *App) clearFailure(tab int) {
	a.sg.fails = slices.DeleteFunc(a.sg.fails, func(f termFailure) bool { return f.tab == tab })
}

// lspErrors returns the error diagnostics of every file, sorted by path.
func (a *App) lspErrors() (paths []string, byPath map[string][]lsp.Diagnostic) {
	byPath = map[string][]lsp.Diagnostic{}
	for p := range a.lsp.diags {
		for _, d := range a.lsp.diagnostics(p) {
			if sevRank(d.Severity) == 1 {
				byPath[p] = append(byPath[p], d)
			}
		}
		if len(byPath[p]) > 0 {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	return paths, byPath
}

// ---- the rules ----

// allSuggestions applies the rules, dismissed ones included.
func (a *App) allSuggestions() []offer {
	var out []offer
	r := a.sg.repo
	if a.sg.fixOffer {
		out = append(out, offer{sgApplyFix, fmt.Sprintf("apply-fix:%d", a.sg.fixSeq), i18n.T("sg.applyfix"), i18n.T("sg.fixdetail")})
	}
	if n := len(r.conflicts); n > 0 {
		out = append(out, offer{sgConflict, "conflict", i18n.T("sg.conflicts"), fmt.Sprintf(i18n.T("sg.files"), n, strings.Join(r.conflicts, ", "))})
	}
	if r.prKnown && len(r.prs) == 0 && r.ahead > 0 && r.base != "" && !isBranchOf(r.base, r.branch) {
		out = append(out, offer{sgCreatePR, "create-pr:" + r.branch, i18n.T("sg.createpr"), fmt.Sprintf(i18n.T("sg.commits"), r.branch, r.ahead)})
	}
	if r.prKnown && len(r.prs) > 0 && !a.prShown() {
		p := r.prs[0]
		out = append(out, offer{sgReviewMode, fmt.Sprintf("review-mode:%s:%d", r.branch, p.Number), i18n.T("sg.reviewmode"), fmt.Sprintf("#%d %s", p.Number, p.Title)})
	}
	for _, f := range a.sg.fails {
		d := fmt.Sprintf(i18n.T("sg.termexit"), f.n, f.code)
		if f.code < 0 {
			d = fmt.Sprintf(i18n.T("sg.term"), f.n, f.line)
		}
		out = append(out, offer{sgFailure, fmt.Sprintf("fail:%d", f.seq), i18n.T("sg.investigate"), d})
	}
	if paths, byPath := a.lspErrors(); len(paths) > 0 {
		n := 0
		for _, p := range paths {
			n += len(byPath[p])
		}
		out = append(out, offer{sgLSPErrors, "lsp-errors", fmt.Sprintf(i18n.T("sg.fixerrors"), n), fmt.Sprintf(i18n.T("sg.files"), len(paths), a.rel(paths[0]))})
	}
	if a.prShown() {
		out = append(out, offer{sgAIReview, fmt.Sprintf("ai-review:%d", a.gd.pr.Number), i18n.T("sg.aireview"), fmt.Sprintf("#%d %s", a.gd.pr.Number, a.gd.pr.Title)})
	}
	return out
}

// isBranchOf reports whether revision base (e.g. origin/main) names branch.
func isBranchOf(base, branch string) bool {
	return base == branch || strings.HasSuffix(base, "/"+branch)
}

// suggestions returns the suggestions shown: the rules minus dismissed ones.
func (a *App) suggestions() []offer {
	all := a.allSuggestions()
	out := all[:0]
	for _, s := range all {
		if !a.sg.dismissed[s.key] {
			out = append(out, s)
		}
	}
	return out
}

// pruneDismissed forgets dismissals whose condition has gone, so the
// suggestion comes back when the condition does.
func (a *App) pruneDismissed() {
	if len(a.sg.dismissed) == 0 {
		return
	}
	live := map[string]bool{}
	for _, s := range a.allSuggestions() {
		live[s.key] = true
	}
	for k := range a.sg.dismissed {
		if !live[k] {
			delete(a.sg.dismissed, k)
		}
	}
}

func (a *App) dismissSuggestion() {
	list := a.suggestions()
	if len(list) == 0 {
		return
	}
	s := list[clamp(a.sg.sel, 0, len(list)-1)]
	if a.sg.dismissed == nil {
		a.sg.dismissed = map[string]bool{}
	}
	a.sg.dismissed[s.key] = true
	if s.kind == sgApplyFix {
		a.sg.fixOffer = false
	}
	a.sg.sel = clamp(a.sg.sel, 0, max(0, len(list)-2))
}

// ---- running a suggestion ----

func (a *App) runSuggestion() tea.Cmd {
	list := a.suggestions()
	if len(list) == 0 {
		return nil
	}
	s := list[clamp(a.sg.sel, 0, len(list)-1)]
	switch s.kind {
	case sgReviewMode:
		return a.openDiff(basePR)
	case sgConflict:
		files := a.sg.repo.conflicts
		return a.runAgent(s.kind, s.key, s.title, func() string { return conflictPrompt(files) })
	case sgCreatePR:
		branch, base := a.sg.repo.branch, a.sg.repo.base
		return a.runAgent(s.kind, s.key, s.title, func() string { return createPRPrompt(branch, base) })
	case sgLSPErrors:
		text := a.errorsText()
		return a.runAgent(s.kind, s.key, s.title, func() string { return lspPrompt(text) })
	case sgFailure:
		for _, f := range a.sg.fails {
			if fmt.Sprintf("fail:%d", f.seq) == s.key {
				cmd := a.runAgent(s.kind, s.key, s.title, func() string { return failurePrompt(f) })
				if a.ag.run != nil {
					a.clearFailure(f.tab)
				}
				return cmd
			}
		}
	case sgApplyFix:
		cmd := a.runAgent(s.kind, s.key, s.title, func() string { return applyFixPrompt() })
		if a.ag.run != nil {
			a.sg.fixOffer = false
		}
		return cmd
	case sgAIReview:
		pr, root, rev := a.gd.pr, a.root, a.gd.rev
		return a.runAgent(s.kind, s.key, s.title, func() string {
			out, err := run(root, nil, "git", "diff", rev, "--no-color", "--no-ext-diff", "-M")
			if err != nil {
				out = []byte(i18n.T("sg.diffunavailable1") + err.Error() + i18n.T("paren.close"))
			}
			return reviewPrompt(pr, string(out))
		})
	}
	return nil
}

// errorsText lists the LSP errors for the prompt.
func (a *App) errorsText() string {
	const maxErrors = 50
	paths, byPath := a.lspErrors()
	var sb strings.Builder
	n := 0
	for _, p := range paths {
		for _, d := range byPath[p] {
			if n == maxErrors {
				sb.WriteString(i18n.T("sg.more"))
				return sb.String()
			}
			n++
			src := ""
			if d.Source != "" {
				src = " (" + d.Source + ")"
			}
			fmt.Fprintf(&sb, "- %s:%d:%d: %s%s\n", a.rel(p), d.Range.Start.Line+1, d.Range.Start.Character+1, firstLine(d.Message), src)
		}
	}
	return sb.String()
}

// ---- prompts ----

func conflictPrompt(files []string) string {
	var sb strings.Builder
	sb.WriteString(i18n.T("sg.prompt.conflict"))
	for _, f := range files {
		fmt.Fprintf(&sb, i18n.T("sg.prompt.conflictfile"), f)
	}
	return sb.String()
}

func createPRPrompt(branch, base string) string {
	return fmt.Sprintf(i18n.T("sg.prompt.createpr"), branch, base)
}

func lspPrompt(errors string) string {
	return i18n.T("sg.prompt.lsp") + errors
}

func failurePrompt(f termFailure) string {
	what := fmt.Sprintf(i18n.T("sg.exitcode"), f.code)
	if f.code < 0 {
		what = i18n.T("sg.testfail") + f.line
	}
	return fmt.Sprintf(i18n.T("sg.prompt.fail"), what, strings.Join(f.output, "\n"))
}

func applyFixPrompt() string { return i18n.T("sg.prompt.applyfix") }

// maxReviewDiff bounds the diff sent for a review.
const maxReviewDiff = 200_000

func reviewPrompt(pr *prInfo, d string) string {
	if len(d) > maxReviewDiff {
		d = d[:maxReviewDiff] + i18n.T("sg.difftruncated")
	}
	return fmt.Sprintf(i18n.T("sg.prompt.review1")+"```json"+i18n.T("sg.prompt.review2")+"```diff\n%s\n```\n", pr.Number, pr.Title, d)
}

// ---- AI review results ----

type reviewComment struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	StartLine int    `json:"start_line"`
	Body      string `json:"body"`
}

var jsonBlock = regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```")

// parseReview reads the comments from the agent's reply: the last json code
// block, or the whole reply when it is a bare array.
func parseReview(text string) ([]reviewComment, error) {
	src := strings.TrimSpace(text)
	if m := jsonBlock.FindAllStringSubmatch(text, -1); len(m) > 0 {
		src = m[len(m)-1][1]
	}
	var out []reviewComment
	if err := json.Unmarshal([]byte(src), &out); err != nil {
		return nil, fmt.Errorf(i18n.T("sg.badjson"), err)
	}
	return out, nil
}

// placeReview turns the agent's review into unsent comments of the pull
// request. Comments GitHub would refuse are moved or dropped: a line outside
// the diff becomes a comment on the file, and files that are not in the
// pull request or differ from its head commit are skipped.
func (a *App) placeReview(run *agentRun) tea.Cmd {
	s := a.ag
	if !a.prShown() || a.gd.pr.Number != run.pr {
		s.add(logEntry{kind: logError, text: i18n.T("sg.prnotshown")})
		return nil
	}
	cs, err := parseReview(run.text.String())
	if err != nil {
		s.add(logEntry{kind: logError, text: err.Error()})
		a.fail(err)
		return nil
	}
	byPath := map[string]*diff.File{}
	for _, f := range a.gd.files {
		byPath[f.Path()] = f
	}
	placed, skipped := 0, 0
	for _, c := range cs {
		p := strings.TrimPrefix(filepath.ToSlash(c.Path), "./")
		if filepath.IsAbs(c.Path) {
			p = a.repoPath(c.Path)
		}
		f := byPath[p]
		if f == nil || a.isStale(p) || strings.TrimSpace(c.Body) == "" {
			skipped++
			continue
		}
		th := &prThread{Path: p, Side: "RIGHT", Pending: true}
		body := c.Body
		switch {
		case c.Line <= 0:
			th.File = true
		case !inDiff(f, c.Line):
			th.File = true
			body = fmt.Sprintf(i18n.T("sg.linebold"), c.Line, body)
		default:
			th.Line = c.Line
			if c.StartLine > 0 && c.StartLine < c.Line && inDiff(f, c.StartLine) {
				th.StartLine = c.StartLine
			}
		}
		th.Comments = []prComment{{Author: "AI", Body: body, Created: time.Now()}}
		a.gd.rv.pending = append(a.gd.rv.pending, th)
		placed++
	}
	msg := fmt.Sprintf(i18n.T("sg.placed"), placed)
	if skipped > 0 {
		msg += fmt.Sprintf(i18n.T("sg.skippedfiles"), skipped)
	}
	s.add(logEntry{kind: logInfo, text: msg})
	a.info("%s", msg)
	return nil
}

// inDiff reports whether line (1-based, new side) is in one of f's hunks,
// where GitHub accepts comments.
func inDiff(f *diff.File, line int) bool {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if l.NewNo == line {
				return true
			}
		}
	}
	return false
}
