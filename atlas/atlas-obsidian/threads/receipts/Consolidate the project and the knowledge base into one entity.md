---
type: receipt
thread: thr-20260919-26da
title: "Consolidate the project and the knowledge base into one entity"
outcome: completed
created: 2026-09-20
---

> [!receipt] Consolidate the project and the knowledge base into one entity · completed
> [Stub](<../stubs/Consolidate the project and the knowledge base into one entity.md>) → [Spec](<../specs/Consolidate the project and the knowledge base into one entity.md>) → [Plan](<../plans/Consolidate the project and the knowledge base into one entity.md>) → **Receipt**
> `thr-20260919-26da` · [Thread](<../archive/Consolidate the project and the knowledge base into one entity.md>) · filed 2026-09-20

Completed. A project is one entity now: `atlas/<name>/` holds the wiki with its
engine, its inbox, its ideas, and its raw store, and the threads with their
stages under `threads/`. `docs/v4-design.md` is the design of record, and
`CLAUDE.md` carries the five rules that replaced the three sets.

What was delivered, against the spec's list:

- One entity. `internal/vault` is gone, merged into `internal/project`;
  `internal/vaults` is `internal/manage`.
- The stage folders sit under `threads/` with the cards and the board, and every
  callout link, the CSS snippet, and the board agree.
- No linking anywhere: no `knowledge` field, no `link`, `unlink`,
  `new-knowledge`, `adopt`, `remove`, or `mode`; one session kind.
- The engine's git commands are scoped to `wiki/`, `.raw/`, `inbox/`, and
  `project.json` (`project.EngineScope`), so an apply never commits the user's
  code or a thread document, and an undo can never revert one.
- One inbox, with a per-item hint of source or note.
- `scope` folded into `description`; `mode` stays in `project.json`; the `vault`
  and `mode` tools folded into `project`, leaving seventeen.
- One session-start hook, one guard, one list in the view.
- `atlas-obsidian upgrade` covers the four migration cases and refuses a knowledge
  base several projects used, naming them.

How it was verified: `make test` passes (`go vet` and `gofmt` clean). New tests
cover the scoped git commands, an apply and an undo beside dirty code and a dirty
thread document, undo's refusal when a page changed since, and one test per
migration case. By hand, twice on a clone of this repository and once here:
`atlas-obsidian upgrade` absorbed `atlas_kb` into `atlas/atlas-obsidian/` with
`git mv`, so `git log --follow` still traces a moved page through the knowledge
base's history; an apply and an undo left the dirty code and the dirty thread
document untouched; the hook, the board, and `lint` read the upgraded project.

Where the work left the plan: `gitx` gained `Scope`, `RestoreFrom`, `Unchanged`,
`Parent`, `Move`, and `Staged`, and `undo` stopped using git's revert, which
needs a clean tree. The `config` operation kind went with the `mode` tool, so
`project.UpdateConfig` is the one writer of `project.json`. `git mv` needed both
paths resolved through symlinks. The Progress section has the detail.

Left undone: this repository has no page describing its work (the describe skill
writes it), and the plugin in Claude Code is still 3.0.0, so an existing session's
MCP server refuses the v4 layout until it is updated.
