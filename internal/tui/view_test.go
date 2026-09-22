package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nathanaday/atlas-obsidian/internal/actions"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

// ansiProfile is termenv.ANSI. Lip Gloss renders plain text under go test, where there
// is no TTY; this profile makes it emit escape codes for one test. The termenv module
// is not imported directly, so the value is spelled out.
const ansiProfile = 2

func withColor(t *testing.T) {
	t.Helper()
	was := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(ansiProfile)
	t.Cleanup(func() { lipgloss.SetColorProfile(was) })
}

func proj(name string, members ...string) Item {
	four, one := 4, 1
	return Item{Entry: registry.Entry{
		ID: "id-" + name, Name: name, Path: "/code/" + name, Mode: project.Generic,
		Created: "2026-09-01", Description: "the " + name + " work", Members: members,
		State: &registry.State{OK: true, Heat: "warm", Pages: &four, Inbox: &one,
			GeneratedAt: "2026-09-12T18:00:00Z", HotTopics: []string{"thread"}, LastOperation: "2026-09-10"},
	}}
}

// sample is a hub over two projects, one of which mirrors the third, one project on
// its own, and one folder the scan could not read.
func sample() []Item {
	items := []Item{proj("platform", "id-webapp", "id-firmware"), proj("webapp", "id-thesis"), proj("firmware"), proj("thesis"), proj("notes")}
	zero := 0
	items[1].Entry.State.DaysIdle = &zero
	items[1].Entry.State.Threads = &registry.ThreadSummary{
		Counts: threads.Counts{Open: 3, Plan: 1, Spec: 2, Phases: 1},
		Open:   []registry.ThreadLine{{ID: "thr-20260917-0001", Title: "Filter vehicle false alarms", Stage: "plan", Priority: "high", Phase: "Alarm quality"}},
		Phases: []string{"Alarm quality", "Launch"},
	}
	items[1].Entry.State.Described = &registry.Description{Page: "wiki/entities/webapp.md", Commit: "abc1234", Behind: 2}
	items = append(items, Item{Entry: registry.Entry{Path: "/old/gateway", Error: missingError, Reason: registry.ReasonMissing}})
	return items
}

const missingError = "not found; work in it again to heal the path, or run atlas-obsidian forget"

func entriesOf(items []Item) []registry.Entry {
	out := make([]registry.Entry, 0, len(items))
	for _, it := range items {
		out = append(out, it.Entry)
	}
	return out
}

// sized is a view at a known size with its map settled.
func sized(items []Item, opener Opener, acts actions.Atlas) view {
	v := newView(items, opener, acts)
	next, _ := v.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	v = next.(view)
	v.graph.settle()
	v.ticking = false
	return v
}

// selectName puts the selection on the node with that name.
func selectName(t *testing.T, v view, name string) view {
	t.Helper()
	for i, n := range v.graph.nodes {
		if n.name == name {
			v.sel = i
			return v
		}
	}
	t.Fatalf("no node %s", name)
	return v
}

func pressV(v view, keys ...tea.KeyType) view {
	for _, k := range keys {
		next, _ := v.Update(tea.KeyMsg{Type: k})
		v = next.(view)
	}
	return v
}

func keyV(v view, s string) view {
	next, _ := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return next.(view)
}

func runCmd(v view, cmd tea.Cmd) view {
	if cmd == nil {
		return v
	}
	next, _ := v.Update(cmd())
	return next.(view)
}

// quits reports whether a command is tea.Quit.
func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func selectedName(v view) string { return v.graph.nodes[v.sel].name }

