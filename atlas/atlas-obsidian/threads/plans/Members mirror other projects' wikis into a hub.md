---
type: plan
thread: thr-20260921-2fec
title: "Members: mirror other projects' wikis into a hub"
created: 2026-09-21
---

> [!plan] Members: mirror other projects' wikis into a hub
> [Stub](<../stubs/Members mirror other projects' wikis into a hub.md>) → [Spec](<../specs/Members mirror other projects' wikis into a hub.md>) → **Plan** → [Receipt](<../receipts/Members mirror other projects' wikis into a hub.md>)
> `thr-20260921-2fec` · [Thread](<../archive/Members mirror other projects' wikis into a hub.md>) · filed 2026-09-21

## Approach

The mirror is one more code-owned operation kind. A new package `internal/mirror` computes the closure and the desired files, and hands them to `txn.Prepare` and `txn.Apply` as a request of kind `sync`, so the log entry, the inflight marker, the manual commit before it, `history`, and `undo` all come from what exists. Link rewriting reuses lint's resolver through one exported function, so the mirror and the health check agree on what a link means.

## Where the work lands

1. `internal/project`: `Config.Members`, the constants `MirrorDir`, `MirrorIndex`, `MaxMembers`.
2. `internal/registry`: `Entry.Members`, read by the scan.
3. `internal/lint`: export `LoadVault` and `Vault.Rewrite` over the existing resolver; treat pages under `wiki/projects/` as mirrored: their links are resolved and dead ones reported, but they take no part in wanted pages, duplicates, orphans, unindexed, frontmatter, sections, or stubs; a `mirror_errors` list from the marker's members against the generated index page.
4. `internal/mirror`: `Closure`, `Validate` (self, unknown, cycle, limits), `Plan` (walk, exclude, transform, index page, diff against the folder), `Sync` (plan, prepare, apply).
5. `internal/txn`: kind `Sync`; every other kind refuses `wiki/projects/`; sync skips the write limit, the frontmatter validation, and the lint warnings; the log entry for a sync carries counts, not pages.
6. `internal/hooks`: the guard refuses `wiki/projects/`; session start runs sync in a project with members and prints one line.
7. `internal/manage` and `internal/actions`: `Edit.AddMembers`, `Edit.RemoveMembers` resolved by name, id, or path and validated; `Atlas.Sync`.
8. `internal/cli`: `edit --add-member --remove-member`, a `sync [PROJECT]` command.
9. `internal/mcpserver`: `project` gains `add_members`, `remove_members`, and the action `sync`; `status` reports members and mirrors.
10. Skills, `docs/usage.md`, `CLAUDE.md`, `README.md`, version 5.3.0.

## Decisions taken while planning

- The index page records each member's commit and no time, so a sync with nothing new writes nothing on any day.
- Links are rewritten to the full vault path with the original link text as the alias, so Obsidian shows the page's name and resolves the path exactly.
- Lint reports what the folder shows: members with no mirror, mirrors with no member. Cycles need the atlas config, so `Validate` refuses them at edit and `sync` and `status` report one that arrived by hand.
- The mirror folder is named by the member's folder name, and two closure members with one folder name refuse the sync.

## Order

mirror package with tests; txn and lint; hooks; manage, actions, CLI, tools; docs and skills; version bump.

## Tests

`internal/mirror`: closure and validation, the transform per link form, index rename, canvas nodes, idempotent second run, deletion on removal, skip on a missing member. `internal/txn`: the kind's scope. `internal/hooks`: the guard. `internal/lint`: mirrored pages and mirror errors. `internal/mcpserver` and `internal/cli`: the edit and the sync end to end.

## Risks

Frontmatter edits must keep every existing line byte for byte; the helper touches only the three keys. A member with a page that has no frontmatter must still mirror, so sync skips the page validation other kinds get.

## Progress

Started 2026-09-21. Built the same day, in the order planned. Every package tests green, `make build` stamps 5.3.0, and a hand run of the binary over two services and a hub did what the design says: two syncs, the second a no-op; links rewritten to full vault paths; the cycle refused with the projects named; the guard denying an edit in a mirror; the session-start line naming the members. Not committed; the change is in the working tree for review.

Two decisions moved while building. The mirror's folder is derived from the member's name, as `Save` keeps it, so the closure is pure and a name that disagrees with its folder is reported when the member is read. And lint gained `preferNear`: a bare name now resolves to the pages nearest the link, the hub's own for a hub page and the same mirror for a mirrored page, because every mirror repeats the hub's basenames by design.
