---
type: plan
thread: thr-20260922-a72e
title: "The view as a graph of projects"
created: 2026-09-22
---

> [!plan] The view as a graph of projects
> [Stub](<../stubs/The view as a graph of projects.md>) → [Spec](<../specs/The view as a graph of projects.md>) → **Plan** → [Receipt](<../receipts/The view as a graph of projects.md>)
> `thr-20260922-a72e` · [Thread](<../archive/The view as a graph of projects.md>) · filed 2026-09-22

## Approach

Three new files in `internal/tui` and a rewrite of the model. `graph.go` is the simulation: nodes with positions and velocities in braille-dot space, where a dot is about square, so distances mean the same on both axes. `canvas.go` is the renderer: a braille buffer for edges with a color per cell, and a text layer for markers and labels placed after the edges. `card.go` is the panel. `view.go` keeps the Bubble Tea model, the openers, the find prompt, and the settings panel, and drops the boards, the tabs, and the caption. `board.go` and `detail.go` go, with their tests; what the card needs of `detail.go` moves to `card.go`.

The simulation runs on `tea.Tick` at 33 ms while its energy is above a threshold, then stops; any change (select, nudge, resize, reload) restarts it. Initial positions come from a hash of each project's id on a circle, so the layout is a pure function of the projects and the screen, and tests can assert on it. The view transform fits every node into the canvas each frame with a smoothed scale.

## Where the work lands

1. `internal/tui/graph.go`: `graph`, `node`, `edge`; `build(items)`; `step(dt)`; `settled()`; `nudge(id, dx, dy)`; `nearest(from, direction)`.
2. `internal/tui/canvas.go`: `canvas` over braille cells; `line(x0, y0, x1, y1, color)`; `label(cell, text, style)`; `render() []string`.
3. `internal/tui/card.go`: `cardLines(entry, ix)`: the rows that have content; members and mirrored-by resolved through the entries.
4. `internal/tui/view.go`: the model, the header, the summary line, the footer that drops hints from the right until it fits, the find prompt, the settings panel, the help overlay, the launch keys, `RunView`.
5. `internal/terminal`: `Open(root)`; `actions.Atlas.OpenTerminal`; CLI `open-terminal NAME`; `docs/usage.md` rows; `CLAUDE.md` layout lines.
6. Tests: `graph_test.go`, `canvas_test.go`, `view_test.go` rewritten, `terminal_test.go`, a CLI case.

## Order

graph and canvas with tests; the model and the card; the terminal package and its command; docs; the whole suite; a hand run in a real terminal.

## Risks

Braille and label placement must keep the frame exactly the screen's size, or the terminal scrolls. Every line is clipped to the width with `lipgloss` before it is joined. The simulation must not spin forever on an unstable configuration: a fixed iteration cap ends it.

## Progress

Started and built 2026-09-22. The map, the card, find, settings, the keys panel, and the `t` key are in; `board.go` and `detail.go` are gone. The built binary was driven in a pseudo-terminal: it enters the alternate screen, draws braille edges before any key arrives, opens the keys panel on `?`, and leaves the screen clean on `q`. The pseudo-terminal had to answer the terminal's background-color and cursor-position queries, which Lip Gloss sends at start and waits on.

Two things moved while building. A run placed later on the canvas covers an earlier one, so a panel covers the labels under it. The footer's keys carry a rank and the least needed go first, so `q quit` and `? keys` survive a narrow screen while `R refresh` goes.