func TestTheScreenIsAMapOfEveryProject(t *testing.T) {
	v := sized(sample(), Opener{}, actions.Atlas{})
	screen := v.View()
	lines := strings.Split(screen, "\n")
	if len(lines) != 30 {
		t.Fatalf("%d lines, want the screen's 30", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > 100 {
			t.Fatalf("line %d is %d wide:\n%s", i, w, line)
		}
	}
	if !strings.Contains(lines[0], "Atlas") || !strings.Contains(lines[0], "5 projects") || !strings.Contains(lines[0], "3 links") || !strings.Contains(lines[0], "1 problem") {
		t.Fatalf("header %q", lines[0])
	}
	for _, name := range []string{"● platform", "● webapp", "● firmware", "● thesis", "● notes", "✗ gateway"} {
		if !strings.Contains(screen, name) {
			t.Fatalf("the map lacks %s:\n%s", name, screen)
		}
	}
	if !strings.ContainsFunc(screen, func(r rune) bool { return r >= 0x2801 && r <= 0x28ff }) {
		t.Fatalf("no braille edges:\n%s", screen)
	}
	// The labels never overlap: every name appears whole.
	plain := stripANSI(screen)
	for _, name := range []string{"platform", "webapp", "firmware", "thesis", "notes", "gateway"} {
		if strings.Count(plain, name) < 1 {
			t.Fatalf("%s is cut:\n%s", name, plain)
		}
	}
	// No caption, no tabs, no end marker.
	for _, absent := range []string{"A project is an", "Projects (", "(end)", "Config"} {
		if strings.Contains(plain, absent) {
			t.Fatalf("%q should be gone:\n%s", absent, plain)
		}
	}
}

func TestTheSummaryNamesTheLinksOfTheSelection(t *testing.T) {
	v := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "platform")
	s := stripANSI(v.summary())
	if !strings.Contains(s, "platform") || !strings.Contains(s, "mirrors firmware, webapp") && !strings.Contains(s, "mirrors webapp, firmware") {
		t.Fatalf("summary %q", s)
	}
	if strings.Contains(s, "no threads") || strings.Contains(s, "inbox empty") {
		t.Fatalf("a zero is left out: %q", s)
	}
	v = selectName(t, v, "webapp")
	s = stripANSI(v.summary())
	if !strings.Contains(s, "mirrors thesis") || !strings.Contains(s, "mirrored by platform") || !strings.Contains(s, "3 threads open") || !strings.Contains(s, "touched today") {
		t.Fatalf("summary %q", s)
	}
	v = selectName(t, v, "gateway")
	if s = stripANSI(v.summary()); !strings.Contains(s, "gateway") || !strings.Contains(s, "not found") {
		t.Fatalf("problem summary %q", s)
	}
	v = sized(nil, Opener{}, actions.Atlas{})
	if !strings.Contains(v.View(), "no projects yet; run `atlas-obsidian init`") {
		t.Fatal(v.View())
	}
}

func TestTheSelectionLightsItsNeighbors(t *testing.T) {
	withColor(t)
	v := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "webapp")
	styles := map[string]lipgloss.Style{}
	for i, n := range v.graph.nodes {
		styles[n.name] = v.nodeStyle(i)
	}
	if styles["webapp"].GetBackground() != projectColor {
		t.Fatal("the selected node is filled")
	}
	if styles["thesis"].GetForeground() != projectColor || !styles["thesis"].GetBold() {
		t.Fatal("a member wears the project color")
	}
	if styles["platform"].GetForeground() != hubColor {
		t.Fatal("a hub wears the hub color")
	}
	if styles["notes"].GetForeground() != mutedColor {
		t.Fatal("the rest fade")
	}
	if styles["gateway"].GetForeground() != problemColor {
		t.Fatal("a problem is red")
	}
}

