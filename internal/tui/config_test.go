package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nathanaday/atlas-obsidian/internal/actions"
	"github.com/nathanaday/atlas-obsidian/internal/home"
)

func TestSettingsSavePreferredHarness(t *testing.T) {
	h := home.Home{Root: t.TempDir()}
	cfg := h.Default()
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	acts := actions.Bind(h, cfg, nil)
	v := sized(nil, Opener{}, acts)
	v = keyV(v, ",")
	if !v.settings || !strings.Contains(stripANSI(v.View()), "Claude Code  saved") {
		t.Fatal(stripANSI(v.View()))
	}
	v = pressV(v, tea.KeyDown)
	if cfg.Harness() != "claude" {
		t.Fatal("selection saved before Enter")
	}
	v = pressV(v, tea.KeyEnter)
	saved, err := h.Load()
	if err != nil || saved.Harness() != "codex" || cfg.Harness() != "codex" {
		t.Fatalf("save: %+v %v", saved, err)
	}
	if !strings.Contains(v.status, "Codex") {
		t.Fatal(v.status)
	}
	// Resizing, a refresh landing, and project keys are safe while settings is open.
	next, _ := v.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v = next.(view)
	next, _ = v.Update(refreshedMsg{})
	v = next.(view)
	for _, key := range []string{"o", "c", "i", "t", "n", "/"} {
		v = keyV(v, key)
		if !v.settings || v.busy != "" || v.find != nil || v.stub != nil {
			t.Fatalf("%s under settings", key)
		}
	}
	v = keyV(v, ",")
	if v.settings {
		t.Fatal(", closes settings")
	}
}

func TestSettingsSaveErrorKeepsPreference(t *testing.T) {
	acts := actions.Atlas{
		PreferredHarness:    func() string { return "claude" },
		SetPreferredHarness: func(string) error { return errors.New("disk full") },
	}
	v := sized(nil, Opener{}, acts)
	v = keyV(v, ",")
	v = pressV(v, tea.KeyDown, tea.KeyEnter)
	if v.errMsg != "disk full" || v.harness() != "claude" {
		t.Fatalf("err=%q harness=%q", v.errMsg, v.harness())
	}
	none := sized(nil, Opener{}, actions.Atlas{})
	none = keyV(none, ",")
	none = pressV(none, tea.KeyEnter)
	if !strings.Contains(none.errMsg, "not available") {
		t.Fatal(none.errMsg)
	}
}

func TestSettingsSaveIDEWithoutChangingHarness(t *testing.T) {
	h := home.Home{Root: t.TempDir()}
	cfg := h.Default()
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	acts := actions.Bind(h, cfg, nil)
	v := sized(nil, Opener{}, acts)
	v = keyV(v, ",")
	v = pressV(v, tea.KeyDown, tea.KeyDown, tea.KeyEnter)
	saved, err := h.Load()
	if err != nil || saved.IDE() != "vscode" || saved.Harness() != "claude" {
		t.Fatalf("save: %+v %v", saved, err)
	}
	if !strings.Contains(v.status, "VS Code") {
		t.Fatal(v.status)
	}
}
