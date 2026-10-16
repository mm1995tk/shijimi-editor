// Package render turns a web page into a PNG. The backend sits behind the
// Renderer interface so headless Chromium can be swapped for something else;
// it is shared by the browser view and the mermaid preview.
package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Options controls one rendering. Zero values mean the defaults.
type Options struct {
	Width, Height int           // viewport in pixels (default 1280x800)
	Timeout       time.Duration // default 30s
}

// Renderer renders a URL to PNG bytes.
type Renderer interface {
	Render(ctx context.Context, url string, opts Options) ([]byte, error)
}

// ErrNotFound means no Chromium executable could be located.
var ErrNotFound = errors.New("Chromium が見つかりません。インストールするか、config.toml の [browser] chromium にパスを指定してください")

var pngMagic = []byte("\x89PNG\r\n\x1a\n")

// candidates are tried in order when no path is configured.
var candidates = []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome"}

// Chromium renders with a headless Chromium / Chrome executable.
type Chromium struct {
	Path string // executable; "" = search PATH
}

// Find returns the executable to use.
func (c Chromium) Find() (string, error) {
	if c.Path != "" {
		p, err := exec.LookPath(c.Path)
		if err != nil {
			return "", fmt.Errorf("%w（指定: %s）", ErrNotFound, c.Path)
		}
		return p, nil
	}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", ErrNotFound
}

// Render implements Renderer.
func (c Chromium) Render(ctx context.Context, url string, opts Options) ([]byte, error) {
	bin, err := c.Find()
	if err != nil {
		return nil, err
	}
	if opts.Width <= 0 {
		opts.Width = 1280
	}
	if opts.Height <= 0 {
		opts.Height = 800
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	dir, err := os.MkdirTemp("", "gh-editor-render-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "shot.png")

	cmd := exec.CommandContext(ctx, bin,
		"--headless", "--disable-gpu", "--no-sandbox", "--hide-scrollbars",
		"--user-data-dir="+filepath.Join(dir, "profile"),
		fmt.Sprintf("--window-size=%d,%d", opts.Width, opts.Height),
		"--screenshot="+out,
		url)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("描画がタイムアウトしました（%s）", opts.Timeout)
		}
		return nil, fmt.Errorf("Chromium の実行に失敗しました: %w: %s", err, strings.TrimSpace(lastLines(stderr.String(), 3)))
	}
	data, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("Chromium が画像を出力しませんでした: %w", err)
	}
	if !bytes.HasPrefix(data, pngMagic) {
		return nil, errors.New("Chromium の出力が PNG ではありません")
	}
	return data, nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
