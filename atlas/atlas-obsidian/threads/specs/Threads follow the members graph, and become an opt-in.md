---
type: spec
thread: thr-20260922-370b
title: "Threads follow the members graph, and become an opt-in"
created: 2026-09-22
---

> [!spec] Threads follow the members graph, and become an opt-in
> [Stub](<../stubs/Threads follow the members graph, and become an opt-in.md>) → **Spec** → [Plan](<../plans/Threads follow the members graph, and become an opt-in.md>) → [Receipt](<../receipts/Threads follow the members graph, and become an opt-in.md>)
> `thr-20260922-370b` · [Thread](<../archive/Threads follow the members graph, and become an opt-in.md>) · filed 2026-09-22

## What will be true

1. **Threads are an opt-in.** `project.json` gains `"threads": true`. A project without it has no thread folders, no thread lines in the session-start hook, no thread counts in `status`, the view, or the registry, and the `thread`, `threads`, and `phase` tools refuse with the call that turns threads on. Turning threads off later changes no file: the folder stays, and the pages go inert. `init` and the wizard offer threads (default on); `edit --threads on|off` and the `project` tool flip the flag.

2. **A hub mirrors its members' threads.** A project with members and threads on holds `threads/projects/<name>/` per member in its closure that has threads on: the member's cards, `archive/`, the four stage folders, `phases/`, and its board, transformed. The closure is the one the wiki mirror uses: flat, each project once, self excluded. A member with threads off contributes nothing. A member that cannot be read keeps its old mirror. Code owns `threads/projects/`; the guard refuses Write and Edit there, and the refusal names the owning project.

3. **One sync, both halves.** `sync` (the command, the `project` tool's action, and the session-start hook) mirrors the wiki and the threads in one pass over one closure. The wiki half commits as one `sync` operation through the engine, as today. The thread half writes plain files, because the threads have no engine, and it reconciles the same way `threads.Sync` does: write on difference, delete on absence, remove a member's folder when it leaves the closure.

4. **The transform is shared.** Sync loads each member's folder as one vault and rewrites every resolvable link with one `destination` function that knows both halves: a link into `wiki/` lands under `wiki/projects/<name>/`, a link into `threads/` lands under `threads/projects/<name>/`. So a member's wiki page that cites a thread document, or a thread document that cites a wiki page, still resolves in the hub. Every mirrored page carries `project` (the member's id) and `mirror_of` (its path in the member). A thread document's relative links stay as written, because the folder layout under the mirror is the member's own.

5. **The hub's board shows everything.** `threads/threads.md` in a hub ends with one section per mirrored member that embeds the member's mirrored board. A session in the hub reads one page for the whole ecosystem.

6. **A thread's writes go to its owner.** The `thread` tool, given an id or a title it cannot resolve in the session's project, looks in the boards of the projects the session's project mirrors. It resolves the thread there, runs the operation in that project, and re-syncs the session's thread mirror. A title that matches in more than one project is refused with the ids. The `project` argument keeps working, and it too re-syncs after the write. A thread the hub owns is an ecosystem-wide thread; nothing new is needed for it.

7. **A hand-made file in a mirror is removed, not moved.** A new page under `threads/projects/` that sync did not write is deleted at the next sync, the same way a stray file under `wiki/projects/` is; the guard refuses the write in a session, so it is rare. There is no reverse sync.

## Why

The wiki mirror and the thread sync are the same loop: compute a desired folder, make the disk match. Threads cross projects by pointing that loop at `threads/`. Routing writes to the owner keeps one writer per folder, which is the rule every derived folder in the project follows. The opt-in is the first step toward a project that holds only the models its user wants, without a schema format yet.

## Not in this thread

- A user-defined entity: its own stages, fields, templates, or skills.
- A second built-in entity.
- A reverse sync from a hub into a member.
- A per-member index page under `threads/projects/`; the hub's board is the index.
- The atlas view: it shows the hub's own counts, not its members'.

## Decisions taken while writing this

| Question | Decision |
|---|---|
| The opt-in key | `threads: true` in `project.json`, absent means off. A future entity adds its own key |
| Existing projects | need `edit --threads on` once; nothing converts a file |
| Where the hub reads member boards for routing | the members' working trees, live, as sync does; never the mirror |
| How the hub board shows members | an embed of each mirrored board, so the board needs no member present to render |
| The thread mirror's writer | direct file writes, reconciled like `threads.Sync`; not the engine |
