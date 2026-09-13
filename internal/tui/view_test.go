package tui

import (
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nathanaday/claude-atlas/internal/tree"
)

func item(rel, name string, heat string) Item {
	cat := ""
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		cat = rel[:i]
	}
	four := 4
	p := &tree.Project{Path: "/tree/" + rel + ".md", Rel: rel, Frontmatter: tree.Frontmatter{Name: name, Vault: "/v/" + name, Priority: "normal", State: "active"}}
	_ = cat
	return Item{Project: p, State: &tree.State{Heat: heat, Pages: &four, GeneratedAt: "2026-09-12T18:00:00Z", OpenThreads: []string{"thread"}}}
}

func pressV(v view, keys ...tea.KeyType) view {
	for _, k := range keys {
		next, _ := v.Update(tea.KeyMsg{Type: k})
		v = next.(view)
	}
	return v
}

func sample() []Item {
	return []Item{
		item("welcome", "welcome", "new"),
		item("engineering/itl/p3", "p3", "hot"),
		item("engineering/usc/cs566/course", "course", "cold"),
		item("engineering/usc/cs566/deep/deeper/buried", "buried", "warm"),
		item("engineering/usc/cs566/deep/other", "other", "warm"),
	}
}

func TestTreeShowsThreeLayersAndFoldsDeeper(t *testing.T) {
	v := newView(sample(), Opener{}, Hooks{})
	out := v.View()
	t.Logf("\n%s", out)
	for _, want := range []string{"▾ engineering", "▾ itl", "▾ usc", "▾ cs566", "▸ deep", "2 projects", "welcome", "p3", "course"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, "buried") || strings.Contains(out, "other") {
		t.Fatal("projects under a folded category should not render")
	}
	if !strings.HasSuffix(v.lines[len(v.lines)-1], "(end)") {
		t.Fatal("tree should end with an explicit (end) marker")
	}
	if got := kinds(v); got != "PCCPCCPF" {
		t.Fatalf("rows %s", got)
	}
}

func kinds(v view) string {
	out := ""
	for _, r := range v.rows {
		switch r.kind {
		case rowFolded:
			out += "F"
		case rowCategory:
			out += "C"
		default:
			out += "P"
		}
	}
	return out
}

func TestDownRevealsTheEndAndNeverWraps(t *testing.T) {
	v := newView(sample(), Opener{}, Hooks{})
	next, _ := v.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	v = next.(view)
	v = pressV(v, tea.KeyUp)
	if v.cursor != 0 {
		t.Fatal("up at the top must stay at the top")
	}
	for i := 0; i < len(v.rows)-1; i++ {
		v = pressV(v, tea.KeyDown)
	}
	if v.cursor != len(v.rows)-1 || !strings.Contains(v.View(), "more lines") {
		t.Fatalf("last row should be selected with the end still hidden: cursor=%d\n%s", v.cursor, v.View())
	}
	v = pressV(v, tea.KeyDown)
	out := v.View()
	if !v.atEnd() || strings.Contains(out, "more lines") || !strings.Contains(out, "(end)") {
		t.Fatalf("one more down should reveal the end marker: cursor=%d\n%s", v.cursor, out)
	}
	v = pressV(v, tea.KeyDown)
	if !v.atEnd() {
		t.Fatal("down at the end must stay at the end")
	}
	v = pressV(v, tea.KeyEnter) // nothing to open here
	if v.detail != nil {
		t.Fatal("enter on the end marker opens nothing")
	}
	v = pressV(v, tea.KeyUp)
	if v.atEnd() || v.cursor != len(v.rows)-1 {
		t.Fatal("up from the end returns to the last row")
	}
}

func TestHintsFollowTheCursor(t *testing.T) {
	v := newView(sample(), Opener{}, Hooks{})
	if out := v.View(); !strings.Contains(out, "o Obsidian · c Claude · e edit") {
		t.Fatalf("project hints missing:\n%s", out)
	}
	v = pressV(v, tea.KeyDown) // engineering
	out := v.View()
	if strings.Contains(out, "Obsidian") || !strings.Contains(out, "Enter fold · - + fold all") {
		t.Fatalf("category hints wrong:\n%s", out)
	}
	v = pressV(v, tea.KeySpace)
	if out := v.View(); !strings.Contains(out, "Enter unfold") {
		t.Fatalf("folded category hints wrong:\n%s", out)
	}
	v = keyV(v, "+")
	for i := 0; i <= len(v.rows); i++ {
		v = pressV(v, tea.KeyDown)
	}
	if hints := v.treeHints(); !v.atEnd() || strings.Contains(hints, "Enter") || strings.Contains(hints, "Obsidian") {
		t.Fatalf("end marker hints wrong: %q", hints)
	}
}

