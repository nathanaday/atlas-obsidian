package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func harnessLabel(harness string) string {
	if harness == "codex" {
		return "Codex"
	}
	return "Claude Code"
}

func (v view) harness() string {
	if v.acts.PreferredHarness != nil {
		return v.acts.PreferredHarness()
	}
	return "claude"
}

func (v view) updateConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown:
		if v.configChoice == "claude" {
			v.configChoice = "codex"
		} else {
			v.configChoice = "claude"
		}
	case tea.KeyEnter:
		if v.acts.SetPreferredHarness == nil {
			v.errMsg = "saving config is not available here"
		} else if err := v.acts.SetPreferredHarness(v.configChoice); err != nil {
			v.errMsg = err.Error()
		} else {
			v.status = "saved preferred harness: " + harnessLabel(v.configChoice)
		}
	case tea.KeyEsc:
		v.goTo(tabProjects)
	}
	return v, nil
}

func (v view) configView() string {
	var b strings.Builder
	b.WriteString(v.head())
	b.WriteString(v.wrapped(title, "Preferred harness"))
	for _, harness := range []string{"claude", "codex"} {
		prefix := "  "
		if v.configChoice == harness {
			prefix = "> "
		}
		label := prefix + harnessLabel(harness)
		if v.harness() == harness {
			label += " (saved)"
		}
		b.WriteString(v.wrapped(captionSt, label))
	}
	b.WriteString(v.wrapped(dim, "The c shortcut starts this harness in the project's work folder."))
	b.WriteString("\n" + v.footer(v.hints()...))
	return v.fit(b.String())
}
