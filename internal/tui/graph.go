package tui

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"
)

// The map's units are braille dots: two across and four down per cell, which is close
// to square on most terminals, so a distance means the same on both axes.
const (
	linkLength = 30.0 // the rest length of an edge, in dots
	charge     = 900.0
	gravity    = 0.03
	drag       = 0.6 // what a velocity keeps each step
	alphaDecay = 0.03
	alphaMin   = 0.004
	reheat     = 0.35 // the energy a nudge or a change puts back
	maxStep    = 12.0 // the furthest a node moves in one step
)

// node is one project on the map.
type node struct {
	id      string
	name    string
	problem bool
	x, y    float64
	vx, vy  float64
	// members are the nodes this one mirrors; hubs are the nodes that mirror it.
	members []int
	hubs    []int
}

// graph is the projects and their member links, with the state of the simulation.
type graph struct {
	nodes []node
	byID  map[string]int
	alpha float64
}

// buildGraph lays the projects on a circle, each at an angle its id fixes, and links
// every project to the members the atlas lists. The layout that follows is then a
// function of the projects alone.
func buildGraph(items []Item) *graph {
	g := &graph{byID: map[string]int{}, alpha: 1}
	for _, it := range items {
		e := it.Entry
		id := e.ID
		if id == "" {
			id = "path:" + e.Path
		}
		g.byID[id] = len(g.nodes)
		g.nodes = append(g.nodes, node{id: id, name: entryName(e), problem: e.Error != ""})
	}
	sort.SliceStable(g.nodes, func(i, j int) bool {
		return strings.ToLower(g.nodes[i].name) < strings.ToLower(g.nodes[j].name)
	})
	for i, n := range g.nodes {
		g.byID[n.id] = i
	}
	radius := linkLength * math.Max(1, math.Sqrt(float64(len(g.nodes))))
	for i := range g.nodes {
		angle := seed(g.nodes[i].id) * 2 * math.Pi
		g.nodes[i].x = radius * math.Cos(angle)
		g.nodes[i].y = radius * math.Sin(angle)
	}
	for _, it := range items {
		from, ok := g.byID[it.Entry.ID]
		if !ok || it.Entry.Error != "" {
			continue
		}
		for _, id := range it.Entry.Members {
			to, ok := g.byID[id]
			if !ok || to == from {
				continue
			}
			g.nodes[from].members = append(g.nodes[from].members, to)
			g.nodes[to].hubs = append(g.nodes[to].hubs, from)
		}
	}
	return g
}

// seed is a number in [0, 1) that an id always maps to.
func seed(id string) float64 {
	h := fnv.New32a()
	h.Write([]byte(id))
	return float64(h.Sum32()%10007) / 10007
}

// edges lists every link once, hub first.
func (g *graph) edges() [][2]int {
	var out [][2]int
	for i, n := range g.nodes {
		for _, m := range n.members {
			out = append(out, [2]int{i, m})
		}
	}
	return out
}

// linked reports whether two nodes share an edge, either way.
func (g *graph) linked(a, b int) bool {
	for _, m := range g.nodes[a].members {
		if m == b {
			return true
		}
	}
	for _, m := range g.nodes[b].members {
		if m == a {
			return true
		}
	}
	return false
}

// settled reports whether the simulation has run out of energy.
func (g *graph) settled() bool { return g.alpha < alphaMin || len(g.nodes) < 2 }

// wake puts energy back so the layout moves again.
func (g *graph) wake() {
	if g.alpha < reheat {
		g.alpha = reheat
	}
}

// nudge pushes one node and wakes the simulation, so the rest of the map answers.
func (g *graph) nudge(i int, dx, dy float64) {
	if i < 0 || i >= len(g.nodes) {
		return
	}
	g.nodes[i].x += dx
	g.nodes[i].y += dy
	g.wake()
}

