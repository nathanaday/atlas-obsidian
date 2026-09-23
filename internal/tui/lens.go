package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

// lens is what the map shows about each project: the same nodes in the same places, with
// the colors, the card, and the summary of one aspect.
type lens int

const (
	lensDetails lens = iota
	lensThreads
	lensGit
	lensCount
)

func (l lens) String() string {
	switch l {
	case lensThreads:
		return "threads"
	case lensGit:
		return "version control"
	}
	return "details"
}

// next is the lens l cycles to.
func (l lens) next() lens { return (l + 1) % lensCount }

// tag is the lens's name in its own color, for the header.
func (l lens) tag() string {
	switch l {
	case lensThreads:
		return threadSt.Render(l.String())
	case lensGit:
		return gitSt.Render(l.String())
	}
	return memberSt.Render(l.String())
}

// openThreads is how many threads a project has open; 0 before a refresh.
func openThreads(e registry.Entry) int {
	if s := e.State; s != nil && s.Threads != nil {
		return s.Threads.Counts.Open
	}
	return 0
}

// gitOf is what git says about a project's work, or nil when it is no repository or no
// refresh ran.
func gitOf(e registry.Entry) *links.Link {
	if s := e.State; s != nil {
		return s.Git
	}
	return nil
}

// lensStyle is how a node that is neither selected nor a problem reads through the
// Threads and Version control lenses: its state's color, else gray.
func (v view) lensStyle(i int) lipgloss.Style {
	it := v.item(i)
	if it == nil {
		return fadedSt
	}
	switch v.lens {
	case lensThreads:
		if openThreads(it.Entry) > 0 {
			return threadSt
		}
	case lensGit:
		if g := gitOf(it.Entry); g != nil {
			switch {
			case g.Changed():
				return dirtySt
			case g.Diverged():
				return threadSt
			}
		}
	}
	return fadedSt
}

// diskRadius is the radius in dots of the disk under a project with n open threads: it
// grows with the square root, so the disk's area follows the count.
func diskRadius(n int) int {
	if n <= 0 {
		return 0
	}
	return min(12, int(math.Round(1.5+2*math.Sqrt(float64(n)))))
}

// disks draws, through the Threads lens, a disk under each project with open threads.
func (v view) disks(c *canvas, dots [][2]int) {
	if v.lens != lensThreads {
		return
	}
	for i := range v.graph.nodes {
		it := v.item(i)
		if it == nil {
			continue
		}
		if r := diskRadius(openThreads(it.Entry)); r > 0 {
			c.disk(dots[i][0], dots[i][1], r, 3)
		}
	}
}

// maxBar bounds a bar of the histogram, in cells.
const maxBar = 30

// threadStages are the stages of an open thread, in the order a thread moves.
var threadStages = []string{threads.Stub, threads.Spec, threads.Plan}

// stageCount is how many open threads are at a stage.
func stageCount(c threads.Counts, stage string) int {
	switch stage {
	case threads.Stub:
		return c.Stub
	case threads.Spec:
		return c.Spec
	case threads.Plan:
		return c.Plan
	}
	return 0
}

// threadCardLines is the body of a project's card through the Threads lens: the open
// count, a bar per stage, what is blocked or stale, the phases, and the open threads as a
// list the cursor walks. width is the card's content; rows is how many lines it may take.
func threadCardLines(e registry.Entry, cursor, width, rows int) []string {
	if e.Error != "" {
		return cardLines(e, nil, nil)
	}
	s := e.State
	switch {
	case s == nil:
		return []string{dim.Render("not refreshed; press R")}
	case !e.Threads:
		return []string{"threads are off", dim.Render("atlas-obsidian edit " + e.Name + " --threads on")}
	case s.Threads == nil:
		return []string{errSt.Render("the threads could not be read")}
	}
	sum := s.Threads
	c := sum.Counts
	var out []string
	if c.Open == 0 {
		out = append(out, "none open")
	} else {
		out = append(out, title.Render(fmt.Sprintf("%d open", c.Open)))
		most := 1
		for _, st := range threadStages {
			most = max(most, stageCount(c, st))
		}
		room := max(1, min(maxBar, width-labelWidth/2-4))
		for _, st := range threadStages {
			n := stageCount(c, st)
			bar := ""
			if n > 0 {
				bar = threadSt.Render(strings.Repeat("█", max(1, n*room/most)))
			}
			out = append(out, label.Width(labelWidth/2).Render(st)+fmt.Sprintf("%2d ", n)+bar)
		}
	}
	var flags []string
	if c.Blocked > 0 {
		flags = append(flags, errSt.Render(fmt.Sprintf("%d blocked", c.Blocked)))
	}
	if c.Stale > 0 {
		flags = append(flags, errSt.Render(fmt.Sprintf("%d stale", c.Stale)))
	}
	if c.Notes > 0 {
		flags = append(flags, fmt.Sprintf("%d note%s waiting", c.Notes, plural(c.Notes)))
	}
	if len(flags) > 0 {
		out = append(out, strings.Join(flags, dim.Render(" · ")))
	}
	if len(sum.Phases) > 0 {
		out = append(out, dim.Render("phases "+strings.Join(sum.Phases, " → ")))
	}
	if len(sum.Open) == 0 {
		return out
	}
	out = append(out, "")
	return append(out, threadList(sum.Open, cursor, width, rows-len(out))...)
}

