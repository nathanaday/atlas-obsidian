---
type: receipt
thread: thr-20260922-a72e
title: "The view as a graph of projects"
outcome: completed
created: 2026-09-22
---

> [!receipt] The view as a graph of projects · completed
> [Stub](<../stubs/The view as a graph of projects.md>) → [Spec](<../specs/The view as a graph of projects.md>) → [Plan](<../plans/The view as a graph of projects.md>) → **Receipt**
> `thr-20260922-a72e` · [Thread](<../archive/The view as a graph of projects.md>) · filed 2026-09-22

## Delivered

- `internal/tui/graph.go`: the simulation. Nodes in braille-dot space, repulsion between every pair, springs along member links, gravity, drag, a decaying energy that settles and sleeps; `nearest` by direction, `cycle`, `find`, `nudge`. Deterministic for a set of projects.
- `internal/tui/canvas.go`: braille edges with a lit and a dim layer, text runs over them, the later run on top, every line exactly the screen's width.
- `internal/tui/card.go`: the card with only the rows that have content, and the panel box.
- `internal/tui/view.go`: the one-line header, the map, the summary line, the footer that drops the least needed keys first; arrows, Tab, Shift+arrows, `/` find, Enter card, `?` keys, `,` settings, `o c i t n R`, Esc, `q`. Problems are red nodes with a card that says the fix. The list, the tabs, the caption, the Problems tab, and the heat emoji are gone.
- `internal/terminal`: opens a terminal window at a folder, the user's own on macOS and `$TERMINAL` or a known emulator on Linux; `actions.Atlas.OpenTerminal`; `atlas-obsidian open-terminal NAME`.
- `docs/usage.md`, `README.md`, `CLAUDE.md`.

## Verified

`go vet ./...` and `go test ./...` pass. Tests cover the simulation (links, self excluded, settles, deterministic, nudge wakes), the canvas (braille, layers, clipping, overlap), the model (the frame's size, the header's counts, every label whole, the summary's links and facts, the styles of the selection and its neighbors, arrows and Tab and find, a nudge that ticks until it settles, the card's rows and the problem's fix, help and settings, every launch key in the background and on a problem, refresh keeping the selection, the thread prompt and a real stub, the footer at widths down to 12), the terminal package with a fake launcher, and the command. The built binary was driven in a pseudo-terminal end to end.

## Not done

Nothing in scope. Uncommitted, for review. Mouse support, zoom, and pan stay out.