func TestArrowsTabAndFindMoveTheSelection(t *testing.T) {
	v := sized(sample(), Opener{}, actions.Atlas{})
	if selectedName(v) != "firmware" {
		t.Fatalf("the first by name is selected: %s", selectedName(v))
	}
	v = pressV(v, tea.KeyTab)
	if selectedName(v) != "gateway" {
		t.Fatalf("Tab: %s", selectedName(v))
	}
	v = pressV(v, tea.KeyShiftTab, tea.KeyShiftTab)
	if selectedName(v) != "webapp" {
		t.Fatalf("Shift+Tab wraps: %s", selectedName(v))
	}
	// An arrow moves to the nearest node that way, and stays put with none there.
	from := v.sel
	moved := false
	for _, k := range []tea.KeyType{tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight} {
		w := pressV(v, k)
		if w.sel != from {
			moved = true
			// The chosen node lies in that direction.
			a, b := v.graph.nodes[from], w.graph.nodes[w.sel]
			switch k {
			case tea.KeyUp:
				if b.y >= a.y {
					t.Fatal("up went down")
				}
			case tea.KeyRight:
				if b.x <= a.x {
					t.Fatal("right went left")
				}
			}
		}
	}
	if !moved {
		t.Fatal("no arrow moved the selection")
	}
	v = keyV(v, "/")
	if v.find == nil {
		t.Fatal("/ opens find")
	}
	for _, r := range "the" {
		v = keyV(v, string(r))
	}
	if selectedName(v) != "thesis" {
		t.Fatalf("typing jumps: %s", selectedName(v))
	}
	if !strings.Contains(v.View(), "find: the") {
		t.Fatal(v.footer())
	}
	v = pressV(v, tea.KeyEsc)
	if v.find != nil || selectedName(v) != "webapp" {
		t.Fatalf("Esc puts the selection back: %s", selectedName(v))
	}
	v = keyV(v, "/")
	v = keyV(v, "n")
	v = pressV(v, tea.KeyEnter)
	if v.find != nil || selectedName(v) != "notes" {
		t.Fatalf("Enter keeps: %s", selectedName(v))
	}
}

func TestANudgeWakesTheMapAndItSettlesAgain(t *testing.T) {
	v := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "notes")
	x := v.graph.nodes[v.sel].x
	next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	v = next.(view)
	if v.graph.nodes[v.sel].x <= x || cmd == nil || !v.ticking {
		t.Fatal("Shift+Right moves the node and starts the ticks")
	}
	for i := 0; i < 2000 && v.ticking; i++ {
		next, cmd = v.Update(tickMsg(time.Now()))
		v = next.(view)
		if cmd == nil && v.ticking {
			t.Fatal("ticking without a next tick")
		}
	}
	if v.ticking {
		t.Fatal("the map never settled")
	}
	// A tick that arrives after the map settled changes nothing.
	next, cmd = v.Update(tickMsg(time.Now()))
	if cmd != nil {
		t.Fatal("a stray tick schedules another")
	}
}

func TestEnterOpensTheCardWithOnlyWhatExists(t *testing.T) {
	v := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "webapp")
	v = pressV(v, tea.KeyEnter)
	if v.panel != panelCard {
		t.Fatal("Enter opens the card")
	}
	screen := stripANSI(v.View())
	for _, want := range []string{"Path", "/code/webapp", "Mirrors", "thesis", "Mirrored by", "platform", "Described", "3 open: 1 plan", "[plan] Filter vehicle false alarms", "Alarm quality → Launch", "Touched", "touched today"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("the card lacks %q:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, "Last operation —") || strings.Contains(screen, "Hot topics") {
		t.Fatalf("rows with nothing to say are left out:\n%s", screen)
	}
	if lines := strings.Split(v.View(), "\n"); len(lines) != 30 {
		t.Fatalf("the card keeps the frame at %d lines", len(lines))
	}
	v = pressV(v, tea.KeyEnter)
	if v.panel != panelNone {
		t.Fatal("Enter again closes the card")
	}
	v = pressV(v, tea.KeyEnter, tea.KeyEsc)
	if v.panel != panelNone {
		t.Fatal("Esc closes the card")
	}
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !quits(cmd) {
		t.Fatal("Esc on the map quits")
	}
	// A project nobody described carries the signal.
	v = selectName(t, v, "firmware")
	v = pressV(v, tea.KeyEnter)
	if screen := stripANSI(v.View()); !strings.Contains(screen, "Signal") || !strings.Contains(screen, "not described") {
		t.Fatalf("signal:\n%s", screen)
	}
	v = pressV(v, tea.KeyEnter)
	// A problem's card says the fix.
	v = selectName(t, v, "gateway")
	v = pressV(v, tea.KeyEnter)
	if screen := stripANSI(v.View()); !strings.Contains(screen, "Reason") || !strings.Contains(screen, "Fix") || !strings.Contains(screen, "work in it again") {
		t.Fatalf("problem card:\n%s", screen)
	}
	// A card on a project nobody refreshed says to press R.
	items := []Item{{Entry: registry.Entry{ID: "id", Name: "fresh", Path: "/fresh"}}}
	v = pressV(sized(items, Opener{}, actions.Atlas{}), tea.KeyEnter)
	if !strings.Contains(stripANSI(v.View()), "not refreshed; press R") {
		t.Fatal(v.View())
	}
}

