---
type: spec
thread: thr-20260922-a72e
title: "The view as a graph of projects"
created: 2026-09-22
---

> [!spec] The view as a graph of projects
> [Stub](<../stubs/The view as a graph of projects.md>) → **Spec** → [Plan](<../plans/The view as a graph of projects.md>) → [Receipt](<../receipts/The view as a graph of projects.md>)
> `thr-20260922-a72e` · [Thread](<../archive/The view as a graph of projects.md>) · filed 2026-09-22

## What is wrong today

Rendered at 100 columns over three projects, the view shows a two-line caption that repeats the documentation, a box per project with its full path (clipped against the wrong width, so it wraps and leaves a lone "…"), a description that repeats the name, and a facts line that says "no threads · inbox empty" when there is nothing to say. Expanding a box adds twelve rows that run past the box and are cut at the screen edge. The footer breaks between "q" and "quit". The heat emoji have inconsistent cell widths. The list is sorted by name and says nothing about how projects connect, which the member feature makes the main fact. Enter expands in place and pushes the rest off screen; the cursor stops on an "(end)" marker; Config is a peer tab of the data; there is no find.

## What will be true

1. `atlas-obsidian` opens one screen: a header line (the atlas, its counts, when it was refreshed), the map, one summary line for the selection, and one footer line of keys that never wraps.
2. The map draws every project as a node and every member link as an edge, laid out by a force simulation (repulsion between nodes, springs along edges, gravity to the center, damping) that runs at about 30 frames a second while it has energy and sleeps when it settles. Edges are braille lines; nodes are a marker and the project's name. The layout is deterministic for a given set of projects, so tests can assert on it.
3. The selected node is filled with the project color. Its members and the projects that mirror it are lit; every other node and edge is dim. The summary line says the project's name, what it mirrors and what mirrors it, and its page and open-thread counts when they are not zero.
4. Arrow keys move the selection to the nearest node in that direction; Tab and Shift+Tab cycle by name; Shift+arrows nudge the selected node and the simulation responds. `/` opens a find prompt that jumps to the first name that matches as the user types. `R` refreshes in the background. `?` shows every key. `,` opens the settings panel (harness, IDE), which replaces the Config tab. `q` and Ctrl+C quit; Esc closes whatever is open.
5. Enter opens the card: a panel over the map with only the rows that have content: path, description, members, mirrored by, git, wiki, threads (counts and up to five open ones), inbox, last touched, signals. A project the atlas could not read is a dim red node whose card shows the path, the reason, and the fix. Esc or Enter closes the card. The launch keys work from the map and the card alike.
6. `o` opens the project in Obsidian, `c` starts the preferred harness, `i` opens the preferred IDE, `n` opens a thread, as today. `t` opens a terminal window at the work folder: a new package `internal/terminal` picks the terminal the user is running in on macOS (`TERM_PROGRAM`) or `Terminal`, and `$TERMINAL` or a known emulator on Linux; `actions.Atlas.OpenTerminal` and the command `open-terminal NAME` expose it.
7. The map fits the screen at any size from 60 by 16 up, and re-fits on resize. Labels never wrap; a label that would overlap another moves a row.
8. The Problems tab, the caption, the box list, the "(end)" marker, and the heat emoji are gone. `docs/usage.md` documents the screen and its keys, with `open-terminal` in the command table; `CLAUDE.md` names the new packages.
9. Tests: the simulation settles and is deterministic; the braille canvas draws a known line; the model is driven with `tea.KeyMsg` through selection, nudge, find, card, settings, and every launch key; `internal/terminal` is tested with a fake launcher on PATH; the CLI test covers `open-terminal`.

## Out of scope

Mouse support. Zoom and pan; the map always fits the screen. Editing members from the view.
