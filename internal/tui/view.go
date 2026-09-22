// Package tui is the atlas view: every project on one screen as a map, each project a
// node and each member link an edge, laid out by a live force simulation. It shows and
// launches; creating and changing project content is the CLI's and the session's job.
package tui

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nathanaday/atlas-obsidian/internal/actions"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

// Item is one entry in the view: what the scan found, with the state the last refresh
// derived (nil when none has run).
type Item struct {
	Entry registry.Entry
}

// Items wraps the entries the CLI loaded.
func Items(entries []registry.Entry) []Item {
	items := make([]Item, 0, len(entries))
	for _, e := range entries {
		items = append(items, Item{Entry: e})
	}
	return items
}

// entryName is the name to show: an entry the scan could not read has only its folder.
func entryName(e registry.Entry) string {
	if e.Error != "" || e.Name == "" {
		return filepath.Base(e.Path)
	}
	return e.Name
}

// Opener connects the view to Obsidian and the preferred harness without the screen
// doing the work itself. Obsidian opens the atlas/<name>/ folder at path. Agent runs the
// selected harness in the work folder at path, holding the terminal until the session
// ends.
type Opener struct {
	Obsidian func(path string) error
	Agent    func(harness, path string) error
}

// now is the clock the screens use; tests may replace it.
var now = time.Now

// frame is how often the simulation steps while it has energy.
const frame = 33 * time.Millisecond

// nudgeCells is how far a Shift+arrow moves a node, in cells.
const nudgeCells = 3.0

type tickMsg time.Time

// agentDoneMsg reports that a harness session ended and the view has the terminal back.
type agentDoneMsg struct {
	harness string
	name    string
	err     error
}

// openedMsg reports the outcome of an open that ran in the background.
type openedMsg struct {
	what string // what opened: Obsidian, the IDE, a terminal
	name string
	err  error
}

// refreshedMsg reports that a background refresh finished.
type refreshedMsg struct {
	err error
}

// launch adapts an Opener.Agent call to what Bubble Tea hands the terminal to.
type launch struct {
	run func() error
}

func (l launch) Run() error        { return l.run() }
func (launch) SetStdin(io.Reader)  {}
func (launch) SetStdout(io.Writer) {}
func (launch) SetStderr(io.Writer) {}

// panelKind is what lies over the map, if anything.
type panelKind int

const (
	panelNone panelKind = iota
	panelCard
	panelHelp
)

type view struct {
	items  []Item
	byID   map[string]*Item
	opener Opener
	acts   actions.Atlas
	graph  *graph
	// sel is the selected node, an index into graph.nodes; -1 when there is none.
	// chosen says the user moved it; until then the selection follows the walk's
	// start, the leftmost project, as the map settles.
	sel     int
	chosen  bool
	ticking bool
	panel   panelKind
	// settings is the settings panel, over everything else while open.
	settings      bool
	settingChoice string
	// find is the prompt that jumps to a name, and the selection before it opened.
	find    *textinput.Model
	findWas int
	// stub is the one-line prompt for a new thread, and the project it opens in.
	stub     *textinput.Model
	stubInto *Item
	busy     string
	status   string
	errMsg   string
	width    int
	height   int
	// refreshed is when the registry was last derived, or "".
	refreshed string
	changed   bool
	// cells is where each node was drawn last, for tests and the summary.
	cells []cell
}

// cell is a node's place on the canvas.
type cell struct{ x, y int }

func newView(items []Item, opener Opener, acts actions.Atlas) view {
	v := view{opener: opener, acts: acts, width: 100, height: 40, sel: -1}
	v.take(items, "")
	v.settingChoice = v.harness()
	return v
}