func TestHelpAndSettingsPanels(t *testing.T) {
	v := sized(sample(), Opener{}, actions.Atlas{})
	v = keyV(v, "?")
	screen := stripANSI(v.View())
	for _, want := range []string{"Keys", "Shift+arrows", "t ", "open a terminal", "? close"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("help lacks %q:\n%s", want, screen)
		}
	}
	v = keyV(v, "?")
	if v.panel != panelNone {
		t.Fatal("? again closes")
	}
	v = keyV(v, ",")
	if !v.settings || !strings.Contains(stripANSI(v.View()), "Settings") {
		t.Fatal(", opens settings")
	}
	v = keyV(v, "o")
	if v.busy != "" {
		t.Fatal("launch keys are quiet under settings")
	}
	v = pressV(v, tea.KeyEsc)
	if v.settings {
		t.Fatal("Esc closes settings")
	}
}

func TestOpenPutsTheProjectsFolderInObsidian(t *testing.T) {
	work := t.TempDir()
	_, err := project.Init(work, project.Options{Name: "webapp", NoGit: true}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	items := sample()
	items[1].Entry.Path = work
	opened := ""
	opener := Opener{Obsidian: func(path string) error { opened = path; return nil }}
	v := selectName(t, sized(items, opener, actions.Atlas{}), "webapp")
	next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	v = next.(view)
	if v.busy == "" || cmd == nil {
		t.Fatal("o opens in the background")
	}
	v = runCmd(v, cmd)
	if opened != filepath.Join(work, "atlas", "webapp") || v.busy != "" || !strings.Contains(v.status, "opened webapp in Obsidian") {
		t.Fatalf("opened=%q busy=%q status=%q", opened, v.busy, v.status)
	}
	failing := Opener{Obsidian: func(string) error { return errors.New("no Obsidian") }}
	v = selectName(t, sized(items, failing, actions.Atlas{}), "webapp")
	next, cmd = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	v = runCmd(next.(view), cmd)
	if v.errMsg != "no Obsidian" || !strings.Contains(v.View(), "no Obsidian") {
		t.Fatalf("the error reaches the footer: %q", v.errMsg)
	}
	none := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "webapp")
	if none = keyV(none, "o"); !strings.Contains(none.errMsg, "not available") {
		t.Fatalf("no opener: %q", none.errMsg)
	}
}

func TestTheHarnessStartsInTheWork(t *testing.T) {
	launched := ""
	opener := Opener{Agent: func(harness, path string) error { launched = harness + " " + path; return nil }}
	v := selectName(t, sized(sample(), opener, actions.Atlas{}), "webapp")
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd == nil || quits(cmd) {
		t.Fatal("c hands the terminal to the harness")
	}
	next, _ := v.Update(agentDoneMsg{name: "webapp", harness: "claude"})
	v = next.(view)
	if !strings.Contains(v.status, "back from Claude Code in webapp") {
		t.Fatalf("status %q", v.status)
	}
	_ = launched
}

