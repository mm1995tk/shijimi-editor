package core

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/msg"
)

func browserUsage() msg.Msg { return msg.T("browser.usage") }

// browser runs `:browser <sub> ...`.
func (a *Core) browser(arg string) []Effect {
	sub, rest, _ := strings.Cut(arg, " ")
	args := strings.Fields(rest)
	switch sub {
	case "forward", "fwd":
		if len(args) < 1 || len(args) > 2 {
			a.Fail(msg.ErrOf(browserUsage()))
			return nil
		}
		ports, err := parsePorts(args)
		if err != nil {
			a.Fail(err)
			return nil
		}
		listen := 0 // free port chosen by the OS
		if len(ports) == 2 {
			listen = ports[1]
		}
		f, err := a.shared.Proxy.Start(a.Cfg.Browser.ListenHost, ports[0], listen)
		if err != nil {
			a.Fail(err)
			return nil
		}
		a.Info("browser.started", f.Listen, f.Target)
	case "list", "ls", "":
		fs := a.shared.Proxy.List()
		if len(fs) == 0 {
			a.InfoText(msg.T("browser.none"))
			return nil
		}
		items := make([]string, len(fs))
		for i, f := range fs {
			items[i] = fmt.Sprintf("%s → localhost:%d", f.Listen, f.Target)
		}
		a.Info("browser.list", strings.Join(items, " / "))
	case "stop", "close":
		if len(args) != 1 {
			a.Fail(msg.ErrOf(browserUsage()))
			return nil
		}
		ports, err := parsePorts(args)
		if err != nil {
			a.Fail(err)
			return nil
		}
		if err := a.shared.Proxy.Stop(ports[0]); err != nil {
			a.Fail(err)
			return nil
		}
		a.Info("browser.stopped", ports[0])
	default:
		a.Fail(msg.ErrOf(browserUsage()))
	}
	return nil
}

func parsePorts(args []string) ([]int, error) {
	out := make([]int, len(args))
	for i, s := range args {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 65535 {
			return nil, msg.Err("browser.badport", s)
		}
		out[i] = n
	}
	return out, nil
}