func TestFoldBranchAndFoldAll(t *testing.T) {
	v := newView(sample(), Opener{}, Hooks{})
	v = pressV(v, tea.KeyDown, tea.KeyDown, tea.KeyDown) // welcome, engineering, itl, p3
	if r := v.current(); r.kind != rowProject || r.item.Project.Name != "p3" {
		t.Fatalf("cursor on %+v", r)
	}
	v = pressV(v, tea.KeySpace) // collapses itl and moves onto it
	if r := v.current(); r.kind != rowCategory || r.path != "engineering/itl" || !v.collapsed["engineering/itl"] {
		t.Fatalf("after fold: %+v collapsed=%v", r, v.collapsed)
	}
	out := v.View()
	if strings.Contains(out, "p3") || !strings.Contains(out, "▸ itl") || !strings.Contains(out, "1 project") {
		t.Fatalf("folded branch still shows its project:\n%s", out)
	}
	v = pressV(v, tea.KeyEnter) // enter on a category expands it again
	if v.collapsed["engineering/itl"] || !strings.Contains(v.View(), "p3") {
		t.Fatal("enter should expand the category")
	}
	v = keyV(v, "-")
	if got := kinds(v); got != "PC" || !strings.Contains(v.View(), "▸ engineering") {
		t.Fatalf("collapse all: rows %s\n%s", got, v.View())
	}
	if r := v.current(); r == nil || r.path != "engineering" {
		t.Fatalf("collapse all should leave the cursor on the visible ancestor: %+v", r)
	}
	v = keyV(v, "+")
	if got := kinds(v); got != "PCCPCCPF" || v.current().path != "engineering" {
		t.Fatalf("expand all: rows %s cursor %+v", got, v.current())
	}
	v = pressV(v, tea.KeyLeft) // left on a category folds it
	if r := v.current(); r.path != "engineering" || !v.collapsed["engineering"] || kinds(v) != "PC" {
		t.Fatalf("left on category: %+v rows %s", r, kinds(v))
	}
	v = pressV(v, tea.KeyUp, tea.KeySpace) // a top-level project has no branch to fold
	if kinds(v) != "PC" || v.cursor != 0 {
		t.Fatal("space on a top-level project changes nothing")
	}
}

func TestEnterOnFoldedZoomsAndEscReturns(t *testing.T) {
	v := newView(sample(), Opener{}, Hooks{})
	for i := 0; i < len(v.rows)-1; i++ {
		v = pressV(v, tea.KeyDown) // down to the folded row at the bottom
	}
	if v.rows[v.cursor].kind != rowFolded {
		t.Fatalf("cursor on %+v", v.rows[v.cursor])
	}
	v = pressV(v, tea.KeyEnter)
	out := v.View()
	t.Logf("\n%s", out)
	if v.root != "engineering/usc/cs566/deep" || !strings.Contains(out, "▾ deeper") || !strings.Contains(out, "buried") || !strings.Contains(out, "other") {
		t.Fatalf("zoom failed: root=%q\n%s", v.root, out)
	}
	if !strings.Contains(out, "engineering › usc › cs566 › deep") {
		t.Fatal("breadcrumb missing")
	}
	v = pressV(v, tea.KeyEsc)
	if v.root != "" || v.rows[v.cursor].kind != rowFolded || v.rows[v.cursor].path != "engineering/usc/cs566/deep" {
		t.Fatalf("esc should return to where the user came from: root=%q row=%+v", v.root, v.rows[v.cursor])
	}
}

