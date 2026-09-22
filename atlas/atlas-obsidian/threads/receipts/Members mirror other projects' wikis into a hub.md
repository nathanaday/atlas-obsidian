---
type: receipt
thread: thr-20260921-2fec
title: "Members: mirror other projects' wikis into a hub"
outcome: completed
created: 2026-09-21
---

> [!receipt] Members: mirror other projects' wikis into a hub · completed
> [Stub](<../stubs/Members mirror other projects' wikis into a hub.md>) → [Spec](<../specs/Members mirror other projects' wikis into a hub.md>) → [Plan](<../plans/Members mirror other projects' wikis into a hub.md>) → **Receipt**
> `thr-20260921-2fec` · [Thread](<../archive/Members mirror other projects' wikis into a hub.md>) · filed 2026-09-21

## Delivered

- `docs/members-design.md`: the design, its principles, the mechanism, and what was accepted and left.
- `internal/mirror`: `Closure`, `Validate`, `Build`, `Sync`; the transform stamps `project`, `mirror_of`, `commit` and rewrites links and canvas nodes; the index page `wiki/projects/projects.md`.
- `internal/lint`: `LoadVault` and `Vault.Rewrite` over the existing resolver; mirrored pages are link targets whose dead links are reported and about which no other finding is made; `mirror_errors`; `preferNear`.
- `internal/txn`: the `sync` kind, confined to `wiki/projects/`, which every other kind now refuses; no write limit, no page validation, no lint warnings, and a counted log entry for a sync.
- `project.Config.Members`, `registry.Entry.Members`, `manage.Edit.AddMembers` and `RemoveMembers` resolved by name, id, or path and validated, `actions.Atlas.Sync`.
- The guard refuses `wiki/projects/`; session start syncs a project with members and prints one line; `describe` skips the mirrors.
- CLI: `edit --add-member --remove-member`, `sync [PROJECT]`, a members row in `show`. Tools: `project` with `add_members`, `remove_members`, and the action `sync`; `status` lists the members and warns when a sync is due.
- Skills `atlas-project`, `wiki-query`, `atlas`, `wiki`; `docs/usage.md`; `README.md`; `CLAUDE.md`; version 5.3.0 in both plugin manifests and the marketplace.

## Verified

`go vet ./...` and `go test ./...` pass. New tests: `internal/mirror` (closure, cycles, limits, folder collisions, the stamp, the whole sync with idempotence, update, removal, undo, a missing member kept, no members), `internal/lint` (mirrored pages, mirror errors, `Vault.Rewrite` over every link form), `internal/txn` (the kind's scope, a sync of bare pages past the write limit), `internal/hooks` (the guard, a session start that syncs), `internal/mcpserver` (edit, validation, status, sync, plan refused into the mirror, remove), `internal/cli` (edit and sync). A hand run of the built binary over a hub and two services matched the design.

## Not done

Nothing in scope. The change is uncommitted, for review. Lint does not detect a cycle; sync and status do, since a cycle needs the atlas config.
