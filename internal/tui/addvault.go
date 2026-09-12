// Package tui holds the interactive screens behind bare CLI commands.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/tree"
)

// AddVault is what the user chose on the add-vault screen.
type AddVault struct {
	Name     string // display name as typed
	Slug     string // file and directory name
	Path     string // where the vault will be created
	Category string // "" for the top level
	Purpose  string
}

type step int

const (
	stepName step = iota
	stepCategory
	stepPurpose
	stepConfirm
	stepCount
)

var (
	title    = lipgloss.NewStyle().Bold(true)
	label    = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(11)
	activeL  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Width(11)
	dim      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	value    = lipgloss.NewStyle()
	cursorSt = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	newSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	rule     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

type model struct {
	vaultsDir  string
	categories []string
	step       step
	name       textinput.Model
	category   picker
	purpose    textinput.Model
	chosen     option
	err        string
	done       bool
	cancelled  bool
}

func newModel(vaultsDir string, categories []string) model {
	name := textinput.New()
	name.Placeholder = "sensor-triage"
	name.Prompt = ""
	name.CharLimit = 80
	name.Focus()
	category := newPicker(categories)
	purpose := textinput.New()
	purpose.Placeholder = "one line on why this exists (optional)"
	purpose.Prompt = ""
	purpose.CharLimit = 200
	purpose.Width = 60
	return model{vaultsDir: vaultsDir, categories: categories, name: name, category: category, purpose: purpose}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) slug() string {
	slug, err := tree.Slugify(m.name.Value())
	if err != nil {
		return ""
	}
	return slug
}

func (m model) path() string { return filepath.Join(m.vaultsDir, m.slug()) }

func (m model) nameError() string {
	if strings.TrimSpace(m.name.Value()) == "" {
		return "type a name"
	}
	if m.slug() == "" {
		return "the name needs at least one letter or digit"
	}
	if _, err := os.Stat(m.path()); err == nil {
		return home.Display(m.path()) + " already exists"
	}
	return ""
}

func (m *model) focus() tea.Cmd {
	m.name.Blur()
	m.category.blur()
	m.purpose.Blur()
	switch m.step {
	case stepName:
		return m.name.Focus()
	case stepCategory:
		return m.category.focus()
	case stepPurpose:
		return m.purpose.Focus()
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, isKey := msg.(tea.KeyMsg)
	if isKey {
		switch key.Type {
		case tea.KeyCtrlC:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyEsc:
			if m.step == stepName {
				m.cancelled = true
				return m, tea.Quit
			}
			m.step--
			m.err = ""
			return m, m.focus()
		case tea.KeyEnter:
			return m.advance()
		}
	}
	var cmd tea.Cmd
	switch m.step {
	case stepName:
		m.name, cmd = m.name.Update(msg)
		m.err = ""
	case stepCategory:
		m.category, cmd = m.category.update(msg)
	case stepPurpose:
		m.purpose, cmd = m.purpose.Update(msg)
	}
	return m, cmd
}

func (m model) advance() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepName:
		if e := m.nameError(); e != "" {
			m.err = e
			return m, nil
		}
	case stepCategory:
		m.chosen = m.category.selected()
	case stepConfirm:
		m.done = true
		return m, tea.Quit
	}
	m.step++
	m.err = ""
	return m, m.focus()
}

func (m model) pagePath() string {
	if m.chosen.value == "" {
		return "tree/" + m.slug() + ".md"
	}
	return "tree/" + m.chosen.value + "/" + m.slug() + ".md"
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("\n  " + title.Render("Add a vault") + "\n\n")

	// Name
	b.WriteString(m.row(stepName, "Name", m.name.View()))
	if m.step == stepName {
		if m.err != "" {
			b.WriteString("             " + errSt.Render(m.err) + "\n")
		} else if m.slug() != "" {
			b.WriteString("             " + dim.Render("→ "+home.Display(m.path())) + "\n")
		}
	} else if m.step > stepName {
		b.WriteString("             " + dim.Render(home.Display(m.path())) + "\n")
	}
	b.WriteString("\n")

	// Category
	switch {
	case m.step < stepCategory:
		b.WriteString(m.row(stepCategory, "Category", dim.Render("choose after the name")))
	case m.step == stepCategory:
		b.WriteString(m.row(stepCategory, "Category", m.category.view("             ")))
	default:
		shown := m.chosen.label
		if m.chosen.create {
			shown += dim.Render("  (new)")
		}
		b.WriteString(m.row(stepCategory, "Category", shown))
	}
	b.WriteString("\n")

	// Purpose
	switch {
	case m.step < stepPurpose:
		b.WriteString(m.row(stepPurpose, "Purpose", dim.Render("optional")))
	case m.step == stepPurpose:
		b.WriteString(m.row(stepPurpose, "Purpose", m.purpose.View()))
	default:
		p := m.purpose.Value()
		if p == "" {
			p = dim.Render("none")
		}
		b.WriteString(m.row(stepPurpose, "Purpose", p))
	}
	b.WriteString("\n")

	if m.step == stepConfirm {
		b.WriteString("  " + rule.Render(strings.Repeat("─", 56)) + "\n")
		b.WriteString("  " + label.Render("Vault") + value.Render(home.Display(m.path())) + "\n")
		b.WriteString("  " + label.Render("Page") + value.Render(m.pagePath()) + "\n")
		b.WriteString("\n  " + title.Render("Enter") + " create this vault   " + dim.Render("Esc back") + "\n")
	} else {
		hints := "Enter next"
		if m.step == stepCategory {
			hints += " · ↑↓ choose · type to filter or name a new category"
		}
		if m.step > stepName {
			hints += " · Esc back"
		} else {
			hints += " · Esc cancel"
		}
		b.WriteString("  " + dim.Render(hints) + "\n")
	}
	return b.String()
}

func (m model) row(s step, name, content string) string {
	l := label
	if m.step == s {
		l = activeL
	}
	return "  " + l.Render(name) + content + "\n"
}

// Categories lists every directory under the tree root, nested paths included.
func Categories(treeRoot string) []string {
	var cats []string
	filepath.WalkDir(treeRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == treeRoot {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(treeRoot, path)
		cats = append(cats, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(cats)
	return cats
}

// RunAddVault shows the screen and returns nil when the user cancels.
func RunAddVault(vaultsDir string, categories []string) (*AddVault, error) {
	final, err := tea.NewProgram(newModel(vaultsDir, categories)).Run()
	if err != nil {
		return nil, fmt.Errorf("interactive screen failed: %w", err)
	}
	m := final.(model)
	if m.cancelled || !m.done {
		return nil, nil
	}
	return &AddVault{
		Name:     strings.TrimSpace(m.name.Value()),
		Slug:     m.slug(),
		Path:     m.path(),
		Category: m.chosen.value,
		Purpose:  strings.TrimSpace(m.purpose.Value()),
	}, nil
}
