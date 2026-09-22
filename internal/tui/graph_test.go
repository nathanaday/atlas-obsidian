package tui

import (
	"math"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/registry"
)

func linkedItems() []Item {
	hub := Item{Entry: registry.Entry{ID: "hub", Name: "platform", Path: "/p", Members: []string{"a", "b"}}}
	a := Item{Entry: registry.Entry{ID: "a", Name: "svc-a", Path: "/a", Members: []string{"b"}}}
	b := Item{Entry: registry.Entry{ID: "b", Name: "svc-b", Path: "/b"}}
	lone := Item{Entry: registry.Entry{ID: "c", Name: "notes", Path: "/c"}}
	broken := Item{Entry: registry.Entry{Path: "/old/gateway", Error: "not found", Reason: registry.ReasonMissing}}
	return []Item{hub, a, b, lone, broken}
}

func TestBuildGraphLinksMembersBothWays(t *testing.T) {
	g := buildGraph(linkedItems())
	if len(g.nodes) != 5 {
		t.Fatalf("nodes %d", len(g.nodes))
	}
	hub, a, b := g.byID["hub"], g.byID["a"], g.byID["b"]
	if len(g.nodes[hub].members) != 2 || len(g.nodes[b].hubs) != 2 || len(g.nodes[a].hubs) != 1 {
		t.Fatalf("links %+v", g.nodes)
	}
	if !g.linked(hub, a) || !g.linked(b, a) || g.linked(hub, g.byID["c"]) {
		t.Fatal("linked")
	}
	if !g.nodes[g.byID["path:/old/gateway"]].problem {
		t.Fatal("a problem is a node")
	}
	// Nodes sort by name, so the order on screen is stable.
	if g.nodes[0].name != "gateway" || g.nodes[1].name != "notes" || g.nodes[2].name != "platform" {
		t.Fatalf("order %v %v", g.nodes[0].name, g.nodes[1].name)
	}
}

func TestTheLayoutSettlesAndIsTheSameTwice(t *testing.T) {
	g := buildGraph(linkedItems())
	g.settle()
	if !g.settled() {
		t.Fatal("did not settle")
	}
	hub, a, c := g.byID["hub"], g.byID["a"], g.byID["c"]
	near := math.Hypot(g.nodes[hub].x-g.nodes[a].x, g.nodes[hub].y-g.nodes[a].y)
	far := math.Hypot(g.nodes[hub].x-g.nodes[c].x, g.nodes[hub].y-g.nodes[c].y)
	if near > 2.5*linkLength || far < near {
		t.Fatalf("linked nodes at %.1f, an unlinked one at %.1f", near, far)
	}
	h := buildGraph(linkedItems())
	h.settle()
	for i := range g.nodes {
		if g.nodes[i].x != h.nodes[i].x || g.nodes[i].y != h.nodes[i].y {
			t.Fatal("two builds differ")
		}
	}
	// A nudge wakes it, and it settles again.
	g.nudge(hub, 20, 0)
	if g.settled() {
		t.Fatal("a nudge should wake the simulation")
	}
	g.settle()
	if !g.settled() {
		t.Fatal("did not settle after a nudge")
	}
}

func TestTheTourVisitsEveryNodeOnceByShortHops(t *testing.T) {
	g := &graph{nodes: []node{{name: "far right", x: 30}, {name: "left", x: -20}, {name: "mid", x: 0}, {name: "up", x: 1, y: -8}}}
	order := g.tour()
	var names []string
	for _, i := range order {
		names = append(names, g.nodes[i].name)
	}
	if strings.Join(names, ",") != "left,mid,up,far right" {
		t.Fatalf("tour %v", names)
	}
	// Forward wraps from the last to the first, and back from the first to the last.
	if g.along(order[3], 1) != order[0] || g.along(order[0], -1) != order[3] {
		t.Fatal("wrap")
	}
	if g.along(order[1], 1) != order[2] || g.along(order[2], -1) != order[1] {
		t.Fatal("steps")
	}
	if (&graph{}).along(0, 1) != -1 {
		t.Fatal("empty")
	}
	g = buildGraph(linkedItems())
	g.settle()
	seen := map[int]bool{}
	for _, i := range g.tour() {
		seen[i] = true
	}
	if len(seen) != len(g.nodes) {
		t.Fatalf("the tour covers %d of %d nodes", len(seen), len(g.nodes))
	}
}

func TestFind(t *testing.T) {
	g := buildGraph(linkedItems())
	if i := g.find("svc"); g.nodes[i].name != "svc-a" {
		t.Fatalf("find prefix: %s", g.nodes[i].name)
	}
	if i := g.find("FORM"); g.nodes[i].name != "platform" {
		t.Fatalf("find inside: %s", g.nodes[i].name)
	}
	if g.find("zzz") != -1 || g.find("") != -1 {
		t.Fatal("no match")
	}
}
