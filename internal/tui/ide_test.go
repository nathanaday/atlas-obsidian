package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nathanaday/atlas-obsidian/internal/actions"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
)

func TestIDEKeyOpensProjectRoot(t *testing.T) {
	for _, failure := range []error{nil, errors.New("VS Code unavailable")} {
		opened := ""
		acts := actions.Atlas{OpenIDE: func(en registry.Entry) error { opened = en.Path; return failure }}
		v := findEntry(t, newView(sample(), Opener{}, acts), "webapp")
		next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
		v = next.(view)
		if v.busy == "" || cmd == nil || opened != "" {
			t.Fatal("i should launch in background")
		}
		v = runCmd(v, cmd)
		if opened != "/code/webapp" || v.busy != "" {
			t.Fatalf("opened=%q busy=%q", opened, v.busy)
		}
		if failure != nil {
			if v.errMsg != failure.Error() {
				t.Fatal(v.errMsg)
			}
		} else if !strings.Contains(v.status, "opened webapp in VS Code") {
			t.Fatal(v.status)
		}
	}
	v := findEntry(t, newView(sample(), Opener{}, actions.Atlas{}), "webapp")
	if v = keyV(v, "i"); !strings.Contains(v.errMsg, "not available") {
		t.Fatal(v.errMsg)
	}
}
