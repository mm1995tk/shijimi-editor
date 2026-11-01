package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/diff"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// External commands (git, gh) and the GitHub pull request data they return.

// prInfo is a pull request as `gh pr view --json prFields` reports it.
type prInfo struct {
	ID          string `json:"id"`
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

const prFields = "id,number,title,url,headRefOid,headRefName,baseRefName,state,isDraft,author"

type prSummary struct {
	ID          string `json:"id"`
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

func listPRs(root string) []Effect {
	return async(func() Event {
		out, err := run(root, nil, "gh", "pr", "list", "--limit", "100", "--json", "number,title,headRefName,isDraft,author")
		if err != nil {
			return prListMsg{err: err}
		}
		var prs []prSummary
		err = json.Unmarshal(out, &prs)
		return prListMsg{prs: prs, err: err}
	})
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

// branchPRs returns the open pull requests whose head is branch.
func branchPRs(root, branch string) ([]prSummary, error) {
	out, err := run(root, nil, "gh", "pr", "list", "--head", branch, "--json", "number,title,headRefName,isDraft,author")
	if err != nil {
		return nil, err
	}
	var prs []prSummary
	err = json.Unmarshal(out, &prs)
	return prs, err
}

// prSync is how the local HEAD stands against the pull request's head commit.
type prSync struct {
	known  bool // the head commit is available locally
	ahead  int  // local commits not in the pull request
	behind int  // pull request commits not in HEAD
}

// text is the status line note for the state, or "" when they agree.
func (s prSync) text() string {
	switch {
	case !s.known:
		return ""
	case s.ahead > 0 && s.behind > 0:
		return i18n.T("gh.diverged")
	case s.behind > 0:
		return i18n.T("gh.prupdated")
	case s.ahead > 0:
		return fmt.Sprintf(i18n.T("gh.unpushed"), s.ahead)
	}
	return ""
}

// fetchPR brings the pull request's head commit and its base branch into the
// local repository, so they can be compared with HEAD. Failures are ignored:
// the comparison then reports what it can.
func fetchPR(root string, pr *prInfo) {
	run(root, nil, "git", "fetch", "--quiet", "origin", fmt.Sprintf("refs/pull/%d/head", pr.Number), pr.BaseRefName)
}

// syncWith compares HEAD with the pull request's head commit oid.
func syncWith(root, oid string) prSync {
	if _, err := run(root, nil, "git", "cat-file", "-e", oid+"^{commit}"); err != nil {
		return prSync{}
	}
	out, err := run(root, nil, "git", "rev-list", "--left-right", "--count", "HEAD..."+oid)
	if err != nil {
		return prSync{}
	}
	var s prSync
	if _, err := fmt.Sscan(string(out), &s.ahead, &s.behind); err != nil {
		return prSync{}
	}
	s.known = true
	return s
}

// ghRun runs gh for gql; tests replace it.
var ghRun = run

// gql runs a GraphQL query through gh and decodes the "data" member into out.
func gql(root, query string, vars map[string]any, out any) error {
	req, _ := json.Marshal(map[string]any{"query": query, "variables": vars})
	data, err := ghRun(root, req, "gh", "api", "graphql", "--input", "-")
	if err != nil {
		return err
	}
	var resp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		return fmt.Errorf("GitHub: %s", resp.Errors[0].Message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(resp.Data, out)
}

// prComment is one comment of a review thread.
type prComment struct {
	Author  string
	Body    string
	Created time.Time
}

// prThread is a review thread: comments on a line, a range of lines, or a
// whole file. Line numbers are 1-based and count in the pull request's head
// commit (Side RIGHT) or its base (LEFT).
type prThread struct {
	ID        string
	Path      string
	Line      int // last line; 0 for a file comment or an outdated thread
	StartLine int // first line of a range, 0 for a single line
	Side      string
	File      bool // the subject is the whole file
	Resolved  bool
	Outdated  bool
	Pending   bool // written here and not sent yet
	Comments  []prComment
}

// first returns the first line of the thread's range.
func (t *prThread) first() int {
	if t.StartLine > 0 {
		return t.StartLine
	}
	return t.Line
}

// onLine reports whether the thread is shown at line (1-based) of the
// current side of a file.
func (t *prThread) onLine(line int) bool {
	return !t.File && !t.Outdated && t.Side == "RIGHT" && t.Line == line
}

// prReviewData is what GitHub knows about the review of a pull request.
type prReviewData struct {
	threads []*prThread
	viewed  map[string]bool
}

const threadsQuery = `query($id: ID!, $after: String) {
  node(id: $id) {
    ... on PullRequest {
      reviewThreads(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id path line startLine diffSide isResolved isOutdated subjectType
          comments(first: 100) { nodes { body createdAt author { login } } }
        }
      }
    }
  }
}`

const filesQuery = `query($id: ID!, $after: String) {
  node(id: $id) {
    ... on PullRequest {
      files(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes { path viewerViewedState }
      }
    }
  }
}`

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

// pageVars returns the variables for the page after cursor ("" = the first).
func pageVars(id, cursor string) map[string]any {
	v := map[string]any{"id": id}
	if cursor != "" {
		v["after"] = cursor
	}
	return v
}

// loadReview reads the review threads and viewed files of pull request pr,
// every page of them.
func loadReview(root string, pr *prInfo) (prReviewData, error) {
	d := prReviewData{viewed: map[string]bool{}}
	for cursor := ""; ; {
		var resp struct {
			Node struct {
				ReviewThreads struct {
					PageInfo pageInfo `json:"pageInfo"`
					Nodes    []struct {
						ID          string `json:"id"`
						Path        string `json:"path"`
						Line        *int   `json:"line"`
						StartLine   *int   `json:"startLine"`
						DiffSide    string `json:"diffSide"`
						IsResolved  bool   `json:"isResolved"`
						IsOutdated  bool   `json:"isOutdated"`
						SubjectType string `json:"subjectType"`
						Comments    struct {
							Nodes []struct {
								Body      string    `json:"body"`
								CreatedAt time.Time `json:"createdAt"`
								Author    *struct {
									Login string `json:"login"`
								} `json:"author"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"node"`
		}
		if err := gql(root, threadsQuery, pageVars(pr.ID, cursor), &resp); err != nil {
			return d, err
		}
		rt := resp.Node.ReviewThreads
		for _, n := range rt.Nodes {
			t := &prThread{ID: n.ID, Path: n.Path, Side: n.DiffSide, Resolved: n.IsResolved, Outdated: n.IsOutdated, File: n.SubjectType == "FILE"}
			if n.Line != nil {
				t.Line = *n.Line
			} else if !t.File {
				t.Outdated = true
			}
			if n.StartLine != nil && *n.StartLine != t.Line {
				t.StartLine = *n.StartLine
			}
			for _, c := range n.Comments.Nodes {
				who := "ghost"
				if c.Author != nil {
					who = c.Author.Login
				}
				t.Comments = append(t.Comments, prComment{Author: who, Body: c.Body, Created: c.CreatedAt})
			}
			if len(t.Comments) > 0 {
				d.threads = append(d.threads, t)
			}
		}
		if !rt.PageInfo.HasNextPage {
			break
		}
		cursor = rt.PageInfo.EndCursor
	}
	for cursor := ""; ; {
		var resp struct {
			Node struct {
				Files struct {
					PageInfo pageInfo `json:"pageInfo"`
					Nodes    []struct {
						Path   string `json:"path"`
						Viewed string `json:"viewerViewedState"`
					} `json:"nodes"`
				} `json:"files"`
			} `json:"node"`
		}
		if err := gql(root, filesQuery, pageVars(pr.ID, cursor), &resp); err != nil {
			return d, err
		}
		fs := resp.Node.Files
		for _, f := range fs.Nodes {
			if f.Viewed == "VIEWED" {
				d.viewed[f.Path] = true
			}
		}
		if !fs.PageInfo.HasNextPage {
			break
		}
		cursor = fs.PageInfo.EndCursor
	}
	return d, nil
}

// replyThread adds a reply to review thread id.
func replyThread(root, id, body string) error {
	return gql(root, `mutation($id: ID!, $body: String!) {
  addPullRequestReviewThreadReply(input: {pullRequestReviewThreadId: $id, body: $body}) { comment { id } }
}`, map[string]any{"id": id, "body": body}, nil)
}

// resolveThread resolves review thread id, or reopens it when resolve is false.
func resolveThread(root, id string, resolve bool) error {
	m := "resolveReviewThread"
	if !resolve {
		m = "unresolveReviewThread"
	}
	return gql(root, `mutation($id: ID!) { `+m+`(input: {threadId: $id}) { thread { id } } }`, map[string]any{"id": id}, nil)
}

// setViewed marks path of pull request pr as viewed, or not viewed.
func setViewed(root string, pr *prInfo, path string, viewed bool) error {
	m := "markFileAsViewed"
	if !viewed {
		m = "unmarkFileAsViewed"
	}
	return gql(root, `mutation($id: ID!, $path: String!) { `+m+`(input: {pullRequestId: $id, path: $path}) { clientMutationId } }`,
		map[string]any{"id": pr.ID, "path": path}, nil)
}

// submitReview sends a review of pr with the pending threads. event is
// APPROVE, REQUEST_CHANGES or COMMENT. The review is created as a draft,
// filled, then submitted; on any failure the draft is deleted, so nothing
// half-sent stays on GitHub.
func submitReview(root string, pr *prInfo, event, body string, threads []*prThread) error {
	// GitHub allows one draft review per person; one left over (from the web,
	// or a send that could not clean up) would make every send fail
	var drafts struct {
		Node struct {
			Reviews struct {
				Nodes []struct {
					ViewerDidAuthor bool `json:"viewerDidAuthor"`
				} `json:"nodes"`
			} `json:"reviews"`
		} `json:"node"`
	}
	if err := gql(root, `query($id: ID!) { node(id: $id) { ... on PullRequest { reviews(states: PENDING, first: 20) { nodes { viewerDidAuthor } } } } }`,
		map[string]any{"id": pr.ID}, &drafts); err != nil {
		return err
	}
	for _, r := range drafts.Node.Reviews.Nodes {
		if r.ViewerDidAuthor {
			return fmt.Errorf(i18n.T("gh.pendingdraft"), pr.URL)
		}
	}
	var created struct {
		AddPullRequestReview struct {
			PullRequestReview struct {
				ID string `json:"id"`
			} `json:"pullRequestReview"`
		} `json:"addPullRequestReview"`
	}
	err := gql(root, `mutation($pr: ID!, $oid: GitObjectID!) {
  addPullRequestReview(input: {pullRequestId: $pr, commitOID: $oid}) { pullRequestReview { id } }
}`, map[string]any{"pr": pr.ID, "oid": pr.HeadRefOid}, &created)
	if err != nil {
		return err
	}
	id := created.AddPullRequestReview.PullRequestReview.ID
	fail := func(err error) error {
		gql(root, `mutation($id: ID!) { deletePullRequestReview(input: {pullRequestReviewId: $id}) { clientMutationId } }`, map[string]any{"id": id}, nil)
		return err
	}
	for _, t := range threads {
		vars := map[string]any{"id": id, "path": t.Path, "body": t.Comments[0].Body}
		args := "pullRequestReviewId: $id, path: $path, body: $body"
		decl := "$id: ID!, $path: String!, $body: String!"
		if t.File {
			args += ", subjectType: FILE"
		} else {
			vars["line"] = t.Line
			args += ", line: $line, side: RIGHT"
			decl += ", $line: Int!"
			if t.StartLine > 0 {
				vars["start"] = t.StartLine
				args += ", startLine: $start, startSide: RIGHT"
				decl += ", $start: Int!"
			}
		}
		q := `mutation(` + decl + `) { addPullRequestReviewThread(input: {` + args + `}) { thread { id } } }`
		if err := gql(root, q, vars, nil); err != nil {
			return fail(fmt.Errorf("%s: %w", t.Path, err))
		}
	}
	vars := map[string]any{"id": id, "event": event}
	q := `mutation($id: ID!, $event: PullRequestReviewEvent!) { submitPullRequestReview(input: {pullRequestReviewId: $id, event: $event}) { pullRequestReview { id } } }`
	if body != "" {
		vars["body"] = body
		q = `mutation($id: ID!, $event: PullRequestReviewEvent!, $body: String) { submitPullRequestReview(input: {pullRequestReviewId: $id, event: $event, body: $body}) { pullRequestReview { id } } }`
	}
	if err := gql(root, q, vars, nil); err != nil {
		return fail(err)
	}
	return nil
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
	h := diff.Hunk{Header: fmt.Sprintf(i18n.T("gh.untracked"), len(lines))}
	for i, l := range lines {
		h.Lines = append(h.Lines, diff.Line{Kind: diff.Add, Text: l, NewNo: i + 1})
	}
	f.Hunks = []diff.Hunk{h}
	f.Added = len(lines)
	return f
}