// take installs the entries, builds the map over them, and keeps the selection on the
// project whose id is keep.
func (v *view) take(items []Item, keep string) {
	v.items = items
	v.byID = map[string]*Item{}
	v.refreshed = ""
	for i := range items {
		e := items[i].Entry
		if e.ID != "" {
			v.byID[e.ID] = &items[i]
		}
		if s := e.State; s != nil && s.GeneratedAt > v.refreshed {
			v.refreshed = s.GeneratedAt
		}
	}
	v.graph = buildGraph(items)
	v.sel = -1
	if i, ok := v.graph.byID[keep]; ok {
		v.sel = i
	} else if order := v.graph.tour(); len(order) > 0 {
		v.sel = order[0]
	}
	v.graph.wake()
}

// current is the selected project, or nil.
func (v *view) current() *Item {
	if v.sel < 0 || v.sel >= len(v.graph.nodes) {
		return nil
	}
	return v.item(v.sel)
}

// item is the entry behind a node.
func (v *view) item(i int) *Item {
	n := v.graph.nodes[i]
	if it, ok := v.byID[n.id]; ok {
		return it
	}
	for j := range v.items {
		if "path:"+v.items[j].Entry.Path == n.id {
			return &v.items[j]
		}
	}
	return nil
}

// nameOf is a project's name by id, for the members a card lists.
func (v *view) nameOf(id string) string {
	if it, ok := v.byID[id]; ok {
		return entryName(it.Entry)
	}
	return id
}

// tick schedules the next step of the simulation.
func tick() tea.Cmd {
	return tea.Tick(frame, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// wake puts energy into the map and starts the ticks when they are not running.
func (v *view) wake() tea.Cmd {
	v.graph.wake()
	if v.ticking || v.graph.settled() {
		return nil
	}
	v.ticking = true
	return tick()
}

func (v view) Init() tea.Cmd {
	v.ticking = true
	return tick()
}

// selectNode moves the selection to a node, from then on the user's choice.
func (v *view) selectNode(i int) {
	if i >= 0 && i < len(v.graph.nodes) {
		v.sel = i
		v.chosen = true
	}
}

func (v view) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if !v.ticking {
			return v, nil
		}
		v.graph.step()
		if v.graph.settled() {
			v.ticking = false
			if !v.chosen {
				if order := v.graph.tour(); len(order) > 0 {
					v.sel = order[0]
				}
			}
			return v, nil
		}
		return v, tick()
	case tea.WindowSizeMsg:
		v.width, v.height = msg.Width, msg.Height
		return v, nil
	case agentDoneMsg:
		if msg.err != nil {
			v.errMsg = msg.err.Error()
		} else {
			v.status = "back from " + harnessLabel(msg.harness) + " in " + msg.name
		}
		// A session may have opened or closed threads; read everything again.
		return v.refresh()
	case openedMsg:
		v.busy = ""
		if msg.err != nil {
			v.errMsg = msg.err.Error()
		} else {
			v.status = "opened " + msg.name + " in " + msg.what
		}
		return v, nil
	case refreshedMsg:
		v.busy = ""
		if msg.err != nil {
			v.errMsg = msg.err.Error()
			return v, nil
		}
		keep := ""
		if it := v.current(); it != nil {
			keep = it.Entry.ID
		}
		if v.acts.Load != nil {
			entries, err := v.acts.Load()
			if err != nil {
				v.errMsg = err.Error()
				return v, nil
			}
			v.take(Items(entries), keep)
		}
		if v.status == "" {
			v.status = "refreshed"
		}
		return v, v.wake()
	case tea.KeyMsg:
		return v.key(msg)
	}
	return v, nil
}

