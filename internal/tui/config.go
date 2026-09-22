package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// The settings panel: one list of choices, a harness and an IDE, with Enter saving the
// one under the cursor.
var settingChoices = []string{"claude", "codex", "vscode"}

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

func (v view) ide() string {
	if v.acts.PreferredIDE != nil {
		return v.acts.PreferredIDE()
	}
	return "vscode"
}

// updateSettings moves along the choices, saves one, or closes the panel.
func (v view) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown:
		delta := 1
		if msg.Type == tea.KeyUp {
			delta = -1
		}
		for i, choice := range settingChoices {
			if choice == v.settingChoice {
				v.settingChoice = settingChoices[max(0, min(len(settingChoices)-1, i+delta))]
				break
			}
		}
	case tea.KeyEnter:
		if v.settingChoice == "vscode" {
			if v.acts.SetPreferredIDE == nil {
				v.errMsg = "saving settings is not available here"
			} else if err := v.acts.SetPreferredIDE("vscode"); err != nil {
				v.errMsg = err.Error()
			} else {
				v.status = "saved preferred IDE: VS Code"
			}
			return v, nil
		}
		if v.acts.SetPreferredHarness == nil {
			v.errMsg = "saving settings is not available here"
		} else if err := v.acts.SetPreferredHarness(v.settingChoice); err != nil {
			v.errMsg = err.Error()
		} else {
			v.status = "saved preferred harness: " + harnessLabel(v.settingChoice)
		}
	case tea.KeyEsc:
		v.settings = false
	default:
		if msg.String() == "," || msg.String() == "q" {
			v.settings = false
		}
	}
	return v, nil
}

// settingsLines is the body of the settings panel.
func (v view) settingsLines() []string {
	var out []string
	line := func(choice, text string, saved bool) {
		prefix := "  "
		if v.settingChoice == choice {
			prefix = "> "
		}
		if saved {
			text += dim.Render("  saved")
		}
		if v.settingChoice == choice {
			out = append(out, memberSt.Render(prefix+text))
		} else {
			out = append(out, prefix+text)
		}
	}
	out = append(out, label.Render("Harness"), faint.Render("what c starts in the work folder"))
	for _, harness := range []string{"claude", "codex"} {
		line(harness, harnessLabel(harness), v.harness() == harness)
	}
	out = append(out, "", label.Render("IDE"), faint.Render("what i opens the work folder in"))
	line("vscode", "VS Code", v.ide() == "vscode")
	return out
}
