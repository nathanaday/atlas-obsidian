---
type: plan
thread: thr-20260922-5fe9
title: "Navigate the TUI map by parent nodes, then enter a cluster"
created: 2026-09-22
---

> [!plan] Navigate the TUI map by parent nodes, then enter a cluster
> [Stub](<../stubs/Navigate the TUI map by parent nodes, then enter a cluster.md>) → [Spec](<../specs/Navigate the TUI map by parent nodes, then enter a cluster.md>) → **Plan** → Receipt
> `thr-20260922-5fe9` · [Thread](<../Navigate the TUI map by parent nodes, then enter a cluster.md>) · filed 2026-09-22

## Approach

The view keeps two graphs: `top`, the overview of every project, and `graph`, the map on screen. On the overview they are the same graph. Entering a cluster builds a new graph from the hub and its closure, copying each node's position from the overview, and makes it the map on screen. Every function that draws, walks, nudges, and ticks already reads `v.graph`, so they work at both levels unchanged. The card and the summary read a project's links from `top`, so a member that two hubs share still lists both hubs inside either cluster.

To make the entry read as a zoom, the view holds a camera during the change: it starts at the overview's transform, so the cluster's nodes first appear exactly where they stood, and eases toward the cluster's fit over about 20 frames while the cluster's simulation spreads the nodes. The ticks run while the simulation has energy or the camera is moving.

Turned down: zooming the overview's frame onto the cluster's box. It is less code, but the nodes of other clusters can sit inside that box.

## Where the work lands

- `internal/tui/graph.go`: `tops` (projects with no hubs, plus a node for each cycle no top project reaches), `closure`, `sub` (the cluster graph seeded from positions), and `tour` over a set of nodes.
- `internal/tui/view.go`: the level (`cluster` id, `top`), Enter, Esc, the camera, find across levels, refresh at a level, the header, the summary, the node and edge styles on the overview, the hints, and the keys panel.
- `internal/tui/graph_test.go`, `view_test.go`: the tests the spec names.
- `docs/usage.md`, `README.md`: the two levels.

## Order

1. Graph: `tops`, `closure`, `sub`, `tour(among)`, with tests. The view walks the whole map as before.
2. View: the overview walks the top projects; Enter enters; Esc leaves; the header, summary, styles, and hints; with tests.
3. The camera ease on entry, find across levels, refresh inside a cluster, with tests.
4. Docs, then a run by hand over the real atlas.

## Tests

`make test` passes. The graph and view tests in the spec. A frame at 60 by 16 inside a cluster fits the screen.

## Risks

- A find typed one key at a time may cross clusters on each key. The view builds a cluster only when the match leaves the current one.
- Refresh rebuilds the overview from its seed positions, so a cluster rebuilt after it takes the positions of the old cluster graph where a node still exists, and does not jump.

## Progress

- 2026-09-22: All four slices done. `graph.tops`, `closure`, `sub`, and `tour(among)`; the view holds `top` and the map on screen, a camera that eases from the overview's transform to the cluster's fit, find across levels, and refresh that keeps the cluster. `make test` passes.
- 2026-09-22: Checked by hand over the real atlas (17 projects, one hub, `usc-f26`) at 120 by 32 and 60 by 16, reading the registry only. The overview walks 13 top projects. The cluster's first frame matches the overview exactly, then it spreads to fill the height.
- 2026-09-22: Changes from the spec. A hub on the overview wears a ring (`◉`), because nothing else showed that Enter opens it. The footer's first hint says "Enter open cluster". The summary names the cluster ("3 in its cluster: …") and does not repeat that Enter opens it. Inside a cluster, the footer adds "Esc back". Leaving a cluster snaps back to the overview and does not animate.
- Next: the receipt.
- 2026-09-22: The user found the overview unbalanced at startup. Cause: `Init` has a value receiver, so its `ticking = true` was lost and the first tick was dropped; the overview stayed at its seed circle until a cluster's ticks ran on after Esc. The bug predates this thread; the `sized` test helper set `ticking` itself and hid it. `newView` now starts the ticks, `sized` takes the real path, and `TestTheMapSettlesFromStartup` fails on the old code.
