package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core"
)

// leftDiffApp is gitDiffApp with a.txt (HEAD "a b c d e", working tree
// "a c d x e") shown side by side in Helix mode.
func leftDiffApp(t *testing.T) *App {
	t.Helper()
	a, _ := gitDiffApp(t)
	a.Helix = true
	a.Gd.Split = true
	a.DiffFor(a.CurText())
	return a
}

func TestLeftSideSelectsAndCopies(t *testing.T) {
	a := leftDiffApp(t)
	right := a.CurText()
	rightCur := right.Buf.Cur
	keys(a, "space", "w", "h")
	l := a.DiffLeft(a.CurText())
	if l == nil {
		t.Fatal("Space w h did not put the cursor on the left side")
	}
	keys(a, "j", "x", "y") // the line "b", which the working tree deleted
	if got := a.Clipboard; got != "b\n" {
		t.Errorf("copied %q, want \"b\\n\"", got)
	}
	if right.Buf.Cur != rightCur {
		t.Errorf("the right cursor moved to %v", right.Buf.Cur)
	}
	if !strings.Contains(a.StatusOf().Position.String(), "selected") {
		t.Errorf("the status line does not tell the left selection: %v", a.StatusOf().Position)
	}
}

func TestLeftSideCannotBeEdited(t *testing.T) {
	a := leftDiffApp(t)
	file := fileText(a)
	before := string(file)
	keys(a, "space", "w", "h")
	l := a.DiffLeft(a.CurText())
	keys(a, "j", "x", "d") // delete the line
	if a.Msg.IsZero() || !a.MsgErr {
		t.Errorf("a refused delete shows no error: %v", a.Msg)
	}
	keys(a, "i", "z", "esc") // type into it
	if got := string(l.Buf.Lines[1]); got != "b" {
		t.Errorf("the base line is now %q", got)
	}
	if l.Buf.Dirty || a.CurText().Buf.Dirty {
		t.Error("an edit went through")
	}
	if got := string(fileText(a)); got != before {
		t.Errorf("the file changed: %q", got)
	}
}

func fileText(a *App) []byte {
	b, _ := os.ReadFile(a.CurText().Buf.Path)
	return b
}

func TestLeftSideBackToRight(t *testing.T) {
	a := leftDiffApp(t)
	keys(a, "space", "w", "h")
	keys(a, "j")
	keys(a, "space", "w", "l")
	if a.DiffLeft(a.CurText()) != nil {
		t.Fatal("Space w l kept the cursor on the left")
	}
	keys(a, "j")
	if got := a.CurText().Buf.Cur.Row; got != 1 {
		t.Errorf("j after going back moved the right cursor to row %d, want 1", got)
	}
	keys(a, "space", "w", "w")
	if a.DiffLeft(a.CurText()) == nil {
		t.Error("Space w w did not switch to the left")
	}
}

func TestLeftSideNeedsSplit(t *testing.T) {
	a := leftDiffApp(t)
	a.Gd.Split = false
	keys(a, "space", "w", "h")
	if a.DiffLeft(a.CurText()) != nil {
		t.Error("the unified diff has a left side")
	}
	dv := openNewFile(t, a)
	a.Gd.Split = true
	keys(a, "space", "w", "h")
	if a.DiffLeft(a.CurText()) != nil || dv.Left != nil {
		t.Error("a new file has a left side")
	}
}

