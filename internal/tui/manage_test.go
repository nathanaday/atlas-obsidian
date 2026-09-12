package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/tree"
	"github.com/nathanaday/claude-atlas/internal/vaults"
)

func fakeAtlas(t *testing.T) (*home.Config, Hooks) {
	t.Helper()
	root := t.TempDir()
	cfg := &home.Config{Schema: home.ConfigSchema, VaultsDir: filepath.Join(root, "Vaults"), AtlasVault: filepath.Join(root, "Atlas")}
	os.MkdirAll(cfg.TreeRoot(), 0o755)
	for _, spec := range []struct{ name, cat string }{{"capstone", "university/cs566"}, {"reading", "personal"}, {"welcome", ""}} {
		vault := filepath.Join(cfg.VaultsDir, spec.name)
		os.MkdirAll(vault, 0o755)
		os.WriteFile(filepath.Join(vault, ".claude-obsidian.json"), []byte("{}"), 0o644)
		if _, err := vaults.Register(cfg, vault, vaults.RegisterOptions{Name: spec.name, Category: spec.cat}); err != nil {
			t.Fatal(err)
		}
	}
	hooks := Hooks{
		Load: func() ([]*tree.Project, error) {
			projects, _, err := tree.Walk(cfg.TreeRoot())
			return projects, err
		},
		Categories: func() []string { return Categories(cfg.TreeRoot()) },
		State: func(rel string) *tree.State {
			if rel == "personal/reading" {
				return &tree.State{Heat: "cold"}
			}
			return nil
		},
		Update: func(p *tree.Project, edit vaults.Edit) error { return vaults.Update(cfg, p, edit) },
		Unlink: vaults.Unlink,
	}
	return cfg, hooks
}

func pressM(m manage, keys ...tea.KeyType) manage {
	for _, k := range keys {
		next, _ := m.Update(tea.KeyMsg{Type: k})
		m = next.(manage)
	}
	return m
}

func typeM(m manage, text string) manage {
	for _, r := range text {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(manage)
	}
	return m
}

func TestBrowseListsByCategoryAndFolds(t *testing.T) {
	_, hooks := fakeAtlas(t)
	m, err := newManage(hooks)
	if err != nil {
		t.Fatal(err)
	}
	view := m.View()
	t.Logf("\n%s", view)
	for _, want := range []string{"3 projects", "personal", "university/cs566", "Top level", "capstone", "reading", "welcome", "❄️"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q", want)
		}
	}
	// rows: [personal] reading [university/cs566] capstone [top] welcome
	if len(m.rows) != 6 || !m.rows[0].header || m.rows[1].project.ID() != "reading" {
		t.Fatalf("rows %+v", m.rows)
	}
	m = pressM(m, tea.KeyRight) // fold personal
	if len(m.rows) != 5 || !m.collapsed["personal"] {
		t.Fatalf("fold failed: %d rows", len(m.rows))
	}
	m = pressM(m, tea.KeyEnter) // unfold via enter on header
	if len(m.rows) != 6 {
		t.Fatal("unfold failed")
	}
}

func TestEditRenameRepriorityAndMoveCategory(t *testing.T) {
	cfg, hooks := fakeAtlas(t)
	m, _ := newManage(hooks)
	m = pressM(m, tea.KeyDown, tea.KeyEnter) // open reading
	if m.mode != editing || m.current.ID() != "reading" {
		t.Fatalf("mode %d current %v", m.mode, m.current)
	}
	m = pressM(m, tea.KeyEnter) // edit name
	m = typeM(m, " List")
	m = pressM(m, tea.KeyEnter)
	if m.draft.Name != "reading List" {
		t.Fatalf("name %q", m.draft.Name)
	}
	m = pressM(m, tea.KeyDown, tea.KeyDown, tea.KeyEnter) // category picker
	m = typeM(m, "leisure")
	m = pressM(m, tea.KeyEnter)
	m = pressM(m, tea.KeyDown, tea.KeyDown, tea.KeyRight) // priority normal → low
	t.Logf("\n%s", m.View())
	if m.draft.Category != "leisure" || m.draft.Priority != "low" || !m.dirty() {
		t.Fatalf("draft %+v", m.draft)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = next.(manage)
	if m.mode != browse || !m.changed || m.err != "" {
		t.Fatalf("save: mode=%d changed=%v err=%q", m.mode, m.changed, m.err)
	}
	t.Logf("\n%s", m.View())
	projects, _, _ := tree.Walk(cfg.TreeRoot())
	moved := tree.FindByRel(projects, "leisure/reading")
	if moved == nil || moved.Name != "reading List" || moved.Priority != "low" {
		t.Fatalf("not applied: %+v", moved)
	}
	if m.rows[m.cursor].project == nil || m.rows[m.cursor].project.Rel != "leisure/reading" {
		t.Fatalf("cursor not on moved project: %+v", m.rows[m.cursor])
	}
}

func TestEscWarnsBeforeDiscarding(t *testing.T) {
	_, hooks := fakeAtlas(t)
	m, _ := newManage(hooks)
	m = pressM(m, tea.KeyDown, tea.KeyEnter, tea.KeyDown, tea.KeyDown, tea.KeyDown, tea.KeyDown, tea.KeyRight) // priority changed
	m = pressM(m, tea.KeyEsc)
	if m.mode != editing || !strings.Contains(m.err, "unsaved") {
		t.Fatalf("first esc should warn: mode=%d err=%q", m.mode, m.err)
	}
	m = pressM(m, tea.KeyEsc)
	if m.mode != browse || m.changed {
		t.Fatalf("second esc should discard: mode=%d changed=%v", m.mode, m.changed)
	}
}

func TestRemoveUnlinksOnly(t *testing.T) {
	cfg, hooks := fakeAtlas(t)
	m, _ := newManage(hooks)
	m = pressM(m, tea.KeyDown, tea.KeyEnter)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = next.(manage)
	if m.mode != confirmRemove || !strings.Contains(m.View(), "stays on disk") {
		t.Fatalf("mode %d\n%s", m.mode, m.View())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = next.(manage)
	if m.mode != browse || !m.changed || len(m.projects) != 2 {
		t.Fatalf("remove: mode=%d changed=%v projects=%d err=%q", m.mode, m.changed, len(m.projects), m.err)
	}
	if _, err := os.Stat(filepath.Join(cfg.VaultsDir, "reading", ".claude-obsidian.json")); err != nil {
		t.Fatal("vault was deleted")
	}
}

func TestMoveVaultAsksFirst(t *testing.T) {
	cfg, hooks := fakeAtlas(t)
	m, _ := newManage(hooks)
	m = pressM(m, tea.KeyDown, tea.KeyEnter, tea.KeyDown, tea.KeyDown, tea.KeyDown, tea.KeyEnter) // vault field
	target := filepath.Join(cfg.VaultsDir, "archive", "reading")
	m.text.SetValue(target)
	m = pressM(m, tea.KeyEnter)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = next.(manage)
	if m.mode != confirmMove {
		t.Fatalf("expected move confirmation, mode=%d err=%q", m.mode, m.err)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = next.(manage)
	if m.err != "" || m.mode != browse {
		t.Fatalf("move: err=%q mode=%d", m.err, m.mode)
	}
	if _, err := os.Stat(filepath.Join(target, ".claude-obsidian.json")); err != nil {
		t.Fatal("vault not moved")
	}
}