// key handles one key press: the prompts first, then the panels, then the map.
func (v view) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return v, tea.Quit
	}
	if v.stub != nil {
		return v.updateStub(msg)
	}
	if v.find != nil {
		return v.updateFind(msg)
	}
	if v.busy != "" {
		return v, nil
	}
	v.status, v.errMsg = "", ""
	if v.settings {
		return v.updateSettings(msg)
	}
	switch msg.String() {
	case "q":
		return v, tea.Quit
	case "?":
		v.panel = togglePanel(v.panel, panelHelp)
		return v, nil
	case ",":
		v.settings = true
		v.settingChoice = v.harness()
		return v, nil
	case "R":
		return v.refresh()
	case "/":
		return v.openFind()
	case "o", "c", "i", "t", "n":
		return v.launch(msg.String())
	}
	switch msg.Type {
	case tea.KeyEnter:
		if v.current() != nil {
			v.panel = togglePanel(v.panel, panelCard)
		}
		return v, nil
	case tea.KeyEsc:
		if v.panel != panelNone {
			v.panel = panelNone
			return v, nil
		}
		return v, tea.Quit
	case tea.KeyRight, tea.KeyDown, tea.KeyTab:
		v.selectNode(v.graph.along(v.sel, 1))
	case tea.KeyLeft, tea.KeyUp, tea.KeyShiftTab:
		v.selectNode(v.graph.along(v.sel, -1))
	case tea.KeyShiftUp, tea.KeyShiftDown, tea.KeyShiftLeft, tea.KeyShiftRight:
		return v.nudge(msg.Type)
	}
	return v, nil
}

// togglePanel opens a panel, or closes it when it is the one open.
func togglePanel(open, want panelKind) panelKind {
	if open == want {
		return panelNone
	}
	return want
}

// nudge pushes the selected node a few cells and lets the map answer.
func (v view) nudge(key tea.KeyType) (tea.Model, tea.Cmd) {
	if v.sel < 0 {
		return v, nil
	}
	_, scale := v.fit()
	step := nudgeCells * 2 / scale
	switch key {
	case tea.KeyShiftUp:
		v.graph.nudge(v.sel, 0, -step)
	case tea.KeyShiftDown:
		v.graph.nudge(v.sel, 0, step)
	case tea.KeyShiftLeft:
		v.graph.nudge(v.sel, -step, 0)
	case tea.KeyShiftRight:
		v.graph.nudge(v.sel, step, 0)
	}
	return v, v.wake()
}

// launch runs one of the launch keys on the selected project.
func (v view) launch(key string) (tea.Model, tea.Cmd) {
	item := v.current()
	if item == nil {
		return v, nil
	}
	if item.Entry.Error != "" {
		v.errMsg = item.Entry.Error
		return v, nil
	}
	switch key {
	case "o":
		return v.open(item)
	case "c":
		return v.agent(item)
	case "i":
		return v.background(item, "opening %s in the IDE…", "the IDE", v.acts.OpenIDE, "opening an IDE")
	case "t":
		return v.background(item, "opening a terminal at %s…", "a terminal", v.acts.OpenTerminal, "opening a terminal")
	case "n":
		return v.openStub(item)
	}
	return v, nil
}

// background runs an action on the entry in the background and reports when it is done.
func (v view) background(item *Item, busy, what string, action func(registry.Entry) error, verb string) (tea.Model, tea.Cmd) {
	if action == nil {
		v.errMsg = verb + " is not available here"
		return v, nil
	}
	name, entry := entryName(item.Entry), item.Entry
	v.busy = fmt.Sprintf(busy, name)
	return v, func() tea.Msg { return openedMsg{what: what, name: name, err: action(entry)} }
}

// refreshCmd rebuilds the registry in the background; nil when no action does it.
func (v view) refreshCmd() tea.Cmd {
	if v.acts.Refresh == nil {
		return nil
	}
	fn := v.acts.Refresh
	return func() tea.Msg {
		_, err := fn()
		return refreshedMsg{err: err}
	}
}

// refresh rebuilds derived state in the background, then reloads the map.
func (v view) refresh() (tea.Model, tea.Cmd) {
	if v.acts.Refresh == nil || v.acts.Load == nil {
		v.errMsg = "refresh is not available here"
		return v, nil
	}
	v.changed = true
	v.busy = "reading every project…"
	return v, v.refreshCmd()
}

