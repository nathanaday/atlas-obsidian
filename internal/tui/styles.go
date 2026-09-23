package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The palette: the project color for what is selected and what it mirrors, a second
// color for what mirrors it, red for what is wrong, yellow for what waits on the user
// through a lens, green for the version control lens, and two grays for what is not in
// focus.
const (
	projectColor = lipgloss.Color("12")
	hubColor     = lipgloss.Color("13")
	problemColor = lipgloss.Color("9")
	waitColor    = lipgloss.Color("11")
	gitColor     = lipgloss.Color("10")
	mutedColor   = lipgloss.Color("245")
	faintColor   = lipgloss.Color("240")
	labelColor   = lipgloss.Color("#FFC600")
)

var (
	title   = lipgloss.NewStyle().Bold(true)
	label   = lipgloss.NewStyle().Foreground(labelColor)
	dim     = lipgloss.NewStyle().Foreground(mutedColor)
	faint   = lipgloss.NewStyle().Foreground(faintColor)
	errSt   = lipgloss.NewStyle().Foreground(problemColor)
	okSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	litSt   = lipgloss.NewStyle().Foreground(projectColor)
	panelSt = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(projectColor).Padding(0, 1)
	// The node labels: the selected one is filled, its members and its hubs wear their
	// colors, a problem is red, and the rest fade while something is selected.
	selectedSt = lipgloss.NewStyle().Background(projectColor).Foreground(lipgloss.Color("15")).Bold(true)
	memberSt   = lipgloss.NewStyle().Foreground(projectColor).Bold(true)
	hubSt      = lipgloss.NewStyle().Foreground(hubColor).Bold(true)
	problemSt  = lipgloss.NewStyle().Foreground(problemColor)
	nodeSt     = lipgloss.NewStyle()
	fadedSt    = lipgloss.NewStyle().Foreground(mutedColor)
	// Through a lens: open threads and a branch apart from its upstream are yellow,
	// uncommitted changes red.
	threadSt = lipgloss.NewStyle().Foreground(waitColor).Bold(true)
	dirtySt  = lipgloss.NewStyle().Foreground(problemColor).Bold(true)
	gitSt    = lipgloss.NewStyle().Foreground(gitColor).Bold(true)
	diskSt   = lipgloss.NewStyle().Foreground(waitColor)
)

// stripANSI drops escape sequences: it measures styled text and renders a row plain.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case inEsc:
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
		case r == 0x1b:
			inEsc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
