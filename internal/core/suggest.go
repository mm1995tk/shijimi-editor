package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/diff"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// Suggestions in the right panel. They come from rules over git, LSP and
// the terminal panel, never from the AI: that keeps them instant and free.
// Choosing one hands a prompt to the agent (agent.go).

type suggestKind int

const (
	SgApplyFix   suggestKind = iota // apply the fix the failure analysis proposed
	sgConflict                      // conflict markers are left in files
	sgCreatePR                      // the branch has commits and no pull request
	sgReviewMode                    // the branch has a pull request
	SgFailure                       // a command in the terminal panel failed
	SgLSPErrors                     // LSP reports errors
	SgAIReview                      // a pull request is shown in diff mode
)

// offer is one row of the panel. key names the condition: dismissing
// hides the suggestion until the condition goes away.
type offer struct {
	Kind   suggestKind
	key    string
	title  string
	Detail string
}

// suggestState is what the rules look at, gathered in the background.
type suggestState struct {
	Sel       int
	dismissed map[string]bool
	repo      repoProbe
	probing   bool
	ticks     int // watch ticks since the last probe
	prTicks   int // watch ticks since pull requests were last asked for
	Fails     []termFailure
	failSeq   int
	FixOffer  bool // the last failure analysis ended with a fix to apply
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
	n      int // number shown for the terminal
	Code   int // exit code, or -1 when a test reported FAIL
	line   string
	Output []string // the end of the terminal at the time
}

const (
	probeTicks   = 5  // watch ticks between repository probes
	prProbeTicks = 60 // watch ticks between pull request lookups
	FailTail     = 200
)

// failLine matches the summary lines test runners print on failure.
var failLine = regexp.MustCompile(`^\s*(--- FAIL|FAIL\b|FAILED\b|test result: FAILED)|^=+ .*\b\d+ failed\b`)

// ---- gathering ----

