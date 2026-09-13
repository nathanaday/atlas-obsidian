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
	kinds := ""
	for _, r := range v.rows {
		if r.kind == rowFolded {
			kinds += "F"
		} else {
			kinds += "P"
		}
	}
	if kinds != "PPPF" {
		t.Fatalf("rows %s", kinds)
	}
}

func TestEnterOnFoldedZoomsAndEscReturns(t *testing.T) {
	v := newView(sample(), Opener{}, Hooks{})
	v = pressV(v, tea.KeyUp) // wraps to the folded row
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
	v = pressV(v, tea.KeyDown, tea.KeyEnter) // p3
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
	v = pressV(v, tea.KeyDown) // p3
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
	v = pressV(v, tea.KeyDown, tea.KeyEnter)
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
	v = pressV(v, tea.KeyDown) // p3
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
