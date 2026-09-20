---
name: wiki-mode
description: "Read or change the project's filing mode and see where a new wiki page of a type belongs: generic (typed folders) or lyt (atomic notes and Maps of Content). Use for wiki mode, what is my mode, set the mode, switch to LYT, use generic, methodology routing. Does not move existing notes."
---

# Filing mode

The mode decides where a new page goes in the wiki. It changes nothing about
evidence or existing files. Tools: `route` to read it, `project` to change it.

| Mode | New pages | Navigation |
|---|---|---|
| `generic` (default) | `wiki/sources/`, `entities/`, `concepts/` by type | `wiki/index.md` |
| `lyt` | `wiki/notes/`, one idea per note, whatever the type | `wiki/mocs/*.md`; `wiki/index.md` is the home map |

## Read and route

`status` gives the current mode. Call `route` with a `type` and `title` to see
the path a new page would take, the skeleton it starts from, and whether that
page already exists. A calling skill may choose a more specific destination when
the user names one, but every write still goes through `plan`.

## Change the mode

1. Confirm the target mode with the user: `generic` or `lyt`.
2. Call `project` with `action: edit` and `mode`. It rewrites one field of
   `project.json` as one commit and nothing else.
3. Show the old mode and the new one, and say that only future pages follow it.

The change affects future pages only. It never creates folders, moves notes,
rewrites links, or migrates content. If the user wants existing pages
reorganized, plan that as a separate `markdown` operation with a complete move
map, and warn that moved pages need their links checked with `wiki-lint`.

## LYT conventions

- A note holds one idea and fits on a screen; split what grows past that.
- A note's `mocs:` property names the maps that reach it.
- A MOC is a navigation hub, not a container; notes stay flat in `wiki/notes/`.
- Every new note joins at least one MOC in the same plan.
