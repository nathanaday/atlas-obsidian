package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestCanvasDrawsBrailleAndLaysTextOver(t *testing.T) {
	withColor(t)
	c := newCanvas(6, 2)
	c.line(0, 0, 11, 0, 1)
	c.line(0, 4, 11, 4, 2)
	c.place(2, 0, "ab")
	plain := lipgloss.NewStyle()
	rows := c.render(plain, plain)
	if len(rows) != 2 {
		t.Fatalf("rows %d", len(rows))
	}
	if rows[0] != "⠉⠉ab⠉⠉" {
		t.Fatalf("row 0 %q", rows[0])
	}
	if rows[1] != "⠉⠉⠉⠉⠉⠉" {
		t.Fatalf("row 1 %q", rows[1])
	}
	for _, row := range rows {
		if lipgloss.Width(row) != 6 {
			t.Fatalf("row %q is %d wide", row, lipgloss.Width(row))
		}
	}
	// A lit cell is styled by the lit style, a dim one by the dim style.
	lit := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styled := c.render(plain, lit)
	if strings.Contains(styled[0], "\x1b") || !strings.Contains(styled[1], "\x1b") {
		t.Fatal("layers")
	}
}

func TestCanvasCutsTextAtTheEdgeAndKeepsWidth(t *testing.T) {
	c := newCanvas(5, 1)
	c.place(3, 0, "toolong")
	c.place(-2, 0, "x")
	c.place(0, 5, "off")
	rows := c.render(lipgloss.NewStyle(), lipgloss.NewStyle())
	if rows[0] != "x  to" || lipgloss.Width(rows[0]) != 5 {
		t.Fatalf("row %q", rows[0])
	}
	if !c.free(1, 0, 2) || c.free(2, 0, 2) {
		t.Fatal("free")
	}
}

func TestCanvasOverlappingRunsShowTheLaterOne(t *testing.T) {
	c := newCanvas(8, 1)
	c.place(0, 0, "first")
	c.place(3, 0, "second")
	rows := c.render(lipgloss.NewStyle(), lipgloss.NewStyle())
	if rows[0] != "firsecon" {
		t.Fatalf("row %q", rows[0])
	}
}

func TestADiskTakesItsOwnLayer(t *testing.T) {
	withColor(t)
	c := newCanvas(10, 4)
	c.line(0, 8, 19, 8, 1)
	c.disk(10, 8, 3, 3)
	color := func(st lipgloss.Style) string { return strings.SplitN(st.Render("x"), "x", 2)[0] }
	rows := c.render(faint, litSt, threadDiskSt)
	if !strings.Contains(rows[2], color(threadDiskSt)) || !strings.Contains(rows[2], color(faint)) || strings.Contains(rows[0], color(threadDiskSt)) {
		t.Fatalf("rows %q", rows)
	}
	if strings.Contains(strings.Join(c.render(faint), ""), color(threadDiskSt)) {
		t.Fatal("a layer past the styles takes the last one")
	}
}