// openFind opens the prompt that jumps to a name as it is typed.
func (v view) openFind() (tea.Model, tea.Cmd) {
	in := textinput.New()
	in.Prompt = "  find: "
	in.Width = max(20, v.width-lipgloss.Width(in.Prompt)-4)
	in.Focus()
	v.find, v.findWas = &in, v.sel
	return v, textinput.Blink
}

// updateFind forwards keys to the prompt and moves the selection to the first match;
// Enter keeps it, Esc puts the selection back.
func (v view) updateFind(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		v.find = nil
		v.selectNode(v.findWas)
		return v, nil
	case tea.KeyEnter:
		v.find = nil
		return v, nil
	}
	in, cmd := v.find.Update(msg)
	v.find = &in
	if i := v.graph.find(in.Value()); i >= 0 {
		v.selectNode(i)
	}
	return v, cmd
}

// openStub opens the one-line prompt that opens a thread in a project.
func (v view) openStub(item *Item) (tea.Model, tea.Cmd) {
	if v.acts.StartThread == nil {
		v.errMsg = "opening a thread is not available here"
		return v, nil
	}
	in := textinput.New()
	in.Prompt = "  new thread in " + entryName(item.Entry) + ": "
	in.Width = max(20, v.width-lipgloss.Width(in.Prompt)-4)
	in.Focus()
	v.stub, v.stubInto = &in, item
	return v, textinput.Blink
}

// updateStub forwards keys to the prompt; Enter opens the thread, Esc cancels.
func (v view) updateStub(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		v.stub, v.stubInto = nil, nil
		return v, nil
	case tea.KeyEnter:
		text := strings.TrimSpace(v.stub.Value())
		item := v.stubInto
		v.stub, v.stubInto = nil, nil
		if text == "" {
			return v, nil
		}
		t, err := v.acts.StartThread(item.Entry, threads.New{Text: text})
		if err != nil {
			v.errMsg = err.Error()
			return v, nil
		}
		v.changed = true
		v.status = fmt.Sprintf("opened %s in %s (%s)", t.Title, entryName(item.Entry), t.ID)
		if cmd := v.refreshCmd(); cmd != nil {
			v.busy = "reading every project…"
			return v, cmd
		}
		return v, nil
	}
	in, cmd := v.stub.Update(msg)
	v.stub = &in
	return v, cmd
}

// agent hands the terminal to the preferred harness session in the entry's folder and
// resumes after.
func (v view) agent(item *Item) (tea.Model, tea.Cmd) {
	if v.opener.Agent == nil {
		v.errMsg = "starting the preferred harness is not available here"
		return v, nil
	}
	harness := v.harness()
	name, path, run := entryName(item.Entry), item.Entry.Path, v.opener.Agent
	return v, tea.Exec(launch{run: func() error { return run(harness, path) }}, func(err error) tea.Msg { return agentDoneMsg{name: name, harness: harness, err: err} })
}

// open starts opening the project's folder in Obsidian in the background.
func (v view) open(item *Item) (tea.Model, tea.Cmd) {
	if v.opener.Obsidian == nil {
		v.errMsg = "opening in Obsidian is not available here"
		return v, nil
	}
	name, path, fn := entryName(item.Entry), item.Entry.Path, v.opener.Obsidian
	v.busy = "opening " + name + " in Obsidian…"
	return v, func() tea.Msg {
		p, err := project.Open(path)
		if err != nil {
			return openedMsg{what: "Obsidian", name: name, err: err}
		}
		return openedMsg{what: "Obsidian", name: name, err: fn(p.Atlas())}
	}
}

// The frame: one header line, the map, one summary line, one footer line.

func (v view) mapHeight() int { return max(5, v.height-3) }