func TestDetailShowsEverything(t *testing.T) {
	v := newView(sample(), Opener{}, Hooks{})
	v = pressV(v, tea.KeyDown, tea.KeyDown, tea.KeyDown, tea.KeyEnter) // p3
	out := v.View()
	t.Logf("\n%s", out)
	for _, want := range []string{"p3", "tree/engineering/itl/p3.md", "🔥 hot", "Vault", "/v/p3", "Pages", "4", "Open threads", "- thread", "Vault check", "Esc back"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	v = pressV(v, tea.KeyEsc)
	if v.detail != nil {
		t.Fatal("esc should close the detail")
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	v := newView(sample(), Opener{}, Hooks{})
	next, _ := v.Update(tea.WindowSizeMsg{Width: 80, Height: 14})
	v = next.(view)
	v = pressV(v, tea.KeyDown, tea.KeyDown, tea.KeyDown)
	r := v.rows[v.cursor]
	if r.start < v.offset || r.end >= v.offset+v.bodyHeight() {
		t.Fatalf("cursor row %d-%d not within offset %d + %d", r.start, r.end, v.offset, v.bodyHeight())
	}
	if !strings.Contains(v.View(), "more lines") && v.offset == 0 {
		t.Fatal("expected scrolling")
	}
}

func TestEmptyTree(t *testing.T) {
	v := newView(nil, Opener{}, Hooks{})
	if !strings.Contains(v.View(), "no projects yet") {
		t.Fatal("empty message missing")
	}
	v = pressV(v, tea.KeyDown, tea.KeyEnter) // must not panic
}

type fakeOpener struct {
	registered      map[string]bool
	running         bool
	opened          []string
	registeredCalls []string
}

func (f *fakeOpener) opener() Opener {
	return Opener{
		Status: func(vault string) (bool, bool, error) { return f.registered[vault], f.running, nil },
		Open:   func(vault string) error { f.opened = append(f.opened, vault); return nil },
		RegisterAndOpen: func(vault string) error {
			f.registeredCalls = append(f.registeredCalls, vault)
			f.opened = append(f.opened, vault)
			return nil
		},
	}
}

func runCmd(v view, cmd tea.Cmd) view {
	if cmd == nil {
		return v
	}
	next, _ := v.Update(cmd())
	return next.(view)
}

func TestOpenRegisteredVaultDirectly(t *testing.T) {
	f := &fakeOpener{registered: map[string]bool{"/v/p3": true}}
	v := newView(sample(), f.opener(), Hooks{})
	v = pressV(v, tea.KeyDown, tea.KeyDown, tea.KeyDown) // p3
	next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	v = next.(view)
	if v.busy == "" || cmd == nil {
		t.Fatalf("expected a background open, busy=%q", v.busy)
	}
	v = runCmd(v, cmd)
	if len(f.opened) != 1 || f.opened[0] != "/v/p3" || len(f.registeredCalls) != 0 || !strings.Contains(v.View(), "opened p3 in Obsidian") {
		t.Fatalf("opened=%v registered=%v\n%s", f.opened, f.registeredCalls, v.View())
	}
}

func TestOpenUnknownVaultAsksThenRegisters(t *testing.T) {
	f := &fakeOpener{registered: map[string]bool{}, running: true}
	v := newView(sample(), f.opener(), Hooks{})
	next, _ := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")}) // welcome
	v = next.(view)
	if v.ask == nil || !strings.Contains(v.View(), "quit and relaunch") {
		t.Fatalf("expected a confirmation\n%s", v.View())
	}
	next, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	v = next.(view)
	if v.ask != nil || len(f.registeredCalls) != 0 {
		t.Fatal("n should cancel without registering")
	}
	next, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	v = next.(view)
	next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	v = next.(view)
	if !strings.Contains(v.busy, "restarting Obsidian") {
		t.Fatalf("busy=%q", v.busy)
	}
	v = runCmd(v, cmd)
	if len(f.registeredCalls) != 1 || f.registeredCalls[0] != "/v/welcome" || v.busy != "" {
		t.Fatalf("registered=%v busy=%q", f.registeredCalls, v.busy)
	}
}

func TestOpenFromDetail(t *testing.T) {
	f := &fakeOpener{registered: map[string]bool{"/v/p3": true}}
	v := newView(sample(), f.opener(), Hooks{})
	v = pressV(v, tea.KeyDown, tea.KeyDown, tea.KeyDown, tea.KeyEnter)
	next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	v = runCmd(next.(view), cmd)
	if len(f.opened) != 1 || v.detail == nil {
		t.Fatalf("opened=%v detail=%v", f.opened, v.detail)
	}
}

func TestClaudeKeyHandsOffTheTerminal(t *testing.T) {
	var got string
	op := Opener{Claude: func(vault string) (*exec.Cmd, error) { got = vault; return exec.Command("true"), nil }}
	v := newView(sample(), op, Hooks{})
	v = pressV(v, tea.KeyDown, tea.KeyDown, tea.KeyDown) // p3
	next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	v = next.(view)
	if got != "/v/p3" || cmd == nil || v.errMsg != "" {
		t.Fatalf("vault=%q cmd=%v err=%q", got, cmd, v.errMsg)
	}
	next, _ = v.Update(claudeDoneMsg{name: "p3"})
	if !strings.Contains(next.(view).View(), "back from Claude Code in p3") {
		t.Fatal("status after return missing")
	}
	none := newView(sample(), Opener{}, Hooks{})
	next, _ = none.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if next.(view).errMsg == "" {
		t.Fatal("missing launcher should report an error")
	}
}
