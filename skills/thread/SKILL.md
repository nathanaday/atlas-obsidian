---
name: thread
description: "Orient in the project's threads: show the board by stage or one thread end to end, change a card (priority, phase, title, block, unblock), create or change a phase, and review the whole board for what is misguided, duplicated, or missing. Use for threads, board, what is open, what should I work on next, my threads, block, unblock, reprioritize, phases, review the board. To move a thread forward, thread-work."
---

# The threads

A thread is one line of work in a project: a feature, a bug, a chore. It
moves stub, spec, plan, receipt, and each stage is a document the user can
open under `threads/`. This skill is the home of that half: the board, the
quick changes to a card, the phases, and a review of the whole board. Moving
a thread to its next stage is `thread-work`'s.

Tools: `status`, `threads`, `thread`, `phase`. Reads
[threads.md](references/threads.md).

This session's project is implied; `project` reaches another one the atlas
lists. In a project with members, a thread named by id or title may be a
member's: the tool makes the change in the project that owns it. Say which
project took the change.

## Show the board

Call `threads`. Show the open threads under their stage, the furthest stage
first, with priority, phase, last update, and `blocked` or `stale` where it
applies. Name the notes that wait in the inbox and any page the tool could
not read, with what the page needs. Give the path of `threads/threads.md`: it
is the same board in Obsidian.

For one thread, call `threads` with `id` and read the documents it lists.

## Change a card

Priority, phase, title, block, unblock: one `thread` call with `id` and the
field. `blocked` holds what the thread waits on, in one line; `""` unblocks.
Never close a thread here: a receipt needs its text and its checks, and
`thread-receipt` writes it.

## Phases

`phase` with `action: create` (`title`, `goal`, optional `order`), `rename`
(`title`, `new_title`; every thread that names it follows), `reorder`, or
`remove` (refused while a thread names it). The goal is prose on the phase
page; refine it with Edit.

## Review the board

When the user asks what to work on, or whether the board makes sense:

1. Read the open threads' documents and the phase goals.
2. Read the wiki for the subjects they name: `wiki/hot.md`, the page that
   describes the work, a Grep. The review reads the wiki and never writes it.
3. Say what you see: threads that would not reach their phase's goal or that
   treat a symptom; threads that repeat each other or a closed one; what
   matters most and why; what is missing.
4. Offer the changes as a list and make the ones the user picks.

## Route

| The thread needs | Skill |
|---|---|
| To move on: the next stage, or all of them | `thread-work` |
| To exist: an idea, a bug, a chore, a note in the inbox | `thread-stub` |
| A definition of done | `thread-spec` |
| An approach | `thread-plan` |
| The work | `thread-run` |
| An end: completed or killed | `thread-receipt` |

## Hand off

`thread-work` for the thread in view, when the user wants it to move. The
wiki: `wiki`. Every project: `atlas`.
