---
type: plan
thread: thr-20260922-370b
title: "Threads follow the members graph, and become an opt-in"
created: 2026-09-22
---

> [!plan] Threads follow the members graph, and become an opt-in
> [Stub](<../stubs/Threads follow the members graph, and become an opt-in.md>) → [Spec](<../specs/Threads follow the members graph, and become an opt-in.md>) → **Plan** → Receipt
> `thr-20260922-370b` · [Thread](<../Threads follow the members graph, and become an opt-in.md>) · filed 2026-09-22

## Approach

Four slices, each a commit that builds and tests green. The thread mirror reuses the wiki mirror's closure, transform, and reconcile; the opt-in is one flag that `threads.Load` reads, so every reader and writer of threads refuses in one place.

## Where the work lands

### Slice 1: the opt-in

- `internal/project/project.go`: `Config.Threads bool` (`json:"threads,omitempty"`). `Folders` keeps `wiki`, `inbox`, `ideas`; `ThreadFolders` holds the seven thread folders. `EnsureFolders` makes the thread folders only when `Config.Threads`.
- `internal/project/init.go`: `Options.NoThreads bool`; `newConfig` sets `Threads: !opts.NoThreads`, so every existing caller and test keeps threads on.
- `internal/threads/threads.go`: `ErrOff`, returned by `Load` when `p.Config.Threads` is false. `Start`, `File`, `Set`, `Reopen`, `Sync`, and the phase functions go through `Load`, so they refuse for free. `Touch` returns nil when off.
- `internal/manage/manage.go`: `Edit.Threads *bool`; `Fields` names it; `EditProject` sets it.
- `internal/cli/cli.go`: `init --no-threads`; `edit --threads on|off`; `show` rows "Threads: on/off"; the `threads` command prints the `ErrOff` message plainly.
- `internal/wizard`: one yes/no question, "Track threads in this project?", default yes.
- `internal/actions/actions.go`: `InitProject.NoThreads`.
- `internal/mcpserver/atlas.go`: `ProjectToolArgs.Threads *bool` for init and edit.
- `internal/registry/registry.go`: `Entry.Threads bool` from the config, so the view and `list` can say off.
- `internal/hooks/hooks.go`: `threadLines` prints "Threads: off in this project; the project tool turns them on (threads: true)." when `Load` returns `ErrOff`; `Touched` ignores the error.
- `internal/tui/view.go`: `n` on a project with threads off shows the error the action returns; no other change.
- Tests: `project` (folders), `threads` (`ErrOff` from every function), `manage` (edit), `cli` (init and edit flags), `hooks` (the off line), `mcpserver` (the tool refuses with the call to make).

### Slice 2: the thread mirror

- `internal/project/project.go`: `ThreadMirrorDir = "threads/projects"`.
- `internal/threads/threads.go`: `Mirrored(rel) bool`.
- `internal/mirror/mirror.go`:
  - `destination(folder, rel)` maps `threads/**` (minus `threads/projects/**`) to `threads/projects/<folder>/**` and keeps its wiki mapping. One function, both halves.
  - `mirrorMember` walks the whole member folder (`lint.LoadVault` already indexes it) and fills two desired sets: wiki files and thread files. Thread files are stamped with `project` and `mirror_of` only. Threads are mirrored only when the hub and the member both have threads on.
  - `existing` reads both mirror folders; `keep` applies to both by folder name.
  - `reconcile(existing, desired, keep)` returns creates, updates, and removes for one half. Build calls it twice.
  - `Plan` gains the thread writes; `Sync` applies the wiki half through `txn` as today, then writes the thread half directly (`writeIfChanged`, delete) and prunes empty folders in both.
  - `SyncThreads(p, ix, now)` applies the thread half alone, for the thread tools to call after a write.
  - `Member.Threads int` counts the thread pages mirrored; `Result` counts stay totals over both halves.
