package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/tree"
)

// Item is one project with the state from its last refresh (nil when never refreshed).
type Item struct {
	Project *tree.Project
	State   *tree.State
}

// Opener connects the view to Obsidian without the screen touching the registry itself.
type Opener struct {
	Status          func(vault string) (registered, running bool, err error)
	Open            func(vault string) error
	RegisterAndOpen func(vault string) error
}

// openedMsg reports the outcome of an Obsidian open that ran in the background.
type openedMsg struct {
	name string
	err  error
}

// maxLayers is how many category layers the tree shows before folding deeper ones.
const maxLayers = 3

type catNode struct {
	name     string
	path     string
	children []*catNode
	projects []*Item
}

func (n *catNode) count() int {
	total := len(n.projects)
	for _, c := range n.children {
		total += c.count()
	}
	return total
}

// buildTree arranges the items whose category sits under root into nested category nodes.
func buildTree(items []Item, root string) *catNode {
	top := &catNode{path: root}
	byPath := map[string]*catNode{root: top}
	for i := range items {
		item := &items[i]
		cat := item.Project.Category()
		if root != "" && cat != root && !strings.HasPrefix(cat, root+"/") {
			continue
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(cat, root), "/")
		node := top
		if rel != "" {
			path := root
			for _, seg := range strings.Split(rel, "/") {
				if path == "" {
					path = seg
				} else {
					path += "/" + seg
				}
				child, ok := byPath[path]
				if !ok {
					child = &catNode{name: seg, path: path}
					byPath[path] = child
					node.children = append(node.children, child)
				}
				node = child
			}
		}
		node.projects = append(node.projects, item)
	}
	var sortNode func(n *catNode)
	sortNode = func(n *catNode) {
		sort.Slice(n.children, func(i, j int) bool { return n.children[i].name < n.children[j].name })
		sort.Slice(n.projects, func(i, j int) bool { return n.projects[i].Project.Name < n.projects[j].Project.Name })
		for _, c := range n.children {
			sortNode(c)
		}
	}
	sortNode(top)
	return top
}

type rowKind int

const (
	rowProject rowKind = iota
	rowFolded
)

// viewRow is a selectable element of the rendered tree with its line span.
type viewRow struct {
	kind  rowKind
	item  *Item
	path  string // folded category path
	count int
	start int
	end   int
}

// frame remembers where the user was before zooming into a folded category.
type frame struct {
	root   string
	cursor int
	offset int
}

type view struct {
	items     []Item
	opener    Opener
	root      string
	stack     []frame
	ask       *Item  // project awaiting a register-and-open confirmation
	askRun    bool   // Obsidian was running when we asked, so it will restart
	busy      string // message while an open runs in the background
	status    string
	errMsg    string
	rows      []viewRow
	lines     []string
	cursor    int
	offset    int
	width     int
	height    int
	detail    *Item
	refreshed string
}

var (
	boxSt    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(muted).Padding(0, 1)
	boxSelSt = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("12")).Padding(0, 1)
	guideSt  = lipgloss.NewStyle().Foreground(muted)
	catSt    = lipgloss.NewStyle().Bold(true)
)

func newView(items []Item, opener Opener) view {
	v := view{items: items, opener: opener, width: 100, height: 40}
	for _, it := range items {
		if it.State != nil && it.State.GeneratedAt > v.refreshed {
			v.refreshed = it.State.GeneratedAt
		}
	}
	v.layout()
	return v
}

func (v view) Init() tea.Cmd { return nil }

func heatMark(state *tree.State) string {
	if state == nil {
		return "—"
	}
	switch state.Heat {
	case "new":
		return "✨"
	case "hot":
		return "🔥"
	case "warm":
		return "🌤️"
	case "cold":
		return "❄️"
	}
	return "⛔"
}

func unfinishedCount(state *tree.State) string {
	if state == nil {
		return "—"
	}
	total, known := 0, false
	for _, v := range []*int{state.Unfinished.EmptySections, state.Unfinished.SeedPages, state.Unfinished.DeadLinks} {
		if v != nil {
			total, known = total+*v, true
		}
	}
	if !known {
		return "—"
	}
	return fmt.Sprint(total)
}

