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

func proj(name, heat string) Item {
	four, one := 4, 1
	return Item{Entry: registry.Entry{
		ID: "id-" + name, Name: name, Path: "/code/" + name, Mode: project.Generic,
		Created: "2026-09-01", Description: "the " + name + " work",
		State: &registry.State{OK: true, Heat: heat, Pages: &four, Inbox: &one,
			GeneratedAt: "2026-09-12T18:00:00Z", HotTopics: []string{"thread"}, LastOperation: "2026-09-10"},
	}}
}

// sample is three projects and one folder the scan could not read.
func sample() []Item {
	items := []Item{proj("webapp", "hot"), proj("firmware", "cold"), proj("thesis", "new")}
	zero := 0
	items[0].Entry.State.DaysIdle = &zero
	items[0].Entry.State.Threads = &registry.ThreadSummary{
		Counts: threads.Counts{Open: 3, Plan: 1, Spec: 2, Phases: 1},
		Open:   []registry.ThreadLine{{ID: "thr-20260917-0001", Title: "Filter vehicle false alarms", Stage: "plan", Priority: "high", Phase: "Alarm quality"}},
		Phases: []string{"Alarm quality", "Launch"},
	}
	items[0].Entry.State.Described = &registry.Description{Page: "wiki/entities/webapp.md", Commit: "abc1234", Behind: 2}
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

// findEntry puts the cursor on the entry with that name, on its tab.
func findEntry(t *testing.T, v view, name string) view {
	t.Helper()
	for i := range v.boards {
		for j, it := range v.boards[i].items {
			if entryName(it.Entry) == name {
				v.tab = boardTab(i)
				v.boards[i].cursor = j
				v.boards[i].layout()
				return v
			}
		}
	}
	t.Fatalf("no box for %s", name)
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

func TestTheScreenIsOneListOfProjects(t *testing.T) {
	v := newView(sample(), Opener{}, actions.Atlas{})
	out := v.View()
	t.Logf("\n%s", out)
	for _, want := range []string{"Atlas   Projects (3)  Problems (1)", "A project is an atlas/<name>/ folder", "refreshed 2026-09-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if v.tab != tabProjects || len(v.boards[0].rows) != 3 {
		t.Fatalf("the Projects tab lists every project: tab=%d rows=%d", v.tab, len(v.boards[0].rows))
	}
	// One box shows both halves: the wiki's pages and the threads.
	for _, want := range []string{"webapp", "the webapp work", "/code/webapp", "4 pages", "3 threads open", "phase: Alarm quality", "1 in inbox", "🔥", "❄️", "✨"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	names := []string{}
	for _, it := range v.boards[0].items {
		names = append(names, it.Entry.Name)
	}
	if got := strings.Join(names, ","); got != "firmware,thesis,webapp" {
		t.Fatalf("by name: %s", got)
	}
	v = pressV(v, tea.KeyRight)
	if v.tab != tabProblems || !strings.Contains(v.View(), "could not read") {
		t.Fatalf("problems: tab=%d", v.tab)
	}
	v = pressV(v, tea.KeyRight)
	if v.tab != tabConfig {
		t.Fatal("the Config tab follows Problems")
	}
	v = pressV(v, tea.KeyLeft, tea.KeyLeft)
	if v.tab != tabProjects {
		t.Fatalf("left stops at Projects: tab=%d", v.tab)
	}
	clean := newView(sample()[:3], Opener{}, actions.Atlas{})
	if strings.Contains(clean.View(), "Problems") || len(clean.tabs()) != 2 {
		t.Fatal("no problems, no Problems tab")
	}
}

func TestEnterExpandsInPlace(t *testing.T) {
	v := findEntry(t, newView(sample(), Opener{}, actions.Atlas{}), "webapp")
	v = pressV(v, tea.KeyEnter)
	out := v.View()
	t.Logf("\n%s", out)
	for _, want := range []string{
		"Path", "/code/webapp", "Mode", "generic", "Description", "the webapp work",
		"Described", "described in wiki/entities/webapp.md at abc1234, 2 commits behind",
		"Wiki", "4 pages", "Last operation", "2026-09-10", "Hot topics", "- thread",
		"Threads", "3 open: 1 plan", "Phases", "Alarm quality → Launch",
		"[plan] Filter vehicle false alarms", "Enter collapse",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	v = pressV(v, tea.KeyEnter)
	if out := v.View(); strings.Contains(out, "Described") || !strings.Contains(out, "Enter details") {
		t.Fatalf("Enter again collapses:\n%s", out)
	}
	v = findEntry(t, v, "thesis")
	v = pressV(v, tea.KeyEnter)
	if out := v.View(); !strings.Contains(out, registry.NotDescribed) {
		t.Fatalf("a project with no page says so:\n%s", out)
	}
	v = findEntry(t, v, "gateway")
	v = pressV(v, tea.KeyEnter)
	if out := v.View(); !strings.Contains(out, "Reason") || !strings.Contains(out, "missing") || !strings.Contains(out, "heal the path") {
		t.Fatalf("a problem explains itself:\n%s", out)
	}
}

func TestEscCollapsesThenQuits(t *testing.T) {
	v := findEntry(t, newView(sample(), Opener{}, actions.Atlas{}), "webapp")
	v = pressV(v, tea.KeyEnter)
	next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	v = next.(view)
	if quits(cmd) || len(v.board().expanded) != 0 {
		t.Fatal("the first Esc collapses")
	}
	_, cmd = v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !quits(cmd) {
		t.Fatal("the second Esc quits")
	}
	_, cmd = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if !quits(cmd) {
		t.Fatal("q quits")
	}
}

func TestCursorMovesAndStopsAtTheEndMarker(t *testing.T) {
	v := newView(sample(), Opener{}, actions.Atlas{})
	v = pressV(v, tea.KeyDown, tea.KeyDown, tea.KeyDown, tea.KeyDown)
	if !v.board().atEnd() || v.current() != nil {
		t.Fatalf("four downs land on the end marker: cursor=%d", v.board().cursor)
	}
	if !strings.Contains(v.View(), "(end)") {
		t.Fatal("the end marker shows")
	}
	v = pressV(v, tea.KeyUp)
	if it := v.current(); it == nil || it.Entry.Name != "webapp" {
		t.Fatal("up from the end lands on the last project")
	}
}

func TestOpenPutsTheProjectsFolderInObsidian(t *testing.T) {
	work := t.TempDir()
	_, err := project.Init(work, project.Options{Name: "webapp", NoGit: true}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	items := sample()
	items[0].Entry.Path = work
	opened := ""
	opener := Opener{Obsidian: func(path string) error { opened = path; return nil }}
	v := findEntry(t, newView(items, opener, actions.Atlas{}), "webapp")
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
	v = findEntry(t, newView(items, failing, actions.Atlas{}), "webapp")
	next, cmd = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	v = runCmd(next.(view), cmd)
	if v.errMsg != "no Obsidian" {
		t.Fatalf("the error reaches the footer: %q", v.errMsg)
	}
	items[0].Entry.Path = t.TempDir()
	opened = ""
	v = findEntry(t, newView(items, opener, actions.Atlas{}), "webapp")
	next, cmd = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	v = runCmd(next.(view), cmd)
	if opened != "" || v.errMsg == "" || v.busy != "" {
		t.Fatalf("missing project: opened=%q error=%q busy=%q", opened, v.errMsg, v.busy)
	}
	none := findEntry(t, newView(sample(), Opener{}, actions.Atlas{}), "webapp")
	if none = keyV(none, "o"); !strings.Contains(none.errMsg, "not available") {
		t.Fatalf("no opener: %q", none.errMsg)
	}
}

func TestClaudeStartsInTheWork(t *testing.T) {
	launched := ""
	opener := Opener{Agent: func(harness, path string) error { launched = path; return nil }}
	v := findEntry(t, newView(sample(), opener, actions.Atlas{}), "webapp")
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd == nil {
		t.Fatal("c hands the terminal to Claude Code")
	}
	if _, ok := cmd().(tea.QuitMsg); ok {
		t.Fatal("c does not quit")
	}
	next, refreshCmd := v.Update(agentDoneMsg{name: "webapp"})
	v = next.(view)
	if !strings.Contains(v.errMsg+v.status, "back from Claude Code in webapp") && refreshCmd != nil {
		t.Fatalf("status=%q err=%q", v.status, v.errMsg)
	}
	if launched != "" {
		t.Fatalf("the launch runs only when Bubble Tea executes it: %q", launched)
	}
	if l := (launch{run: func() error { launched = "ran"; return nil }}); l.Run() != nil || launched != "ran" {
		t.Fatal("the launch adapter runs the opener")
	}
	none := findEntry(t, newView(sample(), Opener{}, actions.Atlas{}), "webapp")
	if none = keyV(none, "c"); !strings.Contains(none.errMsg, "not available") {
		t.Fatalf("no opener: %q", none.errMsg)
	}
}

func TestKeysOnAProblemRefuse(t *testing.T) {
	v := findEntry(t, newView(sample(), Opener{}, actions.Atlas{}), "gateway")
	for _, key := range []string{"o", "c", "i", "n"} {
		v = keyV(v, key)
		if !strings.Contains(v.errMsg, "not found") {
			t.Fatalf("%s on a problem names the error: %q", key, v.errMsg)
		}
	}
}

func TestRefreshReloadsAndKeepsTheCursor(t *testing.T) {
	refreshed := 0
	acts := actions.Atlas{
		Load:    func() ([]registry.Entry, error) { return entriesOf(sample()), nil },
		Refresh: func() (*registry.Index, error) { refreshed++; return &registry.Index{}, nil },
	}
	v := findEntry(t, newView(sample(), Opener{}, acts), "firmware")
	next, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	v = next.(view)
	if v.busy == "" || cmd == nil {
		t.Fatal("R refreshes in the background")
	}
	v = runCmd(v, cmd)
	if refreshed != 1 || v.busy != "" || v.status != "refreshed" || !v.changed {
		t.Fatalf("refreshed=%d busy=%q status=%q changed=%v", refreshed, v.busy, v.status, v.changed)
	}
	if it := v.current(); it == nil || it.Entry.Name != "firmware" {
		t.Fatal("the cursor stays on the same project")
	}
	none := keyV(newView(sample(), Opener{}, actions.Atlas{}), "R")
	if !strings.Contains(none.errMsg, "not available") {
		t.Fatalf("no refresh action: %q", none.errMsg)
	}
}

func TestNewThreadPromptsForOneLine(t *testing.T) {
	var got threads.New
	var into string
	acts := actions.Atlas{
		StartThread: func(e registry.Entry, n threads.New) (*threads.Thread, error) {
			got, into = n, e.Name
			return &threads.Thread{ID: "thr-20260917-abcd", Title: threads.TitleFromText(n.Text)}, nil
		},
	}
	v := findEntry(t, newView(sample(), Opener{}, acts), "webapp")
	v = keyV(v, "n")
	if v.stub == nil || !strings.Contains(v.View(), "new thread in webapp:") || !strings.Contains(v.View(), "Enter open the thread") {
		t.Fatalf("n opens the prompt:\n%s", v.View())
	}
	for _, r := range "fix the login page" {
		v = keyV(v, string(r))
	}
	v = keyV(v, "q") // q is text while the prompt is open
	v = pressV(v, tea.KeyBackspace, tea.KeyEnter)
	if v.stub != nil || into != "webapp" || got.Text != "fix the login page" {
		t.Fatalf("Enter opens the thread: into=%q text=%q", into, got.Text)
	}
	if !v.changed || !strings.Contains(v.status, "opened fix the login page in webapp (thr-20260917-abcd)") {
		t.Fatalf("status=%q changed=%v", v.status, v.changed)
	}
	v = keyV(v, "n")
	v = pressV(v, tea.KeyEsc)
	if v.stub != nil {
		t.Fatal("Esc cancels the prompt")
	}
	v = keyV(v, "n")
	v = pressV(v, tea.KeyEnter)
	if v.stub != nil || into != "webapp" || v.errMsg != "" {
		t.Fatal("an empty line opens nothing and says nothing")
	}
	failing := actions.Atlas{StartThread: func(registry.Entry, threads.New) (*threads.Thread, error) {
		return nil, errors.New("no phase named x")
	}}
	v = findEntry(t, newView(sample(), Opener{}, failing), "webapp")
	v = keyV(v, "n")
	v = keyV(v, "x")
	v = pressV(v, tea.KeyEnter)
	if v.errMsg != "no phase named x" {
		t.Fatalf("an error reaches the footer: %q", v.errMsg)
	}
	none := findEntry(t, newView(sample(), Opener{}, actions.Atlas{}), "webapp")
	if none = keyV(none, "n"); none.stub != nil || !strings.Contains(none.errMsg, "not available") {
		t.Fatalf("no action: %q", none.errMsg)
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
	v := findEntry(t, newView(items, Opener{}, acts), "webapp")
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
	for _, rel := range []string{project.ThreadsDir + "/Write the README.md", project.ThreadsIndex} {
		if _, err := os.Stat(p.Path(rel)); err != nil {
			t.Fatalf("%s is generated: %v", rel, err)
		}
	}
}

func TestHelpTogglesTheFooter(t *testing.T) {
	v := findEntry(t, newView(sample(), Opener{}, actions.Atlas{}), "webapp")
	out := v.View()
	if !strings.Contains(out, "Enter details · o Obsidian · c Claude Code · i VS Code · n new thread") {
		t.Fatalf("help off names the row's keys:\n%s", out)
	}
	v = keyV(v, "h")
	out = v.View()
	if !strings.Contains(out, "↑↓ move") || !strings.Contains(out, "←→ tabs · R refresh · h hide help · q quit") {
		t.Fatalf("help on names every key:\n%s", out)
	}
}

// Every frame is exactly as tall as the screen and no wider, so the terminal never scrolls
// the tab bar out of sight on one tab and leaves it in place on another.
func TestEveryTabFillsTheScreenExactly(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{{Width: 120, Height: 40}, {Width: 80, Height: 24}, {Width: 60, Height: 12}} {
		v := newView(sample(), Opener{}, actions.Atlas{})
		next, _ := v.Update(size)
		v = next.(view)
		for _, name := range []string{"Projects", "Problems"} {
			v = pressV(v, tea.KeyEnter)
			out := v.View()
			if got := strings.Count(out, "\n") + 1; got != size.Height {
				t.Errorf("%s at %dx%d is %d lines, want %d:\n%s", name, size.Width, size.Height, got, size.Height, out)
			}
			for _, line := range strings.Split(out, "\n") {
				if w := lipgloss.Width(line); w > size.Width {
					t.Errorf("%s at %dx%d: %q is %d columns, want at most %d", name, size.Width, size.Height, line, w, size.Width)
				}
			}
			v = pressV(v, tea.KeyRight)
		}
	}
}

func TestAnEmptyScreenSaysWhatToRun(t *testing.T) {
	v := newView(nil, Opener{}, actions.Atlas{})
	if out := v.View(); !strings.Contains(out, "atlas-obsidian init") {
		t.Fatalf("empty:\n%s", out)
	}
}

func TestNarrowBarDropsTheStampThenTheCounts(t *testing.T) {
	v := newView(sample(), Opener{}, actions.Atlas{})
	v.width = 120
	if !strings.Contains(v.tabBar(), "refreshed 2026") {
		t.Fatal("a wide screen shows the stamp")
	}
	v.width = 40
	if bar := v.tabBar(); strings.Contains(bar, "refreshed") || !strings.Contains(bar, "(3)") {
		t.Fatalf("a narrower screen keeps the counts: %q", bar)
	}
	v.width = 22
	if bar := v.tabBar(); strings.Contains(bar, "(3)") {
		t.Fatalf("a narrow screen drops the counts: %q", bar)
	}
}