// header is the first line: the atlas, its counts, and when it was refreshed.
func (v view) header() string {
	projects, problems, links := 0, 0, len(v.graph.edges())
	for _, it := range v.items {
		if it.Entry.Error != "" {
			problems++
		} else {
			projects++
		}
	}
	left := "  " + title.Render("Atlas")
	var counts []string
	counts = append(counts, fmt.Sprintf("%d project%s", projects, plural(projects)))
	if links > 0 {
		counts = append(counts, fmt.Sprintf("%d link%s", links, plural(links)))
	}
	if problems > 0 {
		counts = append(counts, errSt.Render(fmt.Sprintf("%d problem%s", problems, plural(problems))))
	}
	left += "  " + dim.Render(strings.Join(counts, " · "))
	stamp := dim.Render("not refreshed yet")
	if t, err := time.Parse("2006-01-02T15:04:05Z", v.refreshed); err == nil {
		stamp = dim.Render("refreshed " + t.Local().Format("15:04"))
	}
	if pad := v.width - lipgloss.Width(left) - lipgloss.Width(stamp) - 2; pad >= 2 {
		return left + strings.Repeat(" ", pad) + stamp
	}
	return lipgloss.NewStyle().MaxWidth(v.width).Render(left)
}

// labelOf is a node's marker and name.
func labelOf(n node) string {
	if n.problem {
		return "✗ " + n.name
	}
	return "● " + n.name
}

// fit is the transform from the map's dots to the canvas's: the offset of the map's
// top-left corner and the scale. The map fills the canvas, less room for the labels,
// and is never blown up past a size that would spread a small map across the screen.
func (v view) fit() (offset [2]float64, scale float64) {
	c := v.mapHeight()
	labels := 0
	for _, n := range v.graph.nodes {
		labels = max(labels, lipgloss.Width(labelOf(n)))
	}
	padL, padR, padT, padB := 4.0, float64(2*(labels+2)), 4.0, 6.0
	minX, minY, maxX, maxY := v.graph.bounds()
	w, h := math.Max(1, maxX-minX), math.Max(1, maxY-minY)
	availW, availH := math.Max(4, float64(2*v.width)-padL-padR), math.Max(4, float64(4*c)-padT-padB)
	scale = math.Min(availW/w, availH/h)
	scale = math.Min(scale, 2)
	offset[0] = padL + (availW-w*scale)/2 - minX*scale
	offset[1] = padT + (availH-h*scale)/2 - minY*scale
	return offset, scale
}

// draw renders the map into a canvas: the edges, then the labels, the selected one
// first so it keeps its place, and the panel over all of it.
func (v *view) draw() *canvas {
	c := newCanvas(v.width, v.mapHeight())
	offset, scale := v.fit()
	dots := make([][2]int, len(v.graph.nodes))
	v.cells = make([]cell, len(v.graph.nodes))
	for i, n := range v.graph.nodes {
		x := int(math.Round(n.x*scale + offset[0]))
		y := int(math.Round(n.y*scale + offset[1]))
		dots[i] = [2]int{x, y}
		v.cells[i] = cell{x / 2, y / 4}
	}
	for _, e := range v.graph.edges() {
		var layer int8 = 1
		if e[0] == v.sel || e[1] == v.sel {
			layer = 2
		}
		c.line(dots[e[0]][0], dots[e[0]][1], dots[e[1]][0], dots[e[1]][1], layer)
	}
	order := make([]int, 0, len(v.graph.nodes))
	if v.sel >= 0 {
		order = append(order, v.sel)
	}
	for i := range v.graph.nodes {
		if i != v.sel {
			order = append(order, i)
		}
	}
	for _, i := range order {
		text := v.nodeStyle(i).Render(labelOf(v.graph.nodes[i]))
		v.placeLabel(c, v.cells[i], text)
	}
	if lines := v.overlay(); len(lines) > 0 {
		width := lipgloss.Width(lines[0])
		x := max(0, v.width-width-1)
		for i, line := range lines {
			c.place(x, i, line)
		}
	}
	return c
}

