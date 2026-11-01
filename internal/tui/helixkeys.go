package tui

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// helixCtx is what a Helix command sees when it runs.
type helixCtx struct {
	b        *buffer.Buffer
	ext      bool // select mode: moves extend the selection
	count    int
	hadCount bool
}

func (c *helixCtx) repeat(f func()) {
	for i := 0; i < c.count; i++ {
		f()
	}
}

// helixCmd is one named Helix command. multi commands run once per cursor
// when there are several.
type helixCmd struct {
	multi bool
	run   func(a *Core, c *helixCtx) []Effect
}

// helixCommands maps a Helix command name ("move_char_left") to its action.
// Only the normal-mode movement and editing commands have names; other keys
// are still handled by helixNormalKey.
var helixCommands = map[string]helixCmd{}

// helixDefaultKeys is the built-in normal-mode keymap: key (as
// Key.String reports it) to command name.
var helixDefaultKeys = map[string]string{}

// helixCommandNames returns the command names, sorted.
func helixCommandNames() []string {
	names := make([]string, 0, len(helixCommands))
	for n := range helixCommands {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func init() {
	// def registers a command and the keys it has by default.
	def := func(name string, multi bool, run func(a *Core, c *helixCtx) []Effect, keys ...string) {
		helixCommands[name] = helixCmd{multi: multi, run: run}
		for _, k := range keys {
			helixDefaultKeys[k] = name
		}
	}
	// move registers a movement as move_* (extends in select mode) and
	// extend_* (always extends). names holds the Helix suffixes, e.g.
	// "char_left"; the first one gets the default keys.
	move := func(names []string, f func(a *Core, c *helixCtx, ext bool), keys ...string) {
		for i, n := range names {
			var k []string
			if i == 0 {
				k = keys
			}
			def("move_"+n, true, func(a *Core, c *helixCtx) []Effect { f(a, c, c.ext); return nil }, k...)
			def("extend_"+n, true, func(a *Core, c *helixCtx) []Effect { f(a, c, true); return nil })
		}
	}
	dir := func(d buffer.Move) func(a *Core, c *helixCtx, ext bool) {
		return func(a *Core, c *helixCtx, ext bool) { c.repeat(func() { c.b.MoveCursor(d, ext) }) }
	}
	move([]string{"char_left"}, dir(buffer.Left), "h", "left")
	move([]string{"char_right"}, dir(buffer.Right), "l", "right")
	move([]string{"visual_line_down", "line_down"}, dir(buffer.Down), "j", "down")
	move([]string{"visual_line_up", "line_up"}, dir(buffer.Up), "k", "up")
	word := func(forward bool) func(a *Core, c *helixCtx, ext bool) {
		return func(a *Core, c *helixCtx, ext bool) { c.repeat(func() { c.b.SelectWord(forward, ext) }) }
	}
	move([]string{"next_word_start"}, word(true), "w")
	move([]string{"next_long_word_start"}, word(true), "W")
	move([]string{"prev_word_start"}, word(false), "b")
	move([]string{"prev_long_word_start"}, word(false), "B")
	wordEnd := func(a *Core, c *helixCtx, ext bool) {
		if !ext {
			c.b.Anchor = nil
		}
		c.repeat(func() { c.b.WordEnd(true) })
	}
	move([]string{"next_word_end"}, wordEnd, "e")
	move([]string{"next_long_word_end"}, wordEnd, "E")
	def("goto_line_start", true, func(a *Core, c *helixCtx) []Effect { c.b.MoveCursor(buffer.Home, c.ext); return nil }, "home")
	def("goto_line_end", true, func(a *Core, c *helixCtx) []Effect { c.b.MoveCursor(buffer.End, c.ext); return nil }, "end")
	def("extend_to_line_start", true, func(a *Core, c *helixCtx) []Effect { c.b.MoveCursor(buffer.Home, true); return nil })
	def("extend_to_line_end", true, func(a *Core, c *helixCtx) []Effect { c.b.MoveCursor(buffer.End, true); return nil })

	def("page_up", false, func(a *Core, c *helixCtx) []Effect { c.b.MoveLines(-a.pageSize(), c.ext); return nil }, "pgup", "ctrl+b")
	def("page_down", false, func(a *Core, c *helixCtx) []Effect { c.b.MoveLines(a.pageSize(), c.ext); return nil }, "pgdown", "ctrl+f")
	def("jump_backward", false, func(a *Core, c *helixCtx) []Effect { return a.jumpMove(-1) }, "ctrl+o")
	// Ctrl+I arrives as Tab unless the terminal tells them apart.
	def("jump_forward", false, func(a *Core, c *helixCtx) []Effect { return a.jumpMove(1) }, "tab", "ctrl+i")
	def("goto_last_line", false, func(a *Core, c *helixCtx) []Effect {
		a.pushJump()
		if c.hadCount {
			c.b.GotoLine(c.count)
		} else {
			c.b.MoveCursor(buffer.DocEnd, c.ext)
		}
		return nil
	}, "G")

	// --- selection ---
	selectLine := func(a *Core, c *helixCtx) []Effect { c.repeat(c.b.SelectLine); return nil }
	def("extend_line_below", true, selectLine, "x")
	def("extend_to_line_bounds", true, selectLine, "X")
	def("select_all", false, func(a *Core, c *helixCtx) []Effect { c.b.SelectAll(); return nil }, "%")
	def("collapse_selection", true, func(a *Core, c *helixCtx) []Effect { c.b.ClearSelection(); return nil }, ";")
	def("flip_selections", true, func(a *Core, c *helixCtx) []Effect {
		if c.b.Anchor != nil {
			p, q := *c.b.Anchor, c.b.Cur
			c.b.Anchor, c.b.Cur = &q, p
		}
		return nil
	}, "alt+;")
	def("select_mode", false, func(a *Core, c *helixCtx) []Effect {
		if a.mode == modeSelect {
			a.mode = modeNormal
		} else {
			a.mode = modeSelect
		}
		return nil
	}, "v")
	def("normal_mode", true, func(a *Core, c *helixCtx) []Effect {
		a.mode = modeNormal
		a.comp = nil
		c.b.ClearSelection()
		return nil
	}, "esc")
	def("no_op", false, func(a *Core, c *helixCtx) []Effect { return nil })

	// --- editing ---
	deleteSel := func(yank bool) func(a *Core, c *helixCtx) []Effect {
		return func(a *Core, c *helixCtx) []Effect {
			s, e, widened := c.b.LineDeleteRange()
			text := c.b.DeleteRange(s, e)
			if widened {
				// the removed text starts with the preceding newline; keep the
				// register linewise so "p" pastes whole lines
				text = strings.TrimPrefix(text, "\n") + "\n"
			}
			if yank {
				a.clipboard = text
			}
			a.mode = modeNormal
			return nil
		}
	}
	def("delete_selection", true, deleteSel(true), "d")
	def("delete_selection_noyank", true, deleteSel(false), "alt+d")
	changeSel := func(yank bool) func(a *Core, c *helixCtx) []Effect {
		return func(a *Core, c *helixCtx) []Effect {
			b := c.b
			s, e := b.SelectionOrChar()
			if _, _, ok := b.Selection(); ok {
				text := b.DeleteRange(s, e)
				if yank {
					a.clipboard = text
				}
			} else {
				a.clipboard = b.DeleteRange(s, e)
			}
			return a.enterInsert()
		}
	}
	def("change_selection", true, changeSel(true), "c")
	def("change_selection_noyank", true, changeSel(false), "alt+c")
	def("yank", true, func(a *Core, c *helixCtx) []Effect {
		s, e := c.b.SelectionOrChar()
		a.clipboard = c.b.TextRange(s, e)
		a.info(i18n.T("helix.yanked"), len([]rune(a.clipboard)))
		return nil
	}, "y")
	def("paste_after", true, func(a *Core, c *helixCtx) []Effect { a.paste(c.b, a.clipboard, true); return nil }, "p")
	def("paste_before", true, func(a *Core, c *helixCtx) []Effect { a.paste(c.b, a.clipboard, false); return nil }, "P")
	def("replace_with_yanked", true, func(a *Core, c *helixCtx) []Effect {
		if s, e, ok := c.b.Selection(); ok {
			c.b.DeleteRange(s, e)
			c.b.InsertBlock(a.clipboard)
		}
		return nil
	}, "R")
	def("undo", false, func(a *Core, c *helixCtx) []Effect { c.repeat(func() { c.b.Undo() }); return nil }, "u")
	def("redo", false, func(a *Core, c *helixCtx) []Effect { c.repeat(func() { c.b.Redo() }); return nil }, "U")
	def("insert_mode", true, func(a *Core, c *helixCtx) []Effect {
		if s, _, ok := c.b.Selection(); ok {
			c.b.Cur = s
		}
		c.b.ClearSelection()
		return a.enterInsert()
	}, "i")
	def("append_mode", true, func(a *Core, c *helixCtx) []Effect {
		b := c.b
		if _, e, ok := b.Selection(); ok {
			b.Cur = e
		} else if b.Cur.Col < len(b.Lines[b.Cur.Row]) {
			b.Cur.Col++
		}
		b.ClearSelection()
		return a.enterInsert()
	}, "a")
	def("insert_at_line_start", true, func(a *Core, c *helixCtx) []Effect {
		c.b.ClearSelection()
		c.b.Cur.Col = 0
		c.b.MoveCursor(buffer.Home, false)
		return a.enterInsert()
	}, "I")
	def("insert_at_line_end", true, func(a *Core, c *helixCtx) []Effect {
		c.b.ClearSelection()
		c.b.MoveCursor(buffer.End, false)
		return a.enterInsert()
	}, "A")
	def("open_below", true, func(a *Core, c *helixCtx) []Effect { c.b.OpenLine(true); return a.enterInsert() }, "o")
	def("open_above", true, func(a *Core, c *helixCtx) []Effect { c.b.OpenLine(false); return a.enterInsert() }, "O")
	def("repeat_last_insert", false, func(a *Core, c *helixCtx) []Effect { return a.repeatLastInsert(c.b, c.ext) }, ".")
	def("indent", false, func(a *Core, c *helixCtx) []Effect { c.repeat(a.indentLines(c.b, true)); return nil }, ">")
	def("unindent", false, func(a *Core, c *helixCtx) []Effect { c.repeat(a.indentLines(c.b, false)); return nil }, "<")
	def("join_selections", false, func(a *Core, c *helixCtx) []Effect { c.b.JoinLines(); return nil }, "J")
	def("switch_case", true, func(a *Core, c *helixCtx) []Effect { a.changeCase(c.b, "~"); return nil }, "~")
	def("switch_to_lowercase", true, func(a *Core, c *helixCtx) []Effect { a.changeCase(c.b, "`"); return nil }, "`")
	def("switch_to_uppercase", true, func(a *Core, c *helixCtx) []Effect { a.changeCase(c.b, "alt+`"); return nil }, "alt+`")
	def("toggle_comments", false, func(a *Core, c *helixCtx) []Effect { a.toggleComment(); return nil }, "ctrl+c")
}

// binding is what a key does: a named command or a ":" command line.
type binding struct {
	name string // helixCommands key
	ex   string // ":" command without the colon
}

// helixKeymap is the effective keymap of the three modes.
type helixKeymap struct {
	normal, sel, insert map[string]binding
}

// buildHelixKeymap merges config.toml's [keys.*] over the defaults. An
// unknown command name or an unreadable key is reported in warnings and
// skipped, so the key keeps its default.
func buildHelixKeymap(k config.Keys) (km helixKeymap, warnings []string) {
	km.normal = map[string]binding{}
	for key, name := range helixDefaultKeys {
		km.normal[key] = binding{name: name}
	}
	overlay := func(dst map[string]binding, section string, src map[string]string) {
		keys := make([]string, 0, len(src))
		for key := range src {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := src[key]
			norm, ok := normalizeHelixKey(key)
			if !ok {
				warnings = append(warnings, fmt.Sprintf(i18n.T("keys.badkey"), section, key))
				continue
			}
			switch {
			case strings.HasPrefix(val, ":"):
				if strings.TrimSpace(val[1:]) == "" {
					warnings = append(warnings, fmt.Sprintf(i18n.T("keys.unknowncmd"), section, key, val))
					continue
				}
				dst[norm] = binding{ex: strings.TrimSpace(val[1:])}
			default:
				if _, ok := helixCommands[val]; !ok {
					warnings = append(warnings, fmt.Sprintf(i18n.T("keys.unknowncmd"), section, key, val))
					continue
				}
				dst[norm] = binding{name: val}
			}
		}
	}
	overlay(km.normal, "keys.normal", k.Normal)
	km.sel = make(map[string]binding, len(km.normal))
	maps.Copy(km.sel, km.normal)
	overlay(km.sel, "keys.select", k.Select)
	km.insert = map[string]binding{}
	overlay(km.insert, "keys.insert", k.Insert)
	for _, p := range k.Invalid {
		warnings = append(warnings, fmt.Sprintf(i18n.T("keys.invalid"), p))
	}
	return km, warnings
}

// loadKeymap builds the keymap from the configuration and reports problems
// in the status line.
func (a *Core) loadKeymap() {
	var w []string
	a.keymap, w = buildHelixKeymap(a.cfg.Keys)
	if len(w) == 0 {
		return
	}
	msg := w[0]
	if len(w) > 1 {
		msg += fmt.Sprintf(i18n.T("keys.more"), len(w)-1)
	}
	a.fail(errString(msg))
}

// runBinding runs the command or ":" command a key is bound to.
func (a *Core) runBinding(bd binding, b *buffer.Buffer, ext bool, count int, hadCount bool) []Effect {
	if bd.ex != "" {
		return a.execCommand(bd.ex)
	}
	cmd := helixCommands[bd.name]
	c := &helixCtx{b: b, ext: ext, count: count, hadCount: hadCount}
	var out []Effect
	if cmd.multi && b.HasExtra() {
		b.ForEachCursor(func() { out = cmd.run(a, c) })
	} else {
		out = cmd.run(a, c)
	}
	if insertCommands[bd.name] && a.mode == modeInsert && !a.replaying {
		a.recIns = &insertRecord{bd: bd, count: count, hadCount: hadCount}
	}
	return out
}

// insertCommands are the commands that enter insert mode; "." repeats the
// last of them together with the keys typed until Esc.
var insertCommands = map[string]bool{
	"insert_mode": true, "append_mode": true, "insert_at_line_start": true, "insert_at_line_end": true,
	"open_below": true, "open_above": true, "change_selection": true, "change_selection_noyank": true,
}

// insertRecord is one insert: the command that began it and what followed.
type insertRecord struct {
	bd       binding
	count    int
	hadCount bool
	steps    []insertStep
}

// insertStep is a key typed in insert mode, or a completion that was accepted
// (it replaces the back characters before the cursor with text).
type insertStep struct {
	key        Key
	completion bool
	text       string
	back       int
}

// repeatLastInsert replays the last insert: the command that entered insert
// mode, the keys typed, and Esc. Nothing happens before the first insert.
func (a *Core) repeatLastInsert(b *buffer.Buffer, ext bool) []Effect {
	r := a.lastIns
	if r == nil || a.replaying {
		return nil
	}
	a.replaying = true
	defer func() { a.replaying = false }()
	cmds := [][]Effect{a.runBinding(r.bd, b, ext, r.count, r.hadCount)}
	for _, s := range r.steps {
		if a.mode != modeInsert {
			break
		}
		if !s.completion {
			cmds = append(cmds, a.handleInsertKey(s.key))
			continue
		}
		b.ForEachCursor(func() {
			from := b.Cur
			from.Col = max(0, from.Col-s.back)
			b.SelectRange(from, b.Cur)
			b.InsertBlock(s.text)
		})
		a.follow = true
	}
	if a.mode == modeInsert {
		cmds = append(cmds, a.handleInsertKey(Key{Code: KeyEscape}))
	}
	return batch(cmds...)
}

// normalizeHelixKey turns a Helix key name ("C-s", "A-x", "S-tab", "ret")
// into the string Key.String gives for that key.
func normalizeHelixKey(s string) (string, bool) {
	parts := strings.Split(s, "-")
	var ctrl, alt, shift bool
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "C":
			ctrl = true
		case "A":
			alt = true
		case "S":
			shift = true
		default:
			return "", false
		}
	}
	name := parts[len(parts)-1]
	if name == "" {
		return "", false
	}
	if utf8.RuneCountInString(name) == 1 {
		r, _ := utf8.DecodeRuneInString(name)
		if shift && unicode.IsLower(r) {
			name, shift = string(unicode.ToUpper(r)), false
		}
	} else {
		switch name {
		case "ret", "enter":
			name = "enter"
		case "space", "tab", "esc", "backspace", "left", "right", "up", "down", "home", "end":
		case "del":
			name = "delete"
		case "ins":
			name = "insert"
		case "pageup":
			name = "pgup"
		case "pagedown":
			name = "pgdown"
		case "minus":
			name = "-"
		case "lt":
			name = "<"
		case "gt":
			name = ">"
		case "plus":
			name = "+"
		case "semicolon":
			name = ";"
		case "percent":
			name = "%"
		default:
			var n int
			if _, err := fmt.Sscanf(name, "F%d", &n); err != nil || n < 1 || n > 12 || fmt.Sprintf("F%d", n) != name {
				return "", false
			}
			name = strings.ToLower(name)
		}
	}
	var sb strings.Builder
	if ctrl {
		sb.WriteString("ctrl+")
	}
	if alt {
		sb.WriteString("alt+")
	}
	if shift {
		sb.WriteString("shift+")
	}
	sb.WriteString(name)
	return sb.String(), true
}