func TestLeftSideMouse(t *testing.T) {
	a := leftDiffApp(t)
	l := a.layout()
	g := a.dvGeom(l)
	dv := a.DiffFor(a.CurText())
	// the display row of the base line "b" (index 1)
	y := g.Top + dv.OldIdx[1] - dv.Start(a.CurText().Buf.ScrollRow)
	press := func(x, y int) {
		a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	}
	press(l.rightX+core.GutterWidthFor(len(dv.OldIdx)), y)
	lt := a.DiffLeft(a.CurText())
	if lt == nil {
		t.Fatal("clicking the left side did not put the cursor there")
	}
	if got := lt.Buf.Cur.Row; got != 1 {
		t.Errorf("the left cursor is on row %d, want 1", got)
	}
	// drag to the next line selects across the two
	a.HandleMouse(a.locate(l.rightX+core.GutterWidthFor(len(dv.OldIdx))+1, y+1, core.Mouse{Action: core.MouseMotion, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(l.rightX+core.GutterWidthFor(len(dv.OldIdx))+1, y+1, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	if got := lt.Buf.SelectedText(); got == "" {
		t.Errorf("dragging on the left selected nothing (cursor %v)", lt.Buf.Cur)
	}
	// clicking the right side moves the cursor back
	press(g.RightX+g.RightW/2, y)
	if a.DiffLeft(a.CurText()) != nil {
		t.Error("clicking the right side kept the cursor on the left")
	}
	view := ansi.Strip(a.render())
	if !strings.Contains(view, "a.txt") {
		t.Error("the view does not draw")
	}
}

func TestLeftSideReachesALongDeletion(t *testing.T) {
	a, file := gitDiffApp(t)
	a.Helix = true
	var old, cur []string
	old = append(old, "top")
	for i := range 60 {
		old = append(old, "gone"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	old = append(old, "bottom")
	cur = []string{"top", "bottom"}
	os.WriteFile(file, []byte(strings.Join(old, "\n")+"\n"), 0o644)
	gitIn(t, a.Root, "add", "a.txt")
	gitIn(t, a.Root, "commit", "-q", "-m", "long")
	os.WriteFile(file, []byte(strings.Join(cur, "\n")+"\n"), 0o644)
	a.Apply(func() []core.Effect {
		a.ApplyChanges(core.LoadChanges(a.Gd.Gen, a.Root, core.BaseHead, nil).(core.ChangesLoadedMsg))
		return nil
	})
	a.OpenChange(a.Gd.File())
	a.Gd.Split = true
	a.Apply(func() []core.Effect { a.FocusDiffSide(true); return nil })
	keys(a, "G") // the last base line
	view := ansi.Strip(a.render())
	if !strings.Contains(view, "bottom") {
		t.Errorf("the last base line is not shown after G:\n%s", view)
	}
	keys(a, "g", "g")
	for range 40 {
		keys(a, "j")
	}
	view = ansi.Strip(a.render())
	lt := a.DiffLeft(a.CurText())
	if lt == nil {
		t.Fatal("no left side")
	}
	if want := string(lt.Buf.Lines[lt.Buf.Cur.Row]); !strings.Contains(view, want) {
		t.Errorf("the line under the cursor %q is not on screen:\n%s", want, view)
	}
}

func TestLeftSideInVSCodeMode(t *testing.T) {
	a := leftDiffApp(t)
	a.Helix = false
	a.Update(core.Key{Code: '1', Mod: core.ModCtrl})
	if a.DiffLeft(a.CurText()) == nil {
		t.Fatal("Ctrl+1 did not put the cursor on the left side")
	}
	a.Update(core.Key{Code: KeyDown})
	a.Update(core.Key{Code: KeyDown, Mod: core.ModShift})
	a.Update(core.Key{Code: 'c', Mod: core.ModCtrl})
	if a.Clipboard != "b\n" {
		t.Errorf("copied %q, want \"b\\n\"", a.Clipboard)
	}
	a.Update(core.Key{Code: '2', Mod: core.ModCtrl})
	if a.DiffLeft(a.CurText()) != nil {
		t.Error("Ctrl+2 kept the cursor on the left side")
	}
}

func TestLeftSideSpaceYAndP(t *testing.T) {
	a := leftDiffApp(t)
	keys(a, "space", "w", "h")
	keys(a, "j", "x", "space", "y")
	if a.Clipboard != "b\n" {
		t.Errorf("Space y copied %q, want the left selection", a.Clipboard)
	}
	before := string(fileText(a))
	keys(a, "space", "p")
	if string(fileText(a)) != before || a.CurText().Buf.Dirty {
		t.Error("Space p changed the file from the left side")
	}
}

// A double click selects the word in the side it is on, the left one
// included, and leaves the other side alone.
func TestDoubleClickSelectsWordOnBothSides(t *testing.T) {
	a := leftDiffApp(t)
	l := a.layout()
	g := a.dvGeom(l)
	dv := a.DiffFor(a.CurText())
	right := a.CurText()
	press := func(x, y, clicks int) {
		a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MousePress, Button: core.MouseLeft, Clicks: clicks}))
		a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	}
	// the base line "b", which the working tree deleted
	y := g.Top + dv.OldIdx[1] - dv.Start(right.Buf.ScrollRow)
	x := l.rightX + core.GutterWidthFor(len(dv.OldIdx))
	press(x, y, 1)
	lt := a.DiffLeft(a.CurText())
	if lt == nil {
		t.Fatal("clicking the left side did not put the cursor there")
	}
	if got := lt.Buf.SelectedText(); got != "" {
		t.Fatalf("a single click selected %q", got)
	}
	press(x, y, 2)
	if got := lt.Buf.SelectedText(); got != "b" {
		t.Errorf("a double click on the left selected %q, want %q", got, "b")
	}
	if got := right.Buf.SelectedText(); got != "" {
		t.Errorf("the right side selected %q", got)
	}
	// the right side: the line "c"
	y = g.Top + dv.NewIdx[1] - dv.Start(right.Buf.ScrollRow)
	press(g.RightX+core.GutterWidth(right.Buf), y, 1)
	press(g.RightX+core.GutterWidth(right.Buf), y, 2)
	if a.DiffLeft(a.CurText()) != nil {
		t.Fatal("clicking the right side left the cursor on the left")
	}
	if got := right.Buf.SelectedText(); got != "c" {
		t.Errorf("a double click on the right selected %q, want %q", got, "c")
	}
}
