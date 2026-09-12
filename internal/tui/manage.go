package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/tree"
	"github.com/nathanaday/claude-atlas/internal/vaults"
)

// Hooks connect the manage screen to the atlas without the screen touching disk itself.
type Hooks struct {
	Load       func() ([]*tree.Project, error)
	Categories func() []string
	State      func(rel string) *tree.State
	Update     func(*tree.Project, vaults.Edit) error
	Unlink     func(*tree.Project) error
}

type manageMode int

const (
	browse manageMode = iota
	editing
	editText
	editCategory
	confirmRemove
	confirmMove
)

const (
	fieldName = iota
	fieldPurpose
	fieldCategory
	fieldVault
	fieldPriority
	fieldState
	fieldCount
)

var fieldNames = [fieldCount]string{"Name", "Purpose", "Category", "Vault", "Priority", "State"}

type draft struct {
	Name, Purpose, Category, Vault, Priority, State string
}

func draftOf(p *tree.Project) draft {
	return draft{Name: p.Name, Purpose: p.Purpose, Category: p.Category(), Vault: p.VaultPath(), Priority: p.Priority, State: p.State}
}

func (d draft) get(field int) string {
	return [fieldCount]string{d.Name, d.Purpose, d.Category, d.Vault, d.Priority, d.State}[field]
}

func (d *draft) set(field int, v string) {
	switch field {
	case fieldName:
		d.Name = v
	case fieldPurpose:
		d.Purpose = v
	case fieldCategory:
		d.Category = v
	case fieldVault:
		d.Vault = v
	case fieldPriority:
		d.Priority = v
	case fieldState:
		d.State = v
	}
}

// row is one line of the browse list: a category header or a project.
type row struct {
	header   bool
	category string
	project  *tree.Project
	state    *tree.State
}

type manage struct {
	hooks     Hooks
	projects  []*tree.Project
	rows      []row
	collapsed map[string]bool
	cursor    int
	mode      manageMode
	current   *tree.Project
	original  draft
	draft     draft
	field     int
	text      textinput.Model
	picker    picker
	status    string
	err       string
	discard   bool
	changed   bool
	pendingMv vaults.Edit
}

