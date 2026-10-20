package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/diff"
)

// External commands (git, gh) and the GitHub pull request data they return.

// prInfo is a pull request as `gh pr view --json prFields` reports it.
type prInfo struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	HeadRefOid  string `json:"headRefOid"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

const prFields = "number,title,url,headRefOid,headRefName,baseRefName,state,isDraft,author"

type prSummary struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	HeadRefName string `json:"headRefName"`
	IsDraft     bool   `json:"isDraft"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

type prListMsg struct {
	prs []prSummary
	err error
}

func run(dir string, stdin []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("%s: %s", name, firstLine(msg))
	}
	return out, nil
}

func listPRs(root string) tea.Cmd {
	return func() tea.Msg {
		out, err := run(root, nil, "gh", "pr", "list", "--limit", "100", "--json", "number,title,headRefName,isDraft,author")
		if err != nil {
			return prListMsg{err: err}
		}
		var prs []prSummary
		err = json.Unmarshal(out, &prs)
		return prListMsg{prs: prs, err: err}
	}
}

// viewPR returns pull request num, or the current branch's when num is 0.
func viewPR(root string, num int) (*prInfo, error) {
	args := []string{"pr", "view"}
	if num > 0 {
		args = append(args, fmt.Sprint(num))
	}
	out, err := run(root, nil, "gh", append(args, "--json", prFields)...)
	if err != nil {
		return nil, err
	}
	var pr prInfo
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// prComment is a review comment on a pull request.
type prComment struct {
	Path   string
	Line   int
	Side   string // "RIGHT" (new) or "LEFT" (old)
	Body   string
	Author string
}

// loadPRComments returns the review comments of pull request num that are
// still attached to a line.
func loadPRComments(root string, num int) ([]prComment, error) {
	out, err := run(root, nil, "gh", "api", "--paginate", fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/comments", num))
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Path string `json:"path"`
		Line *int   `json:"line"`
		Side string `json:"side"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	// --paginate concatenates JSON arrays; normalise "][" boundaries.
	data := bytes.ReplaceAll(out, []byte("]["), []byte(","))
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var comments []prComment
	for _, c := range raw {
		if c.Line == nil {
			continue // outdated comment
		}
		side := c.Side
		if side == "" {
			side = "RIGHT"
		}
		comments = append(comments, prComment{Path: c.Path, Line: *c.Line, Side: side, Body: c.Body, Author: c.User.Login})
	}
	return comments, nil
}

// submitReview posts a review of pull request pr with comments. event is
// APPROVE, REQUEST_CHANGES or COMMENT.
func submitReview(root string, pr *prInfo, event, body string, comments []prComment) error {
	type c struct {
		Path string `json:"path"`
		Line int    `json:"line"`
		Side string `json:"side"`
		Body string `json:"body"`
	}
	payload := struct {
		CommitID string `json:"commit_id"`
		Body     string `json:"body,omitempty"`
		Event    string `json:"event"`
		Comments []c    `json:"comments"`
	}{CommitID: pr.HeadRefOid, Body: body, Event: event, Comments: []c{}}
	for _, cm := range comments {
		payload.Comments = append(payload.Comments, c{cm.Path, cm.Line, cm.Side, cm.Body})
	}
	data, _ := json.Marshal(payload)
	_, err := run(root, data, "gh", "api", "-X", "POST", fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/reviews", pr.Number), "--input", "-")
	return err
}

// untrackedFile returns the diff of an untracked file: every line added.
func untrackedFile(abs, rel string) *diff.File {
	f := &diff.File{NewPath: rel, Status: "added"}
	st, err := os.Stat(abs)
	if err != nil || st.IsDir() {
		return nil
	}
	if st.Size() > 1<<20 {
		f.Status = "binary"
		return f
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	if bytes.IndexByte(data, 0) >= 0 {
		f.Status = "binary"
		return f
	}
	text := strings.TrimSuffix(string(data), "\n")
	lines := strings.Split(text, "\n")
	h := diff.Hunk{Header: fmt.Sprintf("@@ -0,0 +1,%d @@ (未追跡ファイル)", len(lines))}
	for i, l := range lines {
		h.Lines = append(h.Lines, diff.Line{Kind: diff.Add, Text: l, NewNo: i + 1})
	}
	f.Hunks = []diff.Hunk{h}
	f.Added = len(lines)
	return f
}
