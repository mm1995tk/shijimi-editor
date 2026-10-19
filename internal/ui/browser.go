package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const browserUsage = "使い方: :browser forward <対象ポート> [待ち受けポート] / :browser list / :browser stop <待ち受けポート>"

// browser runs `:browser <sub> ...`.
func (a *App) browser(arg string) tea.Cmd {
	sub, rest, _ := strings.Cut(arg, " ")
	args := strings.Fields(rest)
	switch sub {
	case "forward", "fwd":
		if len(args) < 1 || len(args) > 2 {
			a.fail(fmt.Errorf("%s", browserUsage))
			return nil
		}
		ports, err := parsePorts(args)
		if err != nil {
			a.fail(err)
			return nil
		}
		listen := 0 // free port chosen by the OS
		if len(ports) == 2 {
			listen = ports[1]
		}
		f, err := a.proxy.Start(a.cfg.Browser.ListenHost, ports[0], listen)
		if err != nil {
			a.fail(err)
			return nil
		}
		a.info("転送を開始しました: %s → localhost:%d", f.Listen, f.Target)
	case "list", "ls", "":
		fs := a.proxy.List()
		if len(fs) == 0 {
			a.info("ポート転送はありません（:browser forward <対象ポート> で追加）")
			return nil
		}
		items := make([]string, len(fs))
		for i, f := range fs {
			items[i] = fmt.Sprintf("%s → localhost:%d", f.Listen, f.Target)
		}
		a.info("ポート転送: %s", strings.Join(items, " / "))
	case "stop", "close":
		if len(args) != 1 {
			a.fail(fmt.Errorf("%s", browserUsage))
			return nil
		}
		ports, err := parsePorts(args)
		if err != nil {
			a.fail(err)
			return nil
		}
		if err := a.proxy.Stop(ports[0]); err != nil {
			a.fail(err)
			return nil
		}
		a.info("転送を停止しました: 待ち受けポート %d", ports[0])
	default:
		a.fail(fmt.Errorf("%s", browserUsage))
	}
	return nil
}

func parsePorts(args []string) ([]int, error) {
	out := make([]int, len(args))
	for i, s := range args {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("ポート番号が正しくありません: %s", s)
		}
		out[i] = n
	}
	return out, nil
}