var (
	headerSt = lipgloss.NewStyle().Bold(true)
	selSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	okSt     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	modSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

func newManage(hooks Hooks) (manage, error) {
	text := textinput.New()
	text.Prompt = ""
	text.CharLimit = 300
	text.Width = 60
	m := manage{hooks: hooks, collapsed: map[string]bool{}, text: text}
	if err := m.reload(""); err != nil {
		return m, err
	}
	return m, nil
}

// reload re-reads the tree and puts the cursor on the project named by rel, if any.
func (m *manage) reload(rel string) error {
	projects, err := m.hooks.Load()
	if err != nil {
		return err
	}
	m.projects = projects
	m.picker = newPicker(m.hooks.Categories())
	m.rows = nil
	category := "\x00"
	for _, p := range projects {
		if cat := p.Category(); cat != category {
			category = cat
			m.rows = append(m.rows, row{header: true, category: cat})
		}
		if m.collapsed[category] {
			continue
		}
		m.rows = append(m.rows, row{category: category, project: p, state: m.hooks.State(p.Rel)})
	}
	if rel != "" {
		for i, r := range m.rows {
			if r.project != nil && r.project.Rel == rel {
				m.cursor = i
				return nil
			}
		}
	}
	if m.cursor >= len(m.rows) {
		m.cursor = max(0, len(m.rows)-1)
	}
	return nil
}

func (m manage) Init() tea.Cmd { return nil }

func (m manage) dirty() bool { return m.draft != m.original }

func (m manage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, isKey := msg.(tea.KeyMsg)
	if isKey && key.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	switch m.mode {
	case browse:
		return m.updateBrowse(msg)
	case editing:
		return m.updateEdit(msg)
	case editText:
		if isKey {
			switch key.Type {
			case tea.KeyEnter:
				m.draft.set(m.field, strings.TrimSpace(m.text.Value()))
				m.mode = editing
				return m, nil
			case tea.KeyEsc:
				m.mode = editing
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.text, cmd = m.text.Update(msg)
		return m, cmd
	case editCategory:
		if isKey {
			switch key.Type {
			case tea.KeyEnter:
				m.draft.Category = m.picker.selected().value
				m.mode = editing
				return m, nil
			case tea.KeyEsc:
				m.mode = editing
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.picker, cmd = m.picker.update(msg)
		return m, cmd
	case confirmRemove:
		if isKey {
			switch strings.ToLower(key.String()) {
			case "y":
				name := m.current.Name
				if err := m.hooks.Unlink(m.current); err != nil {
					m.err = err.Error()
					m.mode = editing
					return m, nil
				}
				m.changed = true
				m.status = fmt.Sprintf("removed %s from the atlas; the vault is still on disk", name)
				m.mode = browse
				if err := m.reload(""); err != nil {
					m.err = err.Error()
				}
				return m, nil
			case "n", "esc":
				m.mode = editing
			}
		}
		return m, nil
	case confirmMove:
		if isKey {
			switch strings.ToLower(key.String()) {
			case "y":
				m.pendingMv.MoveVault = true
				return m.apply(m.pendingMv)
			case "n", "esc":
				m.mode = editing
			}
		}
		return m, nil
	}
	return m, nil
}

func (m manage) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok || len(m.rows) == 0 {
		if ok && (key.Type == tea.KeyEsc || key.String() == "q") {
			return m, tea.Quit
		}
		return m, nil
	}
	m.status = ""
	switch key.Type {
	case tea.KeyUp:
		m.cursor = (m.cursor + len(m.rows) - 1) % len(m.rows)
	case tea.KeyDown:
		m.cursor = (m.cursor + 1) % len(m.rows)
	case tea.KeyLeft, tea.KeyRight, tea.KeySpace:
		r := m.rows[m.cursor]
		m.collapsed[r.category] = !m.collapsed[r.category]
		if err := m.reload(""); err != nil {
			m.err = err.Error()
		}
		for i, row := range m.rows {
			if row.header && row.category == r.category {
				m.cursor = i
			}
		}
	case tea.KeyEnter:
		r := m.rows[m.cursor]
		if r.header {
			m.collapsed[r.category] = !m.collapsed[r.category]
			if err := m.reload(""); err != nil {
				m.err = err.Error()
			}
			return m, nil
		}
		m.current = r.project
		m.original = draftOf(r.project)
		m.draft = m.original
		m.field = 0
		m.err = ""
		m.discard = false
		m.mode = editing
	case tea.KeyEsc:
		return m, tea.Quit
	default:
		if key.String() == "q" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m manage) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	m.err = ""
	switch key.Type {
	case tea.KeyUp:
		m.field = (m.field + fieldCount - 1) % fieldCount
	case tea.KeyDown:
		m.field = (m.field + 1) % fieldCount
	case tea.KeyLeft, tea.KeyRight:
		if m.field == fieldPriority || m.field == fieldState {
			m.cycle(key.Type == tea.KeyRight)
		}
	case tea.KeyEnter:
		switch m.field {
		case fieldCategory:
			m.picker.reset()
			m.mode = editCategory
			return m, m.picker.focus()
		case fieldPriority, fieldState:
			m.cycle(true)
		default:
			m.text.SetValue(m.draft.get(m.field))
			m.text.CursorEnd()
			m.mode = editText
			return m, m.text.Focus()
		}
	case tea.KeyEsc:
		if m.dirty() && !m.discard {
			m.discard = true
			m.err = "unsaved changes: press s to save, or Esc again to discard"
			return m, nil
		}
		m.mode = browse
	default:
		switch key.String() {
		case "s":
			return m.save()
		case "r":
			m.mode = confirmRemove
		}
	}
	return m, nil
}

func (m *manage) cycle(forward bool) {
	values := tree.Priorities
	if m.field == fieldState {
		values = tree.States
	}
	current := m.draft.get(m.field)
	idx := 0
	for i, v := range values {
		if v == current {
			idx = i
		}
	}
	if forward {
		idx = (idx + 1) % len(values)
	} else {
		idx = (idx + len(values) - 1) % len(values)
	}
	m.draft.set(m.field, values[idx])
}

func (m manage) save() (tea.Model, tea.Cmd) {
	if !m.dirty() {
		m.mode = browse
		m.status = "no changes"
		return m, nil
	}
	if strings.TrimSpace(m.draft.Name) == "" {
		m.err = "the name cannot be empty"
		return m, nil
	}
	edit := vaults.Edit{Name: m.draft.Name, Priority: m.draft.Priority, State: m.draft.State}
	if m.draft.Purpose != m.original.Purpose {
		edit.Purpose = m.draft.Purpose
		edit.ClearPurpose = m.draft.Purpose == ""
	}
	if m.draft.Category != m.original.Category {
		cat := m.draft.Category
		edit.Category = &cat
	}
	if m.draft.Vault != m.original.Vault {
		target, _ := filepath.Abs(home.Expand(m.draft.Vault))
		edit.Vault = target
		if _, err := os.Stat(target); err != nil {
			m.pendingMv = edit
			m.mode = confirmMove
			return m, nil
		}
	}
	return m.apply(edit)
}

func (m manage) apply(edit vaults.Edit) (tea.Model, tea.Cmd) {
	if err := m.hooks.Update(m.current, edit); err != nil {
		m.err = err.Error()
		m.mode = editing
		return m, nil
	}
	m.changed = true
	rel := m.current.ID()
	if edit.Category != nil {
		if *edit.Category != "" {
			rel = *edit.Category + "/" + rel
		}
	} else if m.current.Category() != "" {
		rel = m.current.Category() + "/" + rel
	}
	m.status = "saved " + m.draft.Name
	m.mode = browse
	if err := m.reload(rel); err != nil {
		m.err = err.Error()
	}
	return m, nil
}

func heatOf(state *tree.State) string {
	if state == nil {
		return dim.Render("—  ")
	}
	switch state.Heat {
	case "hot":
		return "🔥 "
	case "warm":
		return "🌤️ "
	case "cold":
		return "❄️ "
	}
	return "⛔ "
}

func (m manage) View() string {
	switch m.mode {
	case browse:
		return m.viewBrowse()
	case confirmRemove:
		return m.viewEdit() + fmt.Sprintf("\n  %s Remove %s from the atlas? The vault at %s stays on disk.  %s\n",
			errSt.Render("▲"), m.current.Name, home.Display(m.current.VaultPath()), title.Render("y")+" / "+title.Render("n"))
	case confirmMove:
		return m.viewEdit() + fmt.Sprintf("\n  %s %s does not exist. Move the vault directory there?  %s\n",
			errSt.Render("▲"), home.Display(m.pendingMv.Vault), title.Render("y")+" / "+title.Render("n"))
	default:
		return m.viewEdit()
	}
}

func (m manage) viewBrowse() string {
	var b strings.Builder
	count := len(m.projects)
	fmt.Fprintf(&b, "\n  %s   %s\n\n", title.Render("Manage vaults"), dim.Render(fmt.Sprintf("%d project%s", count, plural(count))))
	if len(m.rows) == 0 {
		b.WriteString("  " + dim.Render("no projects yet; run `claude-atlas new-vault`") + "\n")
	}
	for i, r := range m.rows {
		selected := i == m.cursor
		if r.header {
			arrow := "▾"
			if m.collapsed[r.category] {
				arrow = "▸"
			}
			name := r.category
			if name == "" {
				name = "Top level"
			}
			line := arrow + " " + headerSt.Render(name)
			if selected {
				line = selSt.Render(arrow+" ") + selSt.Render(name)
			}
			b.WriteString("  " + line + "\n")
			continue
		}
		name := fmt.Sprintf("%-22s", r.project.Name)
		if selected {
			name = selSt.Render(name)
		}
		b.WriteString(fmt.Sprintf("      %s%s %-8s %-9s %s\n", heatOf(r.state), name, r.project.Priority, r.project.State, dim.Render(home.Display(r.project.VaultPath()))))
	}
	b.WriteString("\n  " + dim.Render("↑↓ move · Enter open · ←→ fold · q quit") + "\n")
	if m.status != "" {
		b.WriteString("  " + okSt.Render(m.status) + "\n")
	}
	if m.err != "" {
		b.WriteString("  " + errSt.Render(m.err) + "\n")
	}
	return b.String()
}

func (m manage) viewEdit() string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s   %s\n\n", title.Render(m.current.Name), dim.Render("tree/"+m.current.Rel+".md"))
	for f := 0; f < fieldCount; f++ {
		marker := "  "
		name := label.Render(fieldNames[f])
		if f == m.field {
			marker = cursorSt.Render("▸ ")
			name = activeL.Render(fieldNames[f])
		}
		var content string
		switch {
		case m.mode == editText && f == m.field:
			content = m.text.View()
		case m.mode == editCategory && f == m.field:
			content = m.picker.view("               ")
		case f == fieldPriority || f == fieldState:
			content = "◂ " + m.draft.get(f) + " ▸"
		case f == fieldCategory:
			content = m.draft.Category
			if content == "" {
				content = topLevel
			}
		case f == fieldVault:
			content = home.Display(m.draft.Vault)
		default:
			content = m.draft.get(f)
			if content == "" {
				content = dim.Render("none")
			}
		}
		if m.draft.get(f) != m.original.get(f) {
			content = strings.TrimRight(content, "\n") + modSt.Render("  •")
		}
		b.WriteString("  " + marker + name + strings.TrimRight(content, "\n") + "\n")
	}
	b.WriteString("\n")
	switch m.mode {
	case editText:
		b.WriteString("  " + dim.Render("Enter keep · Esc cancel") + "\n")
	case editCategory:
		b.WriteString("  " + dim.Render("↑↓ choose · type to filter or name a new category · Enter keep · Esc cancel") + "\n")
	default:
		hints := "↑↓ field · Enter edit · ←→ change"
		if m.dirty() {
			hints += " · " + title.Render("s") + " save"
		}
		hints += " · r remove · Esc back"
		b.WriteString("  " + dim.Render(hints) + "\n")
	}
	if m.err != "" {
		b.WriteString("  " + errSt.Render(m.err) + "\n")
	}
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// RunManage shows the manage screen. It reports whether anything was changed.
func RunManage(hooks Hooks) (bool, error) {
	m, err := newManage(hooks)
	if err != nil {
		return false, err
	}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return false, fmt.Errorf("interactive screen failed: %w", err)
	}
	return final.(manage).changed, nil
}
