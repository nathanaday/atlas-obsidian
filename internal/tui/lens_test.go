package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nathanaday/atlas-obsidian/internal/actions"
	"github.com/nathanaday/atlas-obsidian/internal/claudecode"
)

// nodeAt is the index of a project on the map on screen.
func nodeAt(t *testing.T, v view, name string) int {
	t.Helper()
	for i, n := range v.graph.nodes {
		if n.name == name {
			return i
		}
	}
	t.Fatalf("no %s on the map", name)
	return -1
}

func sameStyle(a, b lipgloss.Style) bool { return a.Render("x") == b.Render("x") }

func TestLCyclesTheLenses(t *testing.T) {
	v := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "webapp")
	for _, want := range []lens{lensThreads, lensGit, lensDetails} {
		v = keyV(v, "l")
		if v.lens != want || !strings.Contains(stripANSI(v.header()), "· "+want.String()) {
			t.Fatalf("lens %v, header %q", v.lens, stripANSI(v.header()))
		}
	}
	// A lens keeps the map: the same nodes in the same places.
	before := v.View()
	v = keyV(v, "l")
	v = keyV(v, "l")
	v = keyV(v, "l")
	if v.View() != before {
		t.Fatal("a full turn of the lenses changed the map")
	}
}

func TestTheThreadsLensColorsAndSizes(t *testing.T) {
	withColor(t)
	v := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "thesis")
	v = keyV(v, "l")
	if !sameStyle(v.nodeStyle(nodeAt(t, v, "webapp")), threadSt) {
		t.Fatal("a project with open threads is yellow")
	}
	if !sameStyle(v.nodeStyle(nodeAt(t, v, "firmware")), fadedSt) {
		t.Fatal("a project with none is gray")
	}
	if !sameStyle(v.nodeStyle(v.sel), selectedSt) {
		t.Fatal("the selection stays filled")
	}
	if !strings.Contains(v.View(), diskSt.Render("⣿")) {
		t.Fatal("a project with open threads sits on a disk")
	}
	if diskRadius(0) != 0 || diskRadius(1) >= diskRadius(4) || diskRadius(4) >= diskRadius(9) || diskRadius(1000) != 12 {
		t.Fatal("the disk grows with the count, within a bound")
	}
}

func TestTheThreadsCard(t *testing.T) {
	v := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "webapp")
	v = keyV(v, "l")
	v = pressV(v, tea.KeyEnter)
	if !v.threadCard() {
		t.Fatal("Enter opens the threads card")
	}
	screen := stripANSI(v.View())
	for _, want := range []string{"webapp · threads", "3 open", "stub    0", "spec    2 ███", "plan    1 █", "1 blocked", "phases Alarm quality → Launch",
		"› [plan] Filter vehicle false alarms", "Alarm quality · high", "[spec] Night mode", "The map is too bright after dark.", "Export to CSV · blocked"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("the card lacks %q:\n%s", want, screen)
		}
	}
	if !strings.Contains(v.hints(), "↑↓ thread") || !strings.Contains(v.hints(), "p plant") || !strings.Contains(v.hints(), "c ask") {
		t.Fatalf("hints %q", v.hints())
	}
	sel := v.sel
	v = pressV(v, tea.KeyDown, tea.KeyDown, tea.KeyDown, tea.KeyDown)
	if v.cursor != 2 || v.sel != sel {
		t.Fatalf("↓ walks the list and stops at its end: cursor %d", v.cursor)
	}
	if !strings.Contains(stripANSI(v.View()), "› [spec] Export to CSV") {
		t.Fatal("the cursor shows")
	}
	if in := v.intent(); !in.Ask || in.Thread != "thr-20260917-0003" {
		t.Fatalf("c asks about the thread under the cursor: %+v", in)
	}
	// A refresh that closed threads keeps the cursor on the list.
	items := sample()
	items[1].Entry.State.Threads.Open = items[1].Entry.State.Threads.Open[:1]
	fewer := v
	fewer.take(items, "id-webapp")
	if fewer.cursor != 0 || !fewer.threadCard() {
		t.Fatalf("after a refresh the cursor is %d", fewer.cursor)
	}
	v = pressV(v, tea.KeyUp, tea.KeyUp, tea.KeyUp)
	if v.cursor != 0 {
		t.Fatalf("↑ stops at the top: %d", v.cursor)
	}
	v = pressV(v, tea.KeyDown, tea.KeyRight)
	if v.sel == sel || v.cursor != 0 {
		t.Fatal("→ moves to the next project and the cursor starts over")
	}
	v = pressV(v, tea.KeyEnter)
	if in := v.intent(); in != (claudecode.Intent{}) {
		t.Fatalf("with the card closed, c starts a plain session: %+v", in)
	}
}