func TestIDEAndTerminalKeysRunInTheBackground(t *testing.T) {
	for _, key := range []string{"i", "t"} {
		for _, failure := range []error{nil, errors.New("unavailable")} {
			opened := ""
			action := func(en registry.Entry) error { opened = en.Path; return failure }
			acts := actions.Atlas{}
			if key == "i" {
				acts.OpenIDE = action
			} else {
				acts.OpenTerminal = action
			}
			v := selectName(t, sized(sample(), Opener{}, acts), "webapp")
			next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			v = next.(view)
			if v.busy == "" || cmd == nil || opened != "" {
				t.Fatalf("%s launches in the background", key)
			}
			v = runCmd(v, cmd)
			if opened != "/code/webapp" || v.busy != "" {
				t.Fatalf("%s: opened=%q busy=%q", key, opened, v.busy)
			}
			if failure != nil && v.errMsg != "unavailable" {
				t.Fatalf("%s: %q", key, v.errMsg)
			}
			if failure == nil && !strings.Contains(v.status, "opened webapp in") {
				t.Fatalf("%s: %q", key, v.status)
			}
		}
		v := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "webapp")
		if v = keyV(v, key); !strings.Contains(v.errMsg, "not available") {
			t.Fatalf("%s without an action: %q", key, v.errMsg)
		}
	}
}

func TestKeysOnAProblemRefuse(t *testing.T) {
	v := selectName(t, sized(sample(), Opener{Obsidian: func(string) error { return nil }}, actions.Atlas{}), "gateway")
	for _, key := range []string{"o", "c", "i", "t", "n"} {
		w := keyV(v, key)
		if w.errMsg != missingError || w.busy != "" {
			t.Fatalf("%s on a problem: err=%q busy=%q", key, w.errMsg, w.busy)
		}
	}
	if !strings.Contains(v.hints(), "R refresh") || strings.Contains(v.hints(), "Obsidian") {
		t.Fatalf("hints %q", v.hints())
	}
}

func TestRefreshReloadsAndKeepsTheSelection(t *testing.T) {
	items := sample()
	refreshed := 0
	acts := actions.Atlas{
		Refresh: func() (*registry.Index, error) { refreshed++; return nil, nil },
		Load: func() ([]registry.Entry, error) {
			entries := entriesOf(items)
			entries = append(entries, proj("extra").Entry)
			return entries, nil
		},
	}
	v := selectName(t, sized(items, Opener{}, acts), "thesis")
	next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	v = next.(view)
	if v.busy == "" || cmd == nil {
		t.Fatal("R refreshes in the background")
	}
	v = runCmd(v, cmd)
	if refreshed != 1 || !v.changed || v.busy != "" || v.status != "refreshed" {
		t.Fatalf("refreshed=%d changed=%v busy=%q status=%q", refreshed, v.changed, v.busy, v.status)
	}
	if len(v.graph.nodes) != 7 || selectedName(v) != "thesis" {
		t.Fatalf("nodes %d selected %s", len(v.graph.nodes), selectedName(v))
	}
	none := sized(sample(), Opener{}, actions.Atlas{})
	if none = keyV(none, "R"); !strings.Contains(none.errMsg, "not available") {
		t.Fatal(none.errMsg)
	}
}