// ProbeRepo looks at the repository in the background. Pull requests are
// asked for only when the branch or HEAD moved, or every prProbeTicks.
func (a *Core) ProbeRepo() []Effect {
	if a.GitTop == "" || a.Sg.probing {
		return nil
	}
	a.Sg.probing = true
	a.Sg.ticks = 0
	root, prev := a.Root, a.Sg.repo
	askPR := a.Sg.prTicks >= prProbeTicks || prev.head == ""
	if askPR {
		a.Sg.prTicks = 0
	}
	return []Effect{RunProbe{Run: func() Event { return RepoProbed{probeRepoState(root, prev, askPR)} }}}
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

func (a *Core) handleRepoProbed(m RepoProbed) {
	a.Sg.probing = false
	a.Sg.repo = m.probe
	a.pruneDismissed()
}

// suggestTick runs on every watch tick: it reads the terminal panel and
// probes the repository now and then.
func (a *Core) suggestTick() []Effect {
	a.ScanTerminals()
	a.pruneDismissed()
	a.Sg.ticks++
	a.Sg.prTicks++
	if a.Sg.ticks >= probeTicks {
		return a.ProbeRepo()
	}
	return nil
}

// ScanTerminals records failed commands: exit codes the shell reports
// (OSC 133;D, see ShellIntegration) and FAIL lines of test runners.
func (a *Core) ScanTerminals() {
	alive := map[int]bool{}
	if t := a.TermShell(); t != nil {
		alive[t.ID()] = true
		for _, code := range t.TakeCommandEnds() {
			switch code {
			case 0:
				a.ClearFailure(t.ID())
			case 130:
				// Ctrl+C: the user stopped it on purpose.
			default:
				a.addFailure(t, code, "")
			}
		}
		for _, l := range t.NewLines() {
			if failLine.MatchString(l) {
				a.addFailure(t, -1, strings.TrimSpace(l))
				break
			}
		}
	}
	a.Sg.Fails = slices.DeleteFunc(a.Sg.Fails, func(f termFailure) bool { return f.tab != MuxFailTab && !alive[f.tab] })
}

// addFailure records a failure for the shell t. A shell still failing keeps its
// record (and the key a dismissal refers to); only what is shown changes.
func (a *Core) addFailure(t Shell, code int, line string) {
	a.AddFailureTab(t.ID(), 1, code, line, t.Tail(FailTail))
}

// AddFailureTab is addFailure for a terminal that is not a Shell.
func (a *Core) AddFailureTab(tab, n, code int, line string, output []string) {
	f := termFailure{tab: tab, n: n, Code: code, line: line, Output: output}
	for i := range a.Sg.Fails {
		if a.Sg.Fails[i].tab == tab {
			f.seq = a.Sg.Fails[i].seq
			a.Sg.Fails[i] = f
			return
		}
	}
	a.Sg.failSeq++
	f.seq = a.Sg.failSeq
	a.Sg.Fails = append(a.Sg.Fails, f)
}

// ClearFailure forgets the failures seen in terminal tab.
func (a *Core) ClearFailure(tab int) {
	a.Sg.Fails = slices.DeleteFunc(a.Sg.Fails, func(f termFailure) bool { return f.tab == tab })
}

// lspErrors returns the error diagnostics of every file, sorted by path.
func (a *Core) lspErrors() (paths []string, byPath map[string][]lsp.Diagnostic) {
	byPath = map[string][]lsp.Diagnostic{}
	for p := range a.Lsp.Diags {
		for _, d := range a.Lsp.diagnostics(p) {
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
func (a *Core) allSuggestions() []offer {
	var out []offer
	r := a.Sg.repo
	if a.Sg.FixOffer {
		out = append(out, offer{SgApplyFix, fmt.Sprintf("apply-fix:%d", a.Sg.fixSeq), i18n.T("sg.applyfix"), i18n.T("sg.fixdetail")})
	}
	if n := len(r.conflicts); n > 0 {
		out = append(out, offer{sgConflict, "conflict", i18n.T("sg.conflicts"), fmt.Sprintf(i18n.T("sg.files"), n, strings.Join(r.conflicts, ", "))})
	}
	if r.prKnown && len(r.prs) == 0 && r.ahead > 0 && r.base != "" && !isBranchOf(r.base, r.branch) {
		out = append(out, offer{sgCreatePR, "create-pr:" + r.branch, i18n.T("sg.createpr"), fmt.Sprintf(i18n.T("sg.commits"), r.branch, r.ahead)})
	}
	if r.prKnown && len(r.prs) > 0 && !a.PrShown() {
		p := r.prs[0]
		out = append(out, offer{sgReviewMode, fmt.Sprintf("review-mode:%s:%d", r.branch, p.Number), i18n.T("sg.reviewmode"), fmt.Sprintf("#%d %s", p.Number, p.Title)})
	}
	for _, f := range a.Sg.Fails {
		d := fmt.Sprintf(i18n.T("sg.termexit"), f.n, f.Code)
		if f.Code < 0 {
			d = fmt.Sprintf(i18n.T("sg.term"), f.n, f.line)
		}
		out = append(out, offer{SgFailure, fmt.Sprintf("fail:%d", f.seq), i18n.T("sg.investigate"), d})
	}
	if paths, byPath := a.lspErrors(); len(paths) > 0 {
		n := 0
		for _, p := range paths {
			n += len(byPath[p])
		}
		out = append(out, offer{SgLSPErrors, "lsp-errors", fmt.Sprintf(i18n.T("sg.fixerrors"), n), fmt.Sprintf(i18n.T("sg.files"), len(paths), a.Rel(paths[0]))})
	}
	if a.PrShown() {
		out = append(out, offer{SgAIReview, fmt.Sprintf("ai-review:%d", a.Gd.Pr.Number), i18n.T("sg.aireview"), fmt.Sprintf("#%d %s", a.Gd.Pr.Number, a.Gd.Pr.Title)})
	}
	return out
}

// isBranchOf reports whether revision base (e.g. origin/main) names branch.
func isBranchOf(base, branch string) bool {
	return base == branch || strings.HasSuffix(base, "/"+branch)
}

// Suggestions returns the suggestions shown: the rules minus dismissed ones.
func (a *Core) Suggestions() []offer {
	all := a.allSuggestions()
	out := all[:0]
	for _, s := range all {
		if !a.Sg.dismissed[s.key] {
			out = append(out, s)
		}
	}
	return out
}

// pruneDismissed forgets dismissals whose condition has gone, so the
// suggestion comes back when the condition does.
func (a *Core) pruneDismissed() {
	if len(a.Sg.dismissed) == 0 {
		return
	}
	live := map[string]bool{}
	for _, s := range a.allSuggestions() {
		live[s.key] = true
	}
	for k := range a.Sg.dismissed {
		if !live[k] {
			delete(a.Sg.dismissed, k)
		}
	}
}

func (a *Core) dismissSuggestion() {
	list := a.Suggestions()
	if len(list) == 0 {
		return
	}
	s := list[Clamp(a.Sg.Sel, 0, len(list)-1)]
	if a.Sg.dismissed == nil {
		a.Sg.dismissed = map[string]bool{}
	}
	a.Sg.dismissed[s.key] = true
	if s.Kind == SgApplyFix {
		a.Sg.FixOffer = false
	}
	a.Sg.Sel = Clamp(a.Sg.Sel, 0, max(0, len(list)-2))
}

// ---- running a suggestion ----

func (a *Core) runSuggestion() []Effect {
	list := a.Suggestions()
	if len(list) == 0 {
		return nil
	}
	s := list[Clamp(a.Sg.Sel, 0, len(list)-1)]
	switch s.Kind {
	case sgReviewMode:
		return a.OpenDiff(BasePR)
	case sgConflict:
		files := a.Sg.repo.conflicts
		return a.RunAgent(s.Kind, s.key, s.title, func() string { return conflictPrompt(files) })
	case sgCreatePR:
		branch, base := a.Sg.repo.branch, a.Sg.repo.base
		return a.RunAgent(s.Kind, s.key, s.title, func() string { return createPRPrompt(branch, base) })
	case SgLSPErrors:
		text := a.errorsText()
		return a.RunAgent(s.Kind, s.key, s.title, func() string { return lspPrompt(text) })
	case SgFailure:
		for _, f := range a.Sg.Fails {
			if fmt.Sprintf("fail:%d", f.seq) == s.key {
				cmd := a.RunAgent(s.Kind, s.key, s.title, func() string { return FailurePrompt(f) })
				if a.Ag.Run != nil {
					a.ClearFailure(f.tab)
				}
				return cmd
			}
		}
	case SgApplyFix:
		cmd := a.RunAgent(s.Kind, s.key, s.title, func() string { return applyFixPrompt() })
		if a.Ag.Run != nil {
			a.Sg.FixOffer = false
		}
		return cmd
	case SgAIReview:
		pr, root, rev := a.Gd.Pr, a.Root, a.Gd.rev
		return a.RunAgent(s.Kind, s.key, s.title, func() string {
			out, err := run(root, nil, "git", "diff", rev, "--no-color", "--no-ext-diff", "-M")
			if err != nil {
				out = []byte(fmt.Sprintf(i18n.T("sg.diffunavailable"), err.Error()))
			}
			return reviewPrompt(pr, string(out))
		})
	}
	return nil
}

// errorsText lists the LSP errors for the prompt.
func (a *Core) errorsText() string {
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
			fmt.Fprintf(&sb, "- %s:%d:%d: %s%s\n", a.Rel(p), d.Range.Start.Line+1, d.Range.Start.Character+1, FirstLine(d.Message), src)
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

// FailurePrompt is the prompt that asks the agent about f.
func FailurePrompt(f termFailure) string {
	what := fmt.Sprintf(i18n.T("sg.exitcode"), f.Code)
	if f.Code < 0 {
		what = i18n.T("sg.testfail") + f.line
	}
	return fmt.Sprintf(i18n.T("sg.prompt.fail"), what, strings.Join(f.Output, "\n"))
}

func applyFixPrompt() string { return i18n.T("sg.prompt.applyfix") }

// maxReviewDiff bounds the diff sent for a review.
const maxReviewDiff = 200_000

func reviewPrompt(pr *PrInfo, d string) string {
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
func (a *Core) placeReview(run *agentRun) []Effect {
	s := a.Ag
	if !a.PrShown() || a.Gd.Pr.Number != run.pr {
		s.add(LogEntry{Kind: logError, Text: i18n.T("sg.prnotshown")})
		return nil
	}
	cs, err := parseReview(run.text.String())
	if err != nil {
		s.add(LogEntry{Kind: logError, Text: err.Error()})
		a.Fail(err)
		return nil
	}
	byPath := map[string]*diff.File{}
	for _, f := range a.Gd.Files {
		byPath[f.Path()] = f
	}
	placed, skipped := 0, 0
	for _, c := range cs {
		p := strings.TrimPrefix(filepath.ToSlash(c.Path), "./")
		if filepath.IsAbs(c.Path) {
			p = a.RepoPath(c.Path)
		}
		f := byPath[p]
		if f == nil || a.IsStale(p) || strings.TrimSpace(c.Body) == "" {
			skipped++
			continue
		}
		th := &PrThread{Path: p, Side: "RIGHT", Pending: true}
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
		th.Comments = []PrComment{{Author: "AI", Body: body, Created: time.Now()}}
		a.Gd.Rv.Pending = append(a.Gd.Rv.Pending, th)
		placed++
	}
	msg := fmt.Sprintf(i18n.T("sg.placed"), placed)
	if skipped > 0 {
		msg += fmt.Sprintf(i18n.T("sg.skippedfiles"), skipped)
	}
	s.add(LogEntry{Kind: LogInfo, Text: msg})
	a.Info("%s", msg)
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