// step advances the simulation one tick: every node repels every other, each edge pulls
// its ends toward the rest length, gravity pulls everything toward the center, and drag
// takes the rest. The energy decays, so a layout left alone settles and stops.
func (g *graph) step() {
	if g.settled() {
		return
	}
	a := g.alpha
	g.alpha *= 1 - alphaDecay
	n := g.nodes
	for i := range n {
		for j := i + 1; j < len(n); j++ {
			dx, dy := n[j].x-n[i].x, n[j].y-n[i].y
			d2 := dx*dx + dy*dy
			if d2 < 1 {
				// Two nodes on one spot push apart along a line their order fixes.
				dx, dy, d2 = 1, 0.5, 1.25
			}
			f := charge * a / d2
			n[i].vx -= dx / math.Sqrt(d2) * f
			n[i].vy -= dy / math.Sqrt(d2) * f
			n[j].vx += dx / math.Sqrt(d2) * f
			n[j].vy += dy / math.Sqrt(d2) * f
		}
	}
	for _, e := range g.edges() {
		i, j := e[0], e[1]
		dx, dy := n[j].x-n[i].x, n[j].y-n[i].y
		d := math.Max(1, math.Hypot(dx, dy))
		f := (d - linkLength) / d * a * 0.5
		n[i].vx += dx * f
		n[i].vy += dy * f
		n[j].vx -= dx * f
		n[j].vy -= dy * f
	}
	for i := range n {
		n[i].vx -= n[i].x * gravity * a
		n[i].vy -= n[i].y * gravity * a
		n[i].vx *= drag
		n[i].vy *= drag
		if v := math.Hypot(n[i].vx, n[i].vy); v > maxStep {
			n[i].vx *= maxStep / v
			n[i].vy *= maxStep / v
		}
		n[i].x += n[i].vx
		n[i].y += n[i].vy
	}
}

// settle runs the simulation until it stops, for a first frame that is already laid
// out, and as a bound in tests.
func (g *graph) settle() {
	for i := 0; i < 1000 && !g.settled(); i++ {
		g.step()
	}
}

// bounds is the box every node lies in.
func (g *graph) bounds() (minX, minY, maxX, maxY float64) {
	if len(g.nodes) == 0 {
		return 0, 0, 0, 0
	}
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, n := range g.nodes {
		minX, maxX = math.Min(minX, n.x), math.Max(maxX, n.x)
		minY, maxY = math.Min(minY, n.y), math.Max(maxY, n.y)
	}
	return
}

// tour is the order the arrow keys walk the map in: from the leftmost node, always to
// the nearest node not yet visited. Every node is reached once, each step is a short
// hop, and the walk never bounces between two neighbors. It follows the layout, so it
// is stable once the map has settled.
func (g *graph) tour() []int {
	n := len(g.nodes)
	if n == 0 {
		return nil
	}
	start := 0
	for i := range g.nodes {
		if g.nodes[i].x < g.nodes[start].x {
			start = i
		}
	}
	order := []int{start}
	seen := make([]bool, n)
	seen[start] = true
	for len(order) < n {
		at := g.nodes[order[len(order)-1]]
		next, best := -1, math.Inf(1)
		for i := range g.nodes {
			if seen[i] {
				continue
			}
			d := math.Hypot(g.nodes[i].x-at.x, g.nodes[i].y-at.y)
			if d < best {
				next, best = i, d
			}
		}
		seen[next] = true
		order = append(order, next)
	}
	return order
}

// step along the tour: the node delta places after from, wrapping at the ends.
func (g *graph) along(from, delta int) int {
	order := g.tour()
	if len(order) == 0 {
		return -1
	}
	at := 0
	for i, id := range order {
		if id == from {
			at = i
		}
	}
	return order[((at+delta)%len(order)+len(order))%len(order)]
}

// find is the first node whose name contains text, without regard to case, or -1.
func (g *graph) find(text string) int {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return -1
	}
	for i, n := range g.nodes {
		if strings.HasPrefix(strings.ToLower(n.name), text) {
			return i
		}
	}
	for i, n := range g.nodes {
		if strings.Contains(strings.ToLower(n.name), text) {
			return i
		}
	}
	return -1
}
