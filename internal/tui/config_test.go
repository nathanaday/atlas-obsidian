package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nathanaday/atlas-obsidian/internal/actions"
	"github.com/nathanaday/atlas-obsidian/internal/home"
)

func TestConfigSavesPreferredHarness(t *testing.T) {
	h := home.Home{Root: t.TempDir()}
	cfg := h.Default()
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	acts := actions.Bind(h, cfg, nil)
	v := newView(nil, Opener{}, acts)
	v = pressV(v, tea.KeyRight)
	if v.tab != tabConfig || !strings.Contains(v.View(), "Claude Code (saved)") {
		t.Fatal(v.View())
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
	// Window resizing, refresh completion, and project keys are safe on Config.
	next, _ := v.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v = next.(view)
	next, _ = v.Update(refreshedMsg{})
	v = next.(view)
	for _, key := range []string{"o", "c", "i", "n", "h"} {
		v = keyV(v, key)
	}
	if v.tab != tabConfig || !strings.Contains(v.View(), "Codex (saved)") {
		t.Fatal(v.View())
	}
	reopened := newView(sample(), Opener{}, actions.Bind(h, saved, nil))
	reopened = findEntry(t, reopened, "webapp")
	if !strings.Contains(reopened.View(), "c Codex") {
		t.Fatal(reopened.View())
	}
	v = pressV(v, tea.KeyUp, tea.KeyEnter)
	if cfg.Harness() != "claude" {
		t.Fatal("could not switch back")
	}
}

func TestConfigSaveErrorKeepsPreference(t *testing.T) {
	v := newView(nil, Opener{}, actions.Atlas{
		PreferredHarness:    func() string { return "claude" },
		SetPreferredHarness: func(string) error { return errors.New("disk unavailable") },
	})
	v = pressV(v, tea.KeyRight, tea.KeyDown, tea.KeyEnter)
	if v.errMsg != "disk unavailable" || v.harness() != "claude" || !strings.Contains(v.View(), "Claude Code (saved)") {
		t.Fatal(v.View())
	}
	v = pressV(v, tea.KeyEsc)
	if v.tab != tabProjects {
		t.Fatal("Escape returns to Projects")
	}
}

func TestConfigSavesIDEWithoutChangingHarness(t *testing.T) {
	h := home.Home{Root: t.TempDir()}
	cfg := h.Default()
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	v := newView(nil, Opener{}, actions.Bind(h, cfg, nil))
	v = pressV(v, tea.KeyRight, tea.KeyDown, tea.KeyDown, tea.KeyEnter)
	saved, err := h.Load()
	if err != nil || saved.PreferredIDE != "vscode" || saved.Harness() != "claude" {
		t.Fatalf("config=%+v err=%v", saved, err)
	}
	if !strings.Contains(v.View(), "> VS Code (saved)") || !strings.Contains(v.status, "saved preferred IDE") {
		t.Fatal(v.View())
	}
	v.acts.SetPreferredIDE = func(string) error { return errors.New("disk unavailable") }
	v = pressV(v, tea.KeyEnter)
	if v.errMsg != "disk unavailable" {
		t.Fatal(v.View())
	}
}