// placeLabel puts a label at its node, or on a free row near it.
func (v view) placeLabel(c *canvas, at cell, text string) {
	width := lipgloss.Width(text)
	x := max(0, min(at.x, v.width-width))
	for _, dy := range []int{0, 1, -1, 2, -2, 3, -3} {
		y := at.y + dy
		if y < 0 || y >= c.h {
			continue
		}
		if c.free(x, y, width) {
			c.place(x, y, text)
			return
		}
	}
	c.place(x, at.y, text)
}

// nodeStyle is how a node's label reads against the selection.
func (v view) nodeStyle(i int) lipgloss.Style {
	n := v.graph.nodes[i]
	switch {
	case i == v.sel:
		return selectedSt
	case n.problem:
		return problemSt
	case v.sel < 0:
		return nodeSt
	case contains(v.graph.nodes[v.sel].members, i):
		return memberSt
	case contains(v.graph.nodes[v.sel].hubs, i):
		return hubSt
	}
	return fadedSt
}

func contains(list []int, i int) bool {
	for _, x := range list {
		if x == i {
			return true
		}
	}
	return false
}

// overlay is the panel over the map, as lines, or nil.
func (v view) overlay() []string {
	width := min(64, max(30, v.width-4))
	tall := v.mapHeight()
	switch {
	case v.settings:
		return panel("Settings", v.settingsLines(), width, tall)
	case v.panel == panelHelp:
		return panel("Keys", helpLines(v.harness()), width, tall)
	case v.panel == panelCard:
		it := v.current()
		if it == nil {
			return nil
		}
		var members, hubs []string
		for _, m := range v.graph.nodes[v.sel].members {
			members = append(members, v.graph.nodes[m].name)
		}
		for _, h := range v.graph.nodes[v.sel].hubs {
			hubs = append(hubs, v.graph.nodes[h].name)
		}
		return panel(entryName(it.Entry), cardLines(it.Entry, members, hubs), width, tall)
	}
	return nil
}

// helpLines is the body of the keys panel.
func helpLines(harness string) []string {
	rows := [][2]string{
		{"→ or Tab", "the next project along the map"},
		{"← or Shift+Tab", "the previous one"},
		{"Shift+arrows", "nudge the selected project; the map answers"},
		{"Enter", "open its card; again to close"},
		{"/", "find a project by name"},
		{"o", "open atlas/<name>/ in Obsidian"},
		{"c", "start " + harnessLabel(harness) + " in the work folder"},
		{"i", "open the work folder in the IDE"},
		{"t", "open a terminal at the work folder"},
		{"n", "open a thread"},
		{"R", "refresh every project"},
		{",", "settings: harness and IDE"},
		{"Esc", "close a panel; on the map, quit"},
		{"q", "quit"},
	}
	var out []string
	for _, r := range rows {
		out = append(out, label.Width(17).Render(r[0])+r[1])
	}
	return out
}

// summary is the line under the map: the selected project, what it mirrors and what
// mirrors it, and its facts.
func (v view) summary() string {
	it := v.current()
	if it == nil {
		if len(v.items) == 0 {
			return dim.Render("no projects yet; run `atlas-obsidian init` in a work folder")
		}
		return ""
	}
	e := it.Entry
	if e.Error != "" {
		return errSt.Render(entryName(e)) + "  " + dim.Render(e.Error)
	}
	n := v.graph.nodes[v.sel]
	parts := []string{memberSt.Render(e.Name)}
	if len(n.members) > 0 {
		var names []string
		for _, m := range n.members {
			names = append(names, v.graph.nodes[m].name)
		}
		parts = append(parts, "mirrors "+memberSt.Render(strings.Join(names, ", ")))
	}
	if len(n.hubs) > 0 {
		var names []string
		for _, h := range n.hubs {
			names = append(names, v.graph.nodes[h].name)
		}
		parts = append(parts, "mirrored by "+hubSt.Render(strings.Join(names, ", ")))
	}
	if e.Description != "" && len(n.members)+len(n.hubs) == 0 {
		parts = append(parts, dim.Render(e.Description))
	}
	for _, f := range facts(e.State) {
		parts = append(parts, dim.Render(f))
	}
	return fitJoin(parts, "  ", v.width-2)
}

