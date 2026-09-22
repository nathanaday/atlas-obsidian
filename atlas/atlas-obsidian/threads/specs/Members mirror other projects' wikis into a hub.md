---
type: spec
thread: thr-20260921-2fec
title: "Members: mirror other projects' wikis into a hub"
created: 2026-09-21
---

> [!spec] Members: mirror other projects' wikis into a hub
> [Stub](<../stubs/Members mirror other projects' wikis into a hub.md>) → **Spec** → [Plan](<../plans/Members mirror other projects' wikis into a hub.md>) → [Receipt](<../receipts/Members mirror other projects' wikis into a hub.md>)
> `thr-20260921-2fec` · [Thread](<../archive/Members mirror other projects' wikis into a hub.md>) · filed 2026-09-21

## What will be true

The full design is `docs/members-design.md`. This spec states what is done when it is done.

1. `project.json` accepts `members`, a list of project ids. `project` (the tool and the CLI) can add and remove members. An edit that would list the project itself, close a cycle through other projects' member lists, name an id the atlas config does not list, or exceed 128 members is refused with a message and saves nothing.
2. `sync` (a command and a tool) rebuilds `wiki/projects/` in the session's project from the transitive closure of its members, flattened, each project once, self excluded. Each member's `wiki/` is copied to `wiki/projects/<name>/`, minus `log.md`, `hot.md`, `meta/`, and the member's own `projects/`. The member's `index.md` becomes `<name>.md`.
3. Every mirrored markdown page carries `project` (the member's id), `mirror_of` (the path inside the member's wiki), and `commit` when the member's engine has a HEAD, and keeps every other property. Every link the resolver can resolve inside the member's wiki is rewritten to a full vault path under `projects/<name>/`; links into `projects/` and links that do not resolve stay as written. A `.canvas` file gets the same rewrite on its file nodes.
4. `wiki/projects/projects.md` is generated: one row per mirrored project with name, id, commit, sync time, and a link to the mirror's root page.
5. Sync writes only files whose bytes differ, deletes files under `wiki/projects/` that the closure no longer produces, and commits once as a `sync` operation through the engine. A second sync with nothing changed commits nothing. `history` shows a sync; `undo` restores the previous mirror.
6. A member the config cannot find, or whose wiki cannot be read, is skipped and reported; its old mirror stays. A closure over 128 projects, or two projects in it with the same folder name, refuses the whole sync.
7. The guard refuses Write and Edit under `wiki/projects/`, and `plan` refuses a path there for every kind.
8. Lint reads the folder alone and reports what it shows: a member listed whose mirror is absent (sync has not run), and a mirror folder with no root page. A cycle needs the atlas config, so `sync` and `status` report one that arrived by hand. Mirrored pages' dead links are reported as any other; every other finding about a mirrored page is the member's, and lint makes none.
9. The session-start hook runs sync in a project that lists members, after `threads.Sync`, and its context line names the mirrored projects.
10. `status` reports the members and the mirrors. The atlas-project skill documents adding and removing members and running sync; the wiki-query skill notes that a hub's wiki includes its mirrors.
11. Tests cover: config validation (self, cycle, unknown id, limit), the transform (frontmatter, each link form, index rename, canvas nodes), the reconcile loop (idempotent second run, deletion on member removal, skip on missing member), the guard, and the tools in-process.
12. `docs/usage.md` maps the new command and tool; `CLAUDE.md` gains the design in its sources-of-truth table and the mirror rule under the constraints.

## Out of scope

Copying captured sources or the ledger into the hub. A "cited by" callout on member pages. Any write into a member.