func TestTheThreadListScrolls(t *testing.T) {
	items := sample()
	sum := items[1].Entry.State.Threads
	for len(sum.Open) < 20 {
		sum.Open = append(sum.Open, sum.Open[1])
	}
	lines := threadList(sum.Open, 0, 60, 10)
	if len(lines) > 10 || !strings.Contains(stripANSI(strings.Join(lines, "\n")), "↓ 16 more") {
		t.Fatalf("top:\n%s", strings.Join(lines, "\n"))
	}
	lines = threadList(sum.Open, 19, 60, 10)
	text := stripANSI(strings.Join(lines, "\n"))
	if len(lines) > 10 || !strings.Contains(text, "↑ 16 more") || strings.Contains(text, "↓") || !strings.Contains(text, "› ") {
		t.Fatalf("bottom:\n%s", text)
	}
}

func TestPlantAndThreadsOff(t *testing.T) {
	opener := Opener{Agent: func(string, string, claudecode.Intent) error { return nil }}
	v := selectName(t, sized(sample(), opener, actions.Atlas{}), "webapp")
	v = keyV(v, "p")
	if v.errMsg != "" {
		t.Fatal("p does nothing outside the threads card")
	}
	v = keyV(v, "l")
	v = pressV(v, tea.KeyEnter)
	if _, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}); cmd == nil {
		t.Fatal("p on the threads card hands the terminal to the harness")
	}
	v = selectName(t, v, "thesis")
	if screen := stripANSI(v.View()); !strings.Contains(screen, "threads are off") || !strings.Contains(screen, "--threads on") {
		t.Fatalf("a project with threads off says so:\n%s", screen)
	}
	v = keyV(v, "p")
	if !strings.Contains(v.errMsg, "threads are off") {
		t.Fatalf("p refuses: %q", v.errMsg)
	}
}

func TestTheVersionControlLens(t *testing.T) {
	withColor(t)
	v := selectName(t, sized(sample(), Opener{}, actions.Atlas{}), "platform")
	v = keyV(v, "l")
	v = keyV(v, "l")
	v = pressV(v, tea.KeyEnter) // opens the cluster: Enter keeps its rule
	for name, want := range map[string]lipgloss.Style{"webapp": dirtySt, "firmware": threadSt, "thesis": fadedSt} {
		if !sameStyle(v.nodeStyle(nodeAt(t, v, name)), want) {
			t.Fatalf("%s wears the wrong color", name)
		}
	}
	v = selectName(t, v, "firmware")
	v = pressV(v, tea.KeyEnter)
	screen := stripANSI(v.View())
	for _, want := range []string{"firmware · version control", "Branch", "Upstream       origin/main · 1 ahead", "Fetched        2026-09-20", "git@example.com:a/firmware.git", "Changes        clean"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("the card lacks %q:\n%s", want, screen)
		}
	}
	if in := v.intent(); !in.Git || !strings.Contains(v.hints(), "c git") {
		t.Fatalf("c handles the git state: %+v %q", in, v.hints())
	}
	if !strings.Contains(stripANSI(v.summary()), "origin/main · 1 ahead") {
		t.Fatalf("summary %q", stripANSI(v.summary()))
	}
	v = selectName(t, v, "notes") // on the overview: leaving the cluster closed the card
	v = pressV(v, tea.KeyEnter)
	if screen := stripANSI(v.View()); !strings.Contains(screen, "not a git repository") {
		t.Fatalf("no git:\n%s", screen)
	}
	if !sameStyle(v.nodeStyle(nodeAt(t, v, "notes")), selectedSt) {
		t.Fatal("the selection stays filled")
	}
}