- `internal/threads/render.go`: `Sync` lists the mirror folders under `threads/projects/` that hold a `threads.md`, and `RenderIndex` ends with one `## <folder>` section per mirror embedding `threads/projects/<folder>/threads`.
- `internal/hooks/hooks.go`: `guardPath` refuses `threads.Mirrored(rel)` first, naming `mirror_of` and `project` from the page's frontmatter and the folder; `membersLine` reports thread pages beside wiki pages.
- Tests in `mirror_test.go`: a hub with two members, one with threads off; embeds and relative links resolve in the hub; a member's wiki page that cites a thread document resolves; idempotent; a member leaving the closure removes its thread mirror; a stray file under `threads/projects/` is removed; a hub with threads off mirrors no threads. `hooks_test.go`: the guard refusal.

### Slice 3: routing

- `internal/mirror/find.go`: `FindThread(p, ix, key) (*project.Project, *threads.Thread, error)`: resolves in `p` first; when `p` has members and the key is not found there, loads each readable member with threads on and resolves; one match wins, more refuse with the ids, none reports "no thread in <p> or its members".
- `internal/mcpserver/server.go`: `thread` without `project` calls `FindThread`; the operation runs in the project returned. After any `thread` or `phase` write, when the session's project has members and threads on, `mirror.SyncThreads` runs. `threads` in a hub lists the hub's board then each member's (live), so the session sees the ids.
- `internal/cli/cli.go`: `thread PROJECT file|set|close|reopen ID` uses `FindThread` too, so the CLI and the tool agree.
- Tests: `mcpserver` in-process over two projects; `mirror` for `FindThread`.

### Slice 4: docs, skills, version

- `docs/members-design.md`: a section "Threads cross projects" with the layout, the transform, routing, and the decisions. `docs/threads-design.md`: the opt-in.
- `CLAUDE.md`: rules 3 and 4 and the constraints mention `threads/projects/` and the flag.
- `README.md`, `docs/usage.md`: the flags and the folder.
- `skills/wiki/references/threads.md`, `skills/thread/SKILL.md`, `skills/thread-stub/SKILL.md`: the mirror, and that a member's thread changes through the tool wherever the session is.
- `hooks` session-start text.
- Version 5.4.0 in both plugin manifests and the Codex manifest.

## Order

1, 2, 3, 4. Slice 2 is the largest; slice 3 depends on it.

## Testing

`make test` after each slice. End to end by hand: make a temp hub with this repository's project as a member, `sync`, open the hub in Obsidian, file a stage on one of this project's threads from a session in the hub, and see it land here and in the mirror.

## Risks

- `lint.Vault.Rewrite` on a markdown link writes a vault-absolute path; a thread document's `<../stubs/X.md>` link would become `threads/projects/f/stubs/X.md`. Obsidian resolves that; other viewers do not. If it reads badly in practice, leave relative markdown links inside `threads/` untouched, since the folder layout under a mirror is the member's own.
- Existing projects on this machine have `threads` absent, so they read as off until `edit --threads on` runs once each.
- The thread half writes after the wiki half commits; a failure between leaves the mirror behind by one sync, which the next sync repairs.

## Progress

Done 2026-09-22, four commits, one per slice, then the hook line for the
members' open threads. Every test passes. By hand: a temporary hub with this
project as its member synced 50 thread pages, its board embedded this
project's board, `thread hub-test show navigate` and `set` routed to this
project and re-synced the hub, the hub's session hook reported the member's
threads, and lint ran clean. The hub was forgotten and removed afterwards.

Deviations from the plan:

- A stray file in a mirror is removed at the next sync, not reported beside
  the board; the spec's item 7 was corrected to say so.
- The session hook gained a line counting the members' open threads, because
  a hub with no threads of its own said "Open threads: none" while its board
  carried three.
- This project had `threads` absent, so `edit atlas-obsidian --threads on`
  ran once (commit `setup: edit threads`). The other 16 projects on this
  machine read as off until the same runs in each.
