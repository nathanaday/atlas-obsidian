---
type: receipt
thread: thr-20260922-370b
title: "Threads follow the members graph, and become an opt-in"
outcome: completed
created: 2026-09-22
---

> [!receipt] Threads follow the members graph, and become an opt-in · completed
> [Stub](<../stubs/Threads follow the members graph, and become an opt-in.md>) → [Spec](<../specs/Threads follow the members graph, and become an opt-in.md>) → [Plan](<../plans/Threads follow the members graph, and become an opt-in.md>) → **Receipt**
> `thr-20260922-370b` · [Thread](<../archive/Threads follow the members graph, and become an opt-in.md>) · filed 2026-09-22

Delivered in 5.4.0, six commits on main (78b177b..b2a974e):

- Threads are an opt-in: `threads: true` in `project.json`; off, every thread function returns `threads.ErrOff`, the tools refuse with the call that turns them on, the hook says so, and no thread folder is made. `init --no-threads`, `edit --threads on|off`, and the `project` tool's `threads` set it.
- A hub mirrors its members' threads under `threads/projects/<name>/` in the same sync as the wikis: one closure, one transform (`mirror.destination` covers both halves, so links between a wiki page and a thread document resolve in the hub), one reconcile. The thread half is plain files; the hub's board embeds each mirrored board; the guard refuses the mirror and names the owner.
- A thread's writes go to its owner: `mirror.FindThread`, used by the `thread` tool and the CLI's `thread` command, then `mirror.SyncThreads`. `threads` in a hub lists the member boards after its own. The session hook counts the members' open threads.

Verified: `go test ./...` green in every package, with new tests for the opt-in (project, threads, manage, cli, hooks, mcpserver), the mirror (both halves, idempotence, a member leaving, a stray file, a hub with threads off), and routing (mirror, mcpserver, cli). By hand: a temporary hub with this project as its member synced 50 thread pages, a `set` from the hub landed here and re-synced the hub, the hook reported the member's threads, lint ran clean.

Left for later: a user-defined entity with its own stages, fields, templates, and skills; a second built-in entity to prove the seam; the atlas view showing members' counts. The other projects on this machine read as threads-off until `edit NAME --threads on` runs in each.