func idleText(state *tree.State) string {
	switch {
	case state == nil || state.DaysIdle == nil:
		return "—"
	case *state.DaysIdle == 0:
		return "today"
	default:
		return fmt.Sprintf("%dd", *state.DaysIdle)
	}
}

func pagesText(state *tree.State) string {
	if state == nil || state.Pages == nil {
		return "—"
	}
	return fmt.Sprint(*state.Pages)
}

// layout renders the tree into lines and records where each selectable row lands.
func (v *view) layout() {
	v.lines = nil
	v.rows = nil
	top := buildTree(v.items, v.root)
	v.renderNode(top, 0)
	if len(v.lines) > 0 {
		v.lines = append(v.lines, dim.Render("(end)"))
	}
	if v.cursor >= len(v.rows) {
		v.cursor = max(0, len(v.rows)-1)
	}
}

func (v *view) guide(depth int) string {
	return guideSt.Render(strings.Repeat("│  ", depth))
}

func (v *view) renderNode(n *catNode, depth int) {
	for _, item := range n.projects {
		v.renderBox(item, depth)
	}
	for _, child := range n.children {
		if depth+1 > maxLayers {
			start := len(v.lines)
			v.lines = append(v.lines, v.guide(depth)+"▸ "+catSt.Render(child.name)+dim.Render(fmt.Sprintf("  %d project%s · Enter to open", child.count(), plural(child.count()))))
			v.rows = append(v.rows, viewRow{kind: rowFolded, path: child.path, count: child.count(), start: start, end: start})
			continue
		}
		v.lines = append(v.lines, v.guide(depth)+"▾ "+catSt.Render(child.name))
		v.renderNode(child, depth+1)
	}
}

func (v *view) renderBox(item *Item, depth int) {
	width := min(46, max(24, v.width-depth*3-6))
	selected := len(v.rows) == v.cursor
	style := boxSt
	if selected {
		style = boxSelSt
	}
	name := item.Project.Name
	if selected {
		name = selSt.Render(name)
	}
	body := fmt.Sprintf("%s %s\n%s", heatMark(item.State), name,
		dim.Render(fmt.Sprintf("%s pages · %s unfinished · idle %s", pagesText(item.State), unfinishedCount(item.State), idleText(item.State))))
	box := style.Width(width).Render(body)
	start := len(v.lines)
	for _, line := range strings.Split(box, "\n") {
		v.lines = append(v.lines, v.guide(depth)+line)
	}
	v.rows = append(v.rows, viewRow{kind: rowProject, item: item, start: start, end: len(v.lines) - 1})
}

func (v *view) ensureVisible() {
	if len(v.rows) == 0 {
		v.offset = 0
		return
	}
	avail := v.bodyHeight()
	r := v.rows[v.cursor]
	if r.start < v.offset {
		v.offset = r.start
	}
	if r.end >= v.offset+avail {
		v.offset = r.end - avail + 1
	}
	if v.offset < 0 {
		v.offset = 0
	}
}

func (v view) bodyHeight() int { return max(5, v.height-6) }

