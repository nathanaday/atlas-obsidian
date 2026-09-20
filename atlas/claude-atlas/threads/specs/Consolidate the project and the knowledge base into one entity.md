---
type: spec
thread: thr-20260919-26da
title: "Consolidate the project and the knowledge base into one entity"
created: 2026-09-19
---

> [!spec] Consolidate the project and the knowledge base into one entity
> [Stub](<../stubs/Consolidate the project and the knowledge base into one entity.md>) → **Spec** → [Plan](<../plans/Consolidate the project and the knowledge base into one entity.md>) → Receipt
> `thr-20260919-26da` · [Thread](<../Consolidate the project and the knowledge base into one entity.md>) · filed 2026-09-19

The design is `docs/v4-design.md`; it is the spec of record for this thread. The four decisions the user took on 2026-09-19: the entity is called a project, its wiki commits into the work's repository, one inbox, and the migration command ships with the change.

Done when:

- One entity. `atlas/<name>/` holds `project.json`, `wiki/`, `threads/`, `inbox/`, `ideas/`, `.raw/`, and one `.obsidian/`. `internal/vault` is gone, merged into `internal/project`.
- The thread folders (`stubs/`, `specs/`, `plans/`, `receipts/`, `phases/`) sit under `threads/` with the cards and the board. Every callout link, the CSS snippet, and the board agree.
- No linking anywhere: no `knowledge` field, no `link`, `unlink`, `new-knowledge`, `remove`, or `--no-knowledge`, no `KnowledgeError`, no second session kind.
- The engine's git commands are scoped to `wiki/`, `.raw/`, and `project.json`, so `apply` never commits the user's code or a thread document as a `manual` operation, and `undo` can never revert one.
- One inbox, with a per-item hint of source or note. `wiki-ingest` takes the sources, `thread-stub` takes the notes, and neither moves a file without showing what it will do.
- `scope` is folded into `description`; `mode` stays in `project.json`; the `vault` and `mode` tools fold into `project`, leaving seventeen.
- One session-start hook, one guard over the new paths, one list in the view.
- `claude-atlas upgrade` covers the four migration cases of the design and refuses, by name and with the command to run, where one knowledge base serves several projects. A 3.x project is refused with `ErrSplit` until it runs.
- `make test` passes, and this repository upgrades itself: `atlas_kb/` becomes `atlas/claude-atlas/wiki/`.
