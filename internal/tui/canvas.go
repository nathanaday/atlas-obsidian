package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// canvas is a screen of cells: braille dots for the edges, with a color per cell, and
// text runs laid over them for markers, labels, and panels. Its rendering is exactly
// h lines of exactly w cells.
type canvas struct {
	w, h  int
	dots  []uint8
	layer []int8 // the strongest color drawn in a cell: 0 none, then 1 up, one per style render takes
	runs  []run
}

// run is text laid over the dots, starting at a cell.
type run struct {
	x, y  int
	text  string
	width int
}

// The braille bit of each dot in a cell: two across, four down.
var brailleBits = [4][2]uint8{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

func newCanvas(w, h int) *canvas {
	w, h = max(1, w), max(1, h)
	return &canvas{w: w, h: h, dots: make([]uint8, w*h), layer: make([]int8, w*h)}
}

// dotsWide and dotsHigh are the canvas in dots.
func (c *canvas) dotsWide() int { return c.w * 2 }
func (c *canvas) dotsHigh() int { return c.h * 4 }

// dot sets one dot, at a strength that only a stronger one replaces.
func (c *canvas) dot(x, y int, layer int8) {
	if x < 0 || y < 0 || x >= c.dotsWide() || y >= c.dotsHigh() {
		return
	}
	i := (y/4)*c.w + x/2
	c.dots[i] |= brailleBits[y%4][x%2]
	if layer > c.layer[i] {
		c.layer[i] = layer
	}
}

// line draws between two dots.
func (c *canvas) line(x0, y0, x1, y1 int, layer int8) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		c.dot(x0, y0, layer)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// disk fills the dots within r of a center dot.
func (c *canvas) disk(cx, cy, r int, layer int8) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if dx, dy := x-cx, y-cy; dx*dx+dy*dy <= r*r {
				c.dot(x, y, layer)
			}
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// place lays text over the cells from (x, y). Text past the right edge is cut. A run
// covers whatever dots lie under it.
func (c *canvas) place(x, y int, text string) {
	if y < 0 || y >= c.h {
		return
	}
	if x < 0 {
		x = 0
	}
	width := lipgloss.Width(text)
	if x+width > c.w {
		text = lipgloss.NewStyle().MaxWidth(c.w - x).Render(text)
		width = lipgloss.Width(text)
	}
	if width == 0 {
		return
	}
	c.runs = append(c.runs, run{x: x, y: y, text: text, width: width})
}

// free reports whether the cells [x, x+width) on row y hold no run yet.
func (c *canvas) free(x, y, width int) bool {
	for _, r := range c.runs {
		if r.y == y && x < r.x+r.width && r.x < x+width {
			return false
		}
	}
	return true
}

// render is the canvas as h lines, each exactly w cells wide. A cell's dots take the
// style of its layer, styles[layer-1], the last style for any layer past them. Where runs
// overlap, the one placed last shows, so a panel placed after the labels covers them.
func (c *canvas) render(styles ...lipgloss.Style) []string {
	out := make([]string, c.h)
	for y := 0; y < c.h; y++ {
		owner := make([]int, c.w)
		for x := range owner {
			owner[x] = -1
		}
		for i, r := range c.runs {
			if r.y != y {
				continue
			}
			for x := r.x; x < r.x+r.width && x < c.w; x++ {
				owner[x] = i
			}
		}
		var b strings.Builder
		for x := 0; x < c.w; {
			if owner[x] >= 0 {
				r := c.runs[owner[x]]
				end := x
				for end < c.w && owner[end] == owner[x] {
					end++
				}
				if x == r.x && end == r.x+r.width {
					b.WriteString(r.text)
				} else {
					plain := []rune(stripANSI(r.text))
					b.WriteString(string(plain[x-r.x : end-r.x]))
				}
				x = end
				continue
			}
			i := y*c.w + x
			if c.dots[i] == 0 {
				b.WriteByte(' ')
			} else {
				st := styles[max(0, min(int(c.layer[i]), len(styles))-1)]
				b.WriteString(st.Render(string(rune(0x2800 + int(c.dots[i])))))
			}
			x++
		}
		out[y] = b.String()
	}
	return out
}
