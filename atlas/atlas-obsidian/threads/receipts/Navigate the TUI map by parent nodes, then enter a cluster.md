---
type: receipt
thread: thr-20260922-5fe9
title: "Navigate the TUI map by parent nodes, then enter a cluster"
outcome: completed
created: 2026-09-22
---

> [!receipt] Navigate the TUI map by parent nodes, then enter a cluster · completed
> [Stub](<../stubs/Navigate the TUI map by parent nodes, then enter a cluster.md>) → [Spec](<../specs/Navigate the TUI map by parent nodes, then enter a cluster.md>) → [Plan](<../plans/Navigate the TUI map by parent nodes, then enter a cluster.md>) → **Receipt**
> `thr-20260922-5fe9` · [Thread](<../archive/Navigate the TUI map by parent nodes, then enter a cluster.md>) · filed 2026-09-22

## Delivered

The map has two levels. The overview shows every project, and the arrows walk only the top projects: those no other project mirrors, including a project with no links. A hub on the overview wears a ring (`◉`), and selecting it lights its whole cluster: the hub and every project it mirrors, directly or through a member. Enter on a hub opens its cluster onto the whole screen. The nodes start where they stood on the overview, and the camera eases to the cluster's fit while they spread out. The header reads `Atlas › <hub>`, and the arrows walk every project in the cluster. Esc closes a panel, then leaves the cluster with the hub selected, then quits. Find searches every project and moves to the level that holds the match; Esc puts the level and the selection back. Refresh keeps the cluster and its layout. An atlas with no links has one level and behaves as before.

The work also fixed a startup bug from the earlier map work: `Init` has a value receiver, so the ticks never started, and the overview stayed on its seed circle until something else started them.

Commits on `main`: `1c4b217` (the two levels), `5f71844` (the map settles at startup), `aa8de4a` (review fixes). Files: `internal/tui/graph.go`, `internal/tui/view.go`, their tests, `docs/usage.md`, `README.md`, `CLAUDE.md`.

## Verified

- `make test` passes.
- The graph tests cover top projects, cluster closure with a nested hub, a member two hubs share, a problem node, a cycle, and a cluster seeded from the overview that is deterministic and keeps its layout across a rebuild.
- The view tests cover walking the top projects, entering a cluster, the camera, walking inside, the card, Esc at each level, find across levels and back, refresh inside a cluster and after the hub loses its links, an atlas with no links, a 60 by 16 cluster, startup ticks, and Esc to an overview that has not settled.
- By hand: the real atlas (17 projects, hub `usc-f26`) rendered at 120 by 32 and 60 by 16. The user ran it and confirmed the startup layout.
- A fresh reviewer read the diff against the spec. It found four problems, all fixed in `aa8de4a` with tests:
  - Esc out of a cluster left an overview that had not settled frozen.
  - When find moved from one cluster into another, the camera started from the old cluster's frame.
  - Enter on a hub under the keys panel opened the card, not the cluster.
  - The top projects were recomputed for every label on every frame.

## Where the work left the spec

- The `◉` marker for a hub. Nothing else showed that Enter opens it.
- The footer, not the summary, says "Enter open cluster".
- Leaving a cluster snaps back to the overview and does not animate.

## Not done

- A refresh on the overview rebuilds the map from its seed positions and animates again, as it did before this thread.
- A top project that becomes a member in a refresh loses the selection to the walk's start.