func TestNewThreadPromptsForOneLine(t *testing.T) {
	var got threads.New
	var into registry.Entry
	acts := actions.Atlas{StartThread: func(e registry.Entry, n threads.New) (*threads.Thread, error) {
		got, into = n, e
		return &threads.Thread{ID: "thr-20260917-0001", Title: "Fix the alarm"}, nil
	}}
	v := selectName(t, sized(sample(), Opener{}, acts), "webapp")
	v = keyV(v, "n")
	if v.stub == nil || !strings.Contains(v.View(), "new thread in webapp:") {
		t.Fatal("n opens the prompt")
	}
	// Keys go to the prompt, not the map.
	v = keyV(v, "q")
	if v.stub == nil {
		t.Fatal("q typed into the prompt")
	}
	v = pressV(v, tea.KeyEsc)
	if v.stub != nil || got.Text != "" {
		t.Fatal("Esc cancels")
	}
	v = keyV(v, "n")
	for _, r := range "Fix the alarm" {
		v = keyV(v, string(r))
	}
	v = pressV(v, tea.KeyEnter)
	if got.Text != "Fix the alarm" || into.Name != "webapp" || v.stub != nil {
		t.Fatalf("got %+v into %s", got, into.Name)
	}
	if !v.changed || !strings.Contains(v.status, "opened Fix the alarm in webapp") {
		t.Fatalf("status %q", v.status)
	}
	v = selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "webapp")
	if v = keyV(v, "n"); !strings.Contains(v.errMsg, "not available") {
		t.Fatal(v.errMsg)
	}
}

func TestNewThreadWritesARealStub(t *testing.T) {
	work := t.TempDir()
	at := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	res, err := project.Init(work, project.Options{Name: "webapp"}, at)
	if err != nil {
		t.Fatal(err)
	}
	p := res.Project
	acts := actions.Atlas{StartThread: func(e registry.Entry, n threads.New) (*threads.Thread, error) {
		return threads.Start(p, n, at)
	}}
	items := []Item{{Entry: registry.Entry{ID: "id", Name: "webapp", Path: work}}}
	v := sized(items, Opener{}, acts)
	v = keyV(v, "n")
	for _, r := range "Write the README" {
		v = keyV(v, string(r))
	}
	v = pressV(v, tea.KeyEnter)
	if v.errMsg != "" {
		t.Fatal(v.errMsg)
	}
	data, err := os.ReadFile(p.Path(project.StubsDir + "/Write the README.md"))
	if err != nil {
		t.Fatalf("the stub is written: %v", err)
	}
	if !strings.Contains(string(data), "type: stub") {
		t.Fatalf("stub:\n%s", data)
	}
}

func TestTheFooterNeverWraps(t *testing.T) {
	v := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "webapp")
	full := v.hints()
	if !strings.Contains(full, "t terminal") || !strings.Contains(full, "q quit") {
		t.Fatalf("hints %q", full)
	}
	for _, width := range []int{100, 70, 50, 30, 12} {
		next, _ := v.Update(tea.WindowSizeMsg{Width: width, Height: 16})
		w := next.(view)
		if lipgloss.Width(w.hints()) > width-2 && width > 20 {
			t.Fatalf("at %d the hints are %d wide: %q", width, lipgloss.Width(w.hints()), w.hints())
		}
		for i, line := range strings.Split(w.View(), "\n") {
			if lipgloss.Width(line) > width {
				t.Fatalf("at %d, line %d overflows: %q", width, i, line)
			}
		}
		if lines := strings.Split(w.View(), "\n"); len(lines) != 16 {
			t.Fatalf("at %d the frame is %d lines", width, len(lines))
		}
	}
	// The first hint survives at any width.
	next, _ := v.Update(tea.WindowSizeMsg{Width: 12, Height: 16})
	if w := next.(view); w.hints() != "Enter card" {
		t.Fatalf("narrowest %q", w.hints())
	}
}

func TestQuitAndCtrlC(t *testing.T) {
	v := sized(sample(), Opener{}, actions.Atlas{})
	if _, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); !quits(cmd) {
		t.Fatal("q quits")
	}
	if _, cmd := v.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); !quits(cmd) {
		t.Fatal("ctrl+c quits")
	}
	v = keyV(v, "?")
	if _, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); !quits(cmd) {
		t.Fatal("q quits from the help panel too")
	}
}