// fitJoin joins parts with sep, dropping parts from the end until the line fits width.
// The first part always stays.
func fitJoin(parts []string, sep string, width int) string {
	for len(parts) > 1 {
		line := strings.Join(parts, sep)
		if lipgloss.Width(line) <= width {
			return line
		}
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// footer is the last line: a prompt, progress, a result, or the keys, never wrapped.
func (v view) footer() string {
	switch {
	case v.stub != nil:
		return v.stub.View() + dim.Render("  Enter open · Esc cancel")
	case v.find != nil:
		return v.find.View() + dim.Render("  Enter keep · Esc back")
	case v.busy != "":
		return "  " + okSt.Render(v.busy)
	case v.errMsg != "":
		return "  " + errSt.Render(v.errMsg)
	case v.status != "":
		return "  " + okSt.Render(v.status)
	}
	return "  " + dim.Render(v.hints())
}

// hint is one key in the footer and how soon a narrow screen may drop it: the higher
// the rank, the sooner it goes.
type hint struct {
	text string
	rank int
}

// hints names the keys in order, dropping the least needed until they fit the width.
func (v view) hints() string {
	var keys []hint
	switch {
	case v.settings:
		keys = []hint{{"↑↓ choose", 0}, {"Enter save", 0}, {"Esc close", 1}}
	case v.panel == panelHelp:
		keys = []hint{{"? close", 0}, {"q quit", 1}}
	case v.current() != nil && v.current().Entry.Error != "":
		keys = []hint{{"Enter card", 0}, {"R refresh", 2}, {"←→ select", 3}, {"? keys", 1}, {"q quit", 0}}
	case v.current() != nil:
		enter := "Enter card"
		if v.panel == panelCard {
			enter = "Enter close"
		}
		keys = []hint{{enter, 0}, {"o Obsidian", 4}, {"c " + harnessLabel(v.harness()), 5}, {"i IDE", 6}, {"t terminal", 7}, {"n thread", 8}, {"/ find", 3}, {"R refresh", 10}, {", settings", 9}, {"? keys", 2}, {"q quit", 1}}
	default:
		keys = []hint{{"R refresh", 2}, {", settings", 3}, {"? keys", 1}, {"q quit", 0}}
	}
	for len(keys) > 1 {
		var parts []string
		for _, k := range keys {
			parts = append(parts, k.text)
		}
		if line := strings.Join(parts, " · "); lipgloss.Width(line) <= v.width-2 {
			return line
		}
		drop, at := -1, -1
		for i, k := range keys {
			if k.rank > drop {
				drop, at = k.rank, i
			}
		}
		keys = append(keys[:at], keys[at+1:]...)
	}
	return keys[0].text
}

// View is the frame: exactly the screen's height, every line at most its width.
func (v view) View() string {
	c := v.draw()
	lines := []string{v.header()}
	lines = append(lines, c.render(faint, litSt)...)
	lines = append(lines, "  "+v.summary(), v.footer())
	clip := lipgloss.NewStyle().MaxWidth(max(1, v.width))
	for i := range lines {
		lines[i] = clip.Render(lines[i])
	}
	for len(lines) < v.height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:v.height], "\n")
}

// RunView shows the atlas until the user quits. It reports whether anything changed: a
// new thread or a refresh happened, so the caller reads the registry again.
func RunView(items []Item, opener Opener, acts actions.Atlas) (bool, error) {
	final, err := tea.NewProgram(newView(items, opener, acts), tea.WithAltScreen()).Run()
	if err != nil {
		return false, fmt.Errorf("interactive screen failed: %w", err)
	}
	return final.(view).changed, nil
}