func (v view) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width, v.height = msg.Width, msg.Height
		v.layout()
		v.ensureVisible()
		return v, nil
	case openedMsg:
		v.busy = ""
		if msg.err != nil {
			v.errMsg = msg.err.Error()
		} else {
			v.status = "opened " + msg.name + " in Obsidian"
		}
		return v, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			return v, tea.Quit
		}
		if v.busy != "" {
			return v, nil
		}
		if v.ask != nil {
			switch strings.ToLower(msg.String()) {
			case "y":
				item := v.ask
				v.ask = nil
				return v.openAsync(item, true)
			case "n", "esc":
				v.ask = nil
			}
			return v, nil
		}
		v.status, v.errMsg = "", ""
		if msg.String() == "o" {
			if v.detail != nil {
				return v.open(v.detail)
			}
			if len(v.rows) > 0 && v.rows[v.cursor].kind == rowProject {
				return v.open(v.rows[v.cursor].item)
			}
			return v, nil
		}
		if v.detail != nil {
			if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter || msg.Type == tea.KeyLeft {
				v.detail = nil
			}
			return v, nil
		}
		switch msg.Type {
		case tea.KeyUp:
			if len(v.rows) > 0 {
				v.cursor = (v.cursor + len(v.rows) - 1) % len(v.rows)
			}
		case tea.KeyDown:
			if len(v.rows) > 0 {
				v.cursor = (v.cursor + 1) % len(v.rows)
			}
		case tea.KeyEnter, tea.KeyRight:
			if len(v.rows) == 0 {
				return v, nil
			}
			r := v.rows[v.cursor]
			if r.kind == rowFolded {
				v.stack = append(v.stack, frame{root: v.root, cursor: v.cursor, offset: v.offset})
				v.root = r.path
				v.cursor = 0
				v.offset = 0
			} else {
				v.detail = r.item
			}
		case tea.KeyEsc, tea.KeyLeft:
			if len(v.stack) == 0 {
				if msg.Type == tea.KeyEsc {
					return v, tea.Quit
				}
				return v, nil
			}
			last := v.stack[len(v.stack)-1]
			v.stack = v.stack[:len(v.stack)-1]
			v.root, v.cursor, v.offset = last.root, last.cursor, last.offset
		}
		v.layout()
		v.ensureVisible()
	}
	return v, nil
}

// open starts opening a project's vault, asking first when Obsidian does not know it.
func (v view) open(item *Item) (tea.Model, tea.Cmd) {
	if v.opener.Status == nil {
		v.errMsg = "opening vaults is not available here"
		return v, nil
	}
	registered, running, err := v.opener.Status(item.Project.VaultPath())
	if err != nil {
		v.errMsg = err.Error()
		return v, nil
	}
	if registered {
		return v.openAsync(item, false)
	}
	v.ask = item
	v.askRun = running
	return v, nil
}

func (v view) openAsync(item *Item, register bool) (tea.Model, tea.Cmd) {
	name, vault := item.Project.Name, item.Project.VaultPath()
	fn := v.opener.Open
	v.busy = "opening " + name + " in Obsidian…"
	if register {
		fn = v.opener.RegisterAndOpen
		v.busy = "registering " + name + " with Obsidian…"
		if v.askRun {
			v.busy = "registering " + name + " and restarting Obsidian…"
		}
	}
	return v, func() tea.Msg { return openedMsg{name: name, err: fn(vault)} }
}

// footer renders the prompt, progress, or status lines under a screen.
func (v view) footer(hints string) string {
	switch {
	case v.ask != nil:
		question := "Register it as a vault?"
		if v.askRun {
			question += " Obsidian will quit and relaunch."
		}
		return fmt.Sprintf("  %s Obsidian does not know %s.\n  %s\n  %s / %s\n",
			errSt.Render("▲"), v.ask.Project.Name, question, title.Render("y"), title.Render("n"))
	case v.busy != "":
		return "  " + okSt.Render(v.busy) + "\n"
	}
	out := "  " + dim.Render(hints) + "\n"
	if v.status != "" {
		out += "  " + okSt.Render(v.status) + "\n"
	}
	if v.errMsg != "" {
		out += "  " + errSt.Render(v.errMsg) + "\n"
	}
	return out
}

