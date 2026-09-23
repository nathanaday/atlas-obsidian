---
type: spec
thread: thr-20260922-5fe9
title: "Navigate the TUI map by parent nodes, then enter a cluster"
created: 2026-09-22
---

> [!spec] Navigate the TUI map by parent nodes, then enter a cluster
> [Stub](<../stubs/Navigate the TUI map by parent nodes, then enter a cluster.md>) → **Spec** → [Plan](<../plans/Navigate the TUI map by parent nodes, then enter a cluster.md>) → [Receipt](<../receipts/Navigate the TUI map by parent nodes, then enter a cluster.md>)
> `thr-20260922-5fe9` · [Thread](<../archive/Navigate the TUI map by parent nodes, then enter a cluster.md>) · filed 2026-09-22

## The problem

The view walks every project in one tour (`graph.tour`). That was fine for a few loose projects. Now hubs gather members: UFCF26 mirrors several projects, and more hubs will follow. With every node in one walk, the user steps through each member of every cluster to get from one group of work to the next. The screen also gives every node the same weight, so the clusters are not the first thing a user sees.

## Terms

- A **top project** is a project that no other project mirrors: it has no hubs. A project with no links is a top project. A project the atlas could not read is a top project.
- A **cluster** is a top project and every project it mirrors, directly or through another member (the closure of `members`, the same set `mirror.Sync` copies). A project can be in two clusters when two hubs mirror it.
- A project that no top project reaches (possible only in a cycle, which `mirror.Validate` refuses) counts as a top project, so every project is reachable.

## What will be true

1. **The overview.** The view opens on the whole map, as today. The arrow keys, Tab, and Shift+Tab walk only the top projects, along the same nearest-neighbor tour, and wrap. Members stay on the map, faded, so the clusters stay visible. The selected top project lights its whole cluster, not only its direct members.
2. **Enter on a hub enters its cluster.** The map then shows only the cluster's projects and the links between them. The layout starts from where those nodes stood on the overview and spreads to fill the screen, so the user sees the cluster open up and does not lose their place. The selection starts on the hub. The arrows walk every project in the cluster, as the tour does today.
3. **Inside a cluster**, Enter opens the card of the selection, as today. The launch keys, nudge, `n`, and `R` work as they do on the overview.
4. **Esc steps back one level:** it closes a panel first; inside a cluster with no panel open, it returns to the overview with the hub selected; on the overview, it quits, as today.
5. **Enter on a top project with no members opens its card**, as today. A cluster of one has nothing to enter. So an atlas with no links behaves exactly as it does now.
6. **The header shows where the user is:** `Atlas` on the overview, `Atlas › UFCF26` inside a cluster. On the overview, the summary line for a hub says how many projects its cluster holds and that Enter opens it.
7. **Find (`/`) searches every project, at either level.** When the match is outside the current view, the view moves to where the match is: into the cluster of the first top project that reaches it, or out to the overview for a top project. Esc puts the level and the selection back where they were.
8. **Refresh keeps the level.** The view stays inside the cluster when the hub still exists and returns to the overview when it does not. The selection stays on the same project when it is still in view.
9. `docs/usage.md`, the keys panel (`?`), the footer hints, and `README.md` describe the two levels.

## The experience

The overview answers "which body of work?" and a cluster answers "which project in it?". Moving between clusters takes one key per cluster, whatever their size. Entering a cluster must feel like zooming in, not like switching screens: the nodes keep their relative places and spread out. Esc always gets the user out, one level at a time.

## Out of scope

- Nested entry. A hub inside a cluster opens its card; it does not open a third level. The cluster already holds the nested hub's members, because it is the closure.
- A zoom or pan that keeps the rest of the map on screen around a cluster.
- Mouse support, and editing members from the view.

## Constraints

- The simulation stays deterministic for a set of projects, so tests can assert on the layout at both levels.
- The map still fits the screen at any size from 60 by 16 up.
- The launch keys and the settings panel do not change. No new action enters `actions.Atlas`: this is navigation inside the view only, so the rule that every TUI key has a CLI command does not apply.

## Decisions

- **A top project is one with no hubs**, not one with members. The stub asks for standalone projects to count, and this rule includes them with no special case.
- **The cluster is the closure, not the direct members.** It matches what a hub mirrors and what `mirror.Sync` copies. The option turned down: direct members only, which would need nested entry to reach the rest.
- **Enter enters; the hub's card is one more Enter away.** Turned down: a separate key to enter. Enter already means "go into the selection", and the stub asks for Enter.
- **The cluster runs its own simulation, seeded from the overview's positions.** It fills the screen and animates out from where the nodes were. Turned down: zooming the overview's frame onto the cluster's box. Other clusters' nodes can lie inside that box and would clutter it.
- **Members stay drawn on the overview, faded.** Hiding them would hide the clusters, and the clusters are the point of the feature.

## Verified by

- Graph tests: top projects and cluster closure on a hand-built graph, with a nested hub, a project that two hubs share, and a standalone project; the tour over top projects wraps; the cluster layout is deterministic and starts from the overview's positions.
- View tests driven with `tea.KeyMsg`: walk the top projects on a sample atlas with two hubs; Enter into a cluster and walk all of its projects; Enter for the card; Esc back to the overview with the hub selected; Esc on the overview quits; find jumps into a cluster and Esc restores the level; refresh inside a cluster keeps it; an atlas with no links behaves as it does today.
- By hand: run the view over the real atlas at 60 by 16 and at a full screen.

## Open questions the work will answer

- Whether the cluster's spread-out animation reads well at 30 frames a second, or needs a stronger reheat to feel like a zoom.
