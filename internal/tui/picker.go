package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const topLevel = "(top level)"

// option is one row of the category picker.
type option struct {
	label  string // what the row shows
	value  string // category path; "" for the top level
	create bool   // the row creates a new category
}

// categoryOptions filters known categories by the typed text and offers to create a new one.
func categoryOptions(known []string, typed string) []option {
	typed = strings.Trim(strings.TrimSpace(typed), "/")
	var opts []option
	if typed == "" {
		opts = append(opts, option{label: topLevel, value: ""})
	}
	exact := false
	for _, cat := range known {
		if typed == "" || strings.Contains(strings.ToLower(cat), strings.ToLower(typed)) {
			opts = append(opts, option{label: cat, value: cat})
		}
		if strings.EqualFold(cat, typed) {
			exact = true
		}
	}
	if typed != "" && !exact {
		opts = append(opts, option{label: typed, value: typed, create: true})
	}
	return opts
}

// picker is a filterable category list that also accepts a new name.
type picker struct {
	known  []string
	input  textinput.Model
	cursor int
}

func newPicker(known []string) picker {
	input := textinput.New()
	input.Placeholder = "type to filter, or a new name"
	input.Prompt = ""
	input.CharLimit = 120
	return picker{known: known, input: input}
}

func (p picker) options() []option { return categoryOptions(p.known, p.input.Value()) }

func (p picker) selected() option {
	opts := p.options()
	if p.cursor >= len(opts) {
		return opts[0]
	}
	return opts[p.cursor]
}

func (p *picker) reset() {
	p.input.SetValue("")
	p.cursor = 0
}

func (p *picker) focus() tea.Cmd { return p.input.Focus() }
func (p *picker) blur()          { p.input.Blur() }

// update handles movement keys itself and passes everything else to the input.
func (p picker) update(msg tea.Msg) (picker, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		n := len(p.options())
		switch key.Type {
		case tea.KeyUp:
			p.cursor = (p.cursor + n - 1) % n
			return p, nil
		case tea.KeyDown:
			p.cursor = (p.cursor + 1) % n
			return p, nil
		}
	}
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		p.cursor = 0
	}
	return p, cmd
}

// view renders the input line and the option rows, indented by pad.
func (p picker) view(pad string) string {
	var b strings.Builder
	b.WriteString(p.input.View() + "\n")
	for i, opt := range p.options() {
		marker := "  "
		text := opt.label
		if opt.create {
			text = newSt.Render("+ new category: " + opt.label)
		}
		if i == p.cursor {
			marker = cursorSt.Render("▸ ")
			if !opt.create {
				text = cursorSt.Render(text)
			}
		}
		b.WriteString(pad + marker + text + "\n")
	}
	return b.String()
}