// threadList is the open threads, two lines each, as many as rows holds around the
// cursor, with a line above and below that counts the rest. A line longer than width
// ends in an ellipsis.
func threadList(open []registry.ThreadLine, cursor, width, rows int) []string {
	fit := max(1, (rows-2)/2)
	start := max(0, min(cursor-fit+1, len(open)-fit))
	end := min(len(open), start+fit)
	var out []string
	if start > 0 {
		out = append(out, dim.Render(fmt.Sprintf("↑ %d more", start)))
	}
	for i := start; i < end; i++ {
		t := open[i]
		head := dim.Render("["+t.Stage+"] ") + t.Title
		if t.Blocked != "" {
			head += errSt.Render(" · blocked")
		}
		if t.Stale {
			head += errSt.Render(" · stale")
		}
		if i == cursor {
			head = litSt.Render("› ") + title.Render(stripANSI(head))
		} else {
			head = "  " + head
		}
		var sub []string
		if t.Phase != "" {
			sub = append(sub, t.Phase)
		}
		if t.Priority != "" && t.Priority != "normal" {
			sub = append(sub, t.Priority)
		}
		if t.Summary != "" {
			sub = append(sub, t.Summary)
		}
		if len(sub) == 0 {
			sub = append(sub, "updated "+t.Updated)
		}
		out = append(out, head, "    "+dim.Render(clipText(strings.Join(sub, " · "), width-4)))
	}
	if end < len(open) {
		out = append(out, dim.Render(fmt.Sprintf("↓ %d more", len(open)-end)))
	}
	return out
}

// clipText cuts plain text to width cells, ending in an ellipsis when it cut.
func clipText(s string, width int) string {
	r := []rune(s)
	if width < 2 || len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "…"
}

// gitCardLines is the body of a project's card through the Version control lens.
func gitCardLines(e registry.Entry) []string {
	if e.Error != "" {
		return cardLines(e, nil, nil)
	}
	if e.State == nil {
		return []string{dim.Render("not refreshed; press R")}
	}
	g := e.State.Git
	if g == nil {
		return []string{"not a git repository"}
	}
	var out []string
	row := func(k, val string) {
		if strings.TrimSpace(val) != "" {
			out = append(out, label.Width(labelWidth).Render(k)+val)
		}
	}
	row("Branch", g.Branch)
	if g.Head != "" {
		row("Head", g.Head+" "+dim.Render(g.Subject))
	}
	row("Upstream", upstreamText(*g))
	if g.Upstream != "" {
		fetched := "never"
		if g.Fetched != "" {
			fetched = g.Fetched
		}
		row("Fetched", fetched)
	}
	row("Remote", home.Display(g.Remote))
	switch {
	case g.Dirty == nil:
	case *g.Dirty == 0:
		row("Changes", okSt.Render("clean"))
	default:
		row("Changes", dirtySt.Render(fmt.Sprintf("%d uncommitted", *g.Dirty)))
	}
	row("Last commit", g.LastCommit)
	return out
}

// upstreamText is the branch's upstream and how far apart the two are.
func upstreamText(g links.Link) string {
	if g.Upstream == "" {
		return dim.Render("none")
	}
	text := g.Upstream
	switch {
	case g.Ahead == nil || g.Behind == nil:
	case !g.Diverged():
		text += " · " + okSt.Render("in step")
	default:
		if *g.Ahead > 0 {
			text += " · " + threadSt.Render(fmt.Sprintf("%d ahead", *g.Ahead))
		}
		if *g.Behind > 0 {
			text += " · " + threadSt.Render(fmt.Sprintf("%d behind", *g.Behind))
		}
	}
	return text
}

// lensFacts is what the summary line says about a project through the lens; nil for the
// Details lens, which says the general facts.
func lensFacts(l lens, e registry.Entry) []string {
	s := e.State
	switch l {
	case lensThreads:
		switch {
		case s == nil:
			return []string{"not refreshed; press R"}
		case !e.Threads:
			return []string{"threads off"}
		case s.Threads == nil:
			return []string{"threads unreadable"}
		}
		return []string{threadSummaryText(s.Threads)}
	case lensGit:
		switch {
		case s == nil:
			return []string{"not refreshed; press R"}
		case s.Git == nil:
			return []string{"no git"}
		}
		g := *s.Git
		out := []string{g.Branch}
		if g.Changed() {
			out = append(out, fmt.Sprintf("%d uncommitted", *g.Dirty))
		} else if g.Dirty != nil {
			out = append(out, "clean")
		}
		if g.Upstream != "" {
			out = append(out, stripANSI(upstreamText(g)))
		}
		return out
	}
	return nil
}