func (v view) View() string {
	if v.detail != nil {
		return v.viewDetail()
	}
	var b strings.Builder
	crumb := "all projects"
	if v.root != "" {
		crumb = strings.ReplaceAll(v.root, "/", " › ")
	}
	stamp := ""
	if t, err := time.Parse("2006-01-02T15:04:05Z", v.refreshed); err == nil {
		stamp = "refreshed " + t.Local().Format("2006-01-02 15:04")
	} else {
		stamp = "not refreshed yet"
	}
	fmt.Fprintf(&b, "\n  %s   %s   %s\n\n", title.Render("Atlas"), catSt.Render(crumb), dim.Render(fmt.Sprintf("%d project%s · %s", len(v.items), plural(len(v.items)), stamp)))
	if len(v.lines) == 0 {
		b.WriteString("  " + dim.Render("no projects yet; run `claude-atlas new-vault`") + "\n")
	}
	end := min(len(v.lines), v.offset+v.bodyHeight())
	for _, line := range v.lines[v.offset:end] {
		b.WriteString("  " + line + "\n")
	}
	if end < len(v.lines) {
		b.WriteString("  " + dim.Render(fmt.Sprintf("… %d more lines", len(v.lines)-end)) + "\n")
	}
	hints := "↑↓ move · Enter details · o open in Obsidian"
	if len(v.stack) > 0 {
		hints += " · Esc back"
	}
	b.WriteString("\n" + v.footer(hints+" · q quit"))
	return b.String()
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func (v view) viewDetail() string {
	p, s := v.detail.Project, v.detail.State
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s   %s\n\n", title.Render(p.Name), dim.Render("tree/"+p.Rel+".md"))
	if s == nil {
		b.WriteString("  " + dim.Render("never refreshed; run `claude-atlas refresh`") + "\n\n")
	} else {
		summary := heatMark(s) + " " + dash(s.Heat)
		if s.Created != "" {
			summary += " · created " + s.Created
		}
		if s.LastTouched != "" {
			summary += " · last touched " + s.LastTouched + " (" + idleText(s) + ")"
		}
		b.WriteString("  " + summary + "\n\n")
	}
	row := func(k, val string) { fmt.Fprintf(&b, "  %s%s\n", label.Width(16).Render(k), val) }
	row("Vault", home.Display(p.VaultPath()))
	row("Category", dash(p.Category()))
	row("Priority", p.Priority)
	row("State", p.State)
	row("Blocked on", dash(p.BlockedOn))
	row("Review after", dash(p.ReviewAfter))
	if s != nil {
		row("Pages", pagesText(s))
		u := s.Unfinished
		var bits []string
		for _, kv := range []struct {
			k string
			v *int
		}{{"empty sections", u.EmptySections}, {"seed pages", u.SeedPages}, {"dead links", u.DeadLinks}} {
			if kv.v != nil {
				bits = append(bits, fmt.Sprintf("%d %s", *kv.v, kv.k))
			}
		}
		row("Unfinished", dash(strings.Join(bits, " · ")))
		row("Last operation", dash(s.LastOperation))
		check := okSt.Render("ok")
		if !s.VaultOK {
			check = errSt.Render(dash(s.VaultError))
		}
		row("Vault check", check)
		if t, err := time.Parse("2006-01-02T15:04:05Z", s.GeneratedAt); err == nil {
			row("Refreshed", t.Local().Format("2006-01-02 15:04"))
		}
	}
	section := func(name, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		b.WriteString("\n  " + catSt.Render(name) + "\n")
		for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
			b.WriteString("    " + line + "\n")
		}
	}
	section("Purpose", p.Purpose)
	section("Done when", p.DefinitionOfDone)
	if s != nil && len(s.OpenThreads) > 0 {
		b.WriteString("\n  " + catSt.Render("Open threads") + "\n")
		for _, t := range s.OpenThreads {
			b.WriteString("    - " + t + "\n")
		}
	}
	if len(p.Repos) > 0 {
		b.WriteString("\n  " + catSt.Render("Repos") + "\n")
		for _, r := range p.Repos {
			b.WriteString("    - " + r + "\n")
		}
	}
	b.WriteString("\n" + v.footer("o open in Obsidian · Esc back · q quit"))
	return b.String()
}

// RunView shows the tree until the user quits.
func RunView(items []Item, opener Opener) error {
	_, err := tea.NewProgram(newView(items, opener), tea.WithAltScreen()).Run()
	if err != nil {
		return fmt.Errorf("interactive screen failed: %w", err)
	}
	return nil
}
