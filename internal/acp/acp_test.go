package acp

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/acp/acptest"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

func TestMain(m *testing.M) {
	acptest.Main()
	// The assertions below read the Japanese strings.
	i18n.SetLang(i18n.Japanese)
	os.Exit(m.Run())
}

type recorder struct {
	mu      sync.Mutex
	text    strings.Builder
	kinds   []string
	files   map[string]string
	written map[string]string
	allow   bool
	asked   []PermissionRequest
}

func (r *recorder) ReadTextFile(_ context.Context, q ReadRequest) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.files[q.Path], nil
}

func (r *recorder) WriteTextFile(_ context.Context, q WriteRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.written[q.Path] = q.Content
	return nil
}

func (r *recorder) RequestPermission(_ context.Context, q PermissionRequest) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asked = append(r.asked, q)
	if r.allow {
		return "allow", nil
	}
	return "reject", nil
}

func (r *recorder) SessionUpdate(_ string, u Update) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kinds = append(r.kinds, u.Kind)
	r.text.WriteString(u.Text())
}

func start(t *testing.T, h Handler) *Client {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c, err := Start(Options{Command: exe, Env: map[string]string{acptest.Env: "1"}, Dir: t.TempDir()}, h)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPromptReadsAndWritesThroughTheHandler(t *testing.T) {
	r := &recorder{files: map[string]string{"/p/a.txt": "hello\n"}, written: map[string]string{}, allow: true}
	c := start(t, r)
	if got := c.Info().Name(); got != "Fake Agent" {
		t.Errorf("name = %q", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sid, err := c.NewSession(ctx, "/p")
	if err != nil || sid != "s1" {
		t.Fatalf("session = %q, %v", sid, err)
	}
	stop, err := c.Prompt(ctx, sid, Text("READ:/p/a.txt\nWRITE:/p/b.txt"))
	if err != nil || stop != "end_turn" {
		t.Fatalf("stop = %q, %v", stop, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if got := r.text.String(); !strings.Contains(got, "内容: hello") {
		t.Errorf("text = %q", got)
	}
	if r.written["/p/b.txt"] != "written by agent\n" {
		t.Errorf("written = %v", r.written)
	}
	if len(r.asked) != 1 || r.asked[0].ToolCall.Title != "Edit /p/b.txt" || len(r.asked[0].Options) != 2 {
		t.Errorf("asked = %+v", r.asked)
	}
	if !strings.Contains(strings.Join(r.kinds, ","), "tool_call,") {
		t.Errorf("kinds = %v", r.kinds)
	}
}

func TestRejectedPermissionSkipsTheWrite(t *testing.T) {
	r := &recorder{written: map[string]string{}}
	c := start(t, r)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sid, _ := c.NewSession(ctx, "/p")
	if _, err := c.Prompt(ctx, sid, Text("WRITE:/p/b.txt")); err != nil {
		t.Fatal(err)
	}
	if len(r.written) != 0 {
		t.Errorf("written = %v", r.written)
	}
}

func TestCancelEndsTheTurn(t *testing.T) {
	c := start(t, &recorder{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sid, _ := c.NewSession(ctx, "/p")
	done := make(chan string, 1)
	go func() {
		stop, _ := c.Prompt(ctx, sid, Text("WAIT"))
		done <- stop
	}()
	time.Sleep(50 * time.Millisecond)
	if err := c.Cancel(sid); err != nil {
		t.Fatal(err)
	}
	if stop := <-done; stop != "cancelled" {
		t.Errorf("stop = %q", stop)
	}
}

func TestCallFailsWhenTheAgentExits(t *testing.T) {
	c := start(t, &recorder{})
	c.cmd.Process.Kill()
	<-c.Done()
	if _, err := c.NewSession(context.Background(), "/p"); err == nil {
		t.Fatal("no error after the agent exited")
	}
}

func TestStartReportsAMissingCommand(t *testing.T) {
	if _, err := Start(Options{Command: "shijimi-no-such-agent"}, &recorder{}); err == nil {
		t.Fatal("no error")
	}
}

func TestTooLongLineEndsTheAgent(t *testing.T) {
	old := maxLine
	maxLine = 1024
	defer func() { maxLine = old }()
	c := start(t, &recorder{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sid, _ := c.NewSession(ctx, "/p")
	if _, err := c.Prompt(ctx, sid, Text("HUGE:200000\nWAIT")); err == nil || ctx.Err() != nil {
		t.Fatalf("err = %v, ctx = %v", err, ctx.Err())
	}
	<-c.Done()
	if c.Err() == nil || !strings.Contains(c.Err().Error(), "読めません") {
		t.Errorf("Err = %v", c.Err())
	}
}
