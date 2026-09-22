package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/refresh"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
)

// labelWidth is the label column of a card.
const labelWidth = 15

// maxCardThreads bounds the open threads a card lists.
const maxCardThreads = 5

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// touchedText says when a project was last touched, or nothing before a refresh.
func touchedText(s *registry.State) string {
	switch {
	case s == nil || s.DaysIdle == nil:
		return ""
	case *s.DaysIdle == 0:
		return "touched today"
	default:
		return fmt.Sprintf("idle %d day%s", *s.DaysIdle, plural(*s.DaysIdle))
	}
}

// facts is what a project's state says in a few words: pages, open threads, the inbox,
// and when it was touched. A zero is left out; a project nobody refreshed says so.
func facts(s *registry.State) []string {
	if s == nil {
		return []string{"not refreshed; press R"}
	}
	var out []string
	if s.Pages != nil {
		out = append(out, fmt.Sprintf("%d page%s", *s.Pages, plural(*s.Pages)))
	}
	if s.Threads != nil && s.Threads.Counts.Open > 0 {
		n := s.Threads.Counts.Open
		text := fmt.Sprintf("%d thread%s open", n, plural(n))
		if s.Threads.Counts.Blocked > 0 {
			text += fmt.Sprintf(", %d blocked", s.Threads.Counts.Blocked)
		}
		out = append(out, text)
	}
	if s.Inbox != nil && *s.Inbox > 0 {
		out = append(out, fmt.Sprintf("%d in inbox", *s.Inbox))
	}
	if t := touchedText(s); t != "" {
		out = append(out, t)
	}
	return out
}

// threadSummaryText is one line of thread counts.
func threadSummaryText(t *registry.ThreadSummary) string {
	c := t.Counts
	if c.Open == 0 {
		if c.Notes > 0 {
			return fmt.Sprintf("none open · %d note%s waiting", c.Notes, plural(c.Notes))
		}
		return "none open"
	}
	out := fmt.Sprintf("%d open: %d plan · %d spec · %d stub", c.Open, c.Plan, c.Spec, c.Stub)
	if c.Blocked > 0 {
		out += fmt.Sprintf(" · %d blocked", c.Blocked)
	}
	if c.Stale > 0 {
		out += errSt.Render(fmt.Sprintf(" · %d stale", c.Stale))
	}
	if c.Notes > 0 {
		out += fmt.Sprintf(" · %d note%s waiting", c.Notes, plural(c.Notes))
	}
	return out
}

// problemFix says what puts an entry the scan could not read right.
func problemFix(e registry.Entry) string {
	path := home.Display(e.Path)
	switch e.Reason {
	case registry.ReasonMissing:
		return strings.TrimPrefix(e.Error, "not found; ")
	case registry.ReasonNotProject:
		return "run atlas-obsidian init " + path + ", or atlas-obsidian forget " + path
	case registry.ReasonUnreadable:
		return "repair the identity file and press R"
	case registry.ReasonSchema:
		return "written by a newer atlas-obsidian; update the binary"
	}
	return e.Error
}

// cardLines is the body of a project's card: only the rows that have something to say.
// names turns a project id into its name.
func cardLines(e registry.Entry, members, hubs []string) []string {
	var out []string
	row := func(k, val string) {
		if strings.TrimSpace(val) == "" {
			return
		}
		out = append(out, label.Width(labelWidth).Render(k)+val)
	}
	if e.Error != "" {
		row("Path", home.Display(e.Path))
		row("Reason", e.Reason)
		row("Fix", problemFix(e))
		return out
	}
	row("Path", home.Display(e.Path))
	row("Description", e.Description)
	row("Mirrors", strings.Join(members, ", "))
	row("Mirrored by", strings.Join(hubs, ", "))
	s := e.State
	if s == nil {
		row("State", dim.Render("not refreshed; press R"))
		return out
	}
	if g := s.Git; g != nil {
		row("Git", refresh.LinkSummary(*g))
	}
	if d := s.Described; d != nil {
		row("Described", d.Summary())
	}
	if s.Pages != nil {
		wiki := fmt.Sprintf("%d page%s", *s.Pages, plural(*s.Pages))
		if u := s.Unfinished.Text(); u != "" {
			wiki += " · " + u
		}
		row("Wiki", wiki)
	}
	row("Last operation", s.LastOperation)
	if s.Inbox != nil && *s.Inbox > 0 {
		row("Inbox", fmt.Sprintf("%d waiting", *s.Inbox))
	}
	if s.Threads != nil {
		row("Threads", threadSummaryText(s.Threads))
		if len(s.Threads.Phases) > 0 {
			row("Phases", strings.Join(s.Threads.Phases, " → "))
		}
		for i, t := range s.Threads.Open {
			if i == maxCardThreads {
				out = append(out, label.Width(labelWidth).Render("")+dim.Render(fmt.Sprintf("… and %d more", len(s.Threads.Open)-i)))
				break
			}
			k := ""
			if i == 0 {
				k = "Open"
			}
			line := fmt.Sprintf("[%s] %s", t.Stage, t.Title)
			if t.Phase != "" {
				line += dim.Render(" · " + t.Phase)
			}
			if t.Blocked != "" {
				line += errSt.Render(" · blocked")
			}
			if t.Stale {
				line += errSt.Render(" · stale")
			}
			out = append(out, label.Width(labelWidth).Render(k)+line)
		}
	}
	row("Touched", touchedText(s))
	for _, note := range refresh.Signals(e, now()) {
		out = append(out, label.Width(labelWidth).Render("Signal")+errSt.Render(note))
	}
	return out
}

// panel renders a titled box of lines, each clipped to the box, at most tall lines high.
// width is the box with its border; the content is four narrower.
func panel(titleText string, lines []string, width, tall int) []string {
	content := max(10, width-4)
	fit := lipgloss.NewStyle().MaxWidth(content)
	pad := lipgloss.NewStyle().Width(content)
	body := []string{title.Render(titleText), ""}
	for _, line := range lines {
		body = append(body, pad.Render(fit.Render(line)))
	}
	if tall > 0 && len(body) > tall-2 {
		body = append(body[:max(1, tall-3)], dim.Render("…"))
	}
	return strings.Split(panelSt.Render(strings.Join(body, "\n")), "\n")
}
