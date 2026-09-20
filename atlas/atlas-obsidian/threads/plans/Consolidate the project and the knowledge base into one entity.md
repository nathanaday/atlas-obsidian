---
type: plan
thread: thr-20260919-26da
title: "Consolidate the project and the knowledge base into one entity"
created: 2026-09-19
---

> [!plan] Consolidate the project and the knowledge base into one entity
> [Stub](<../stubs/Consolidate the project and the knowledge base into one entity.md>) → [Spec](<../specs/Consolidate the project and the knowledge base into one entity.md>) → **Plan** → [Receipt](<../receipts/Consolidate the project and the knowledge base into one entity.md>)
> `thr-20260919-26da` · [Thread](<../archive/Consolidate the project and the knowledge base into one entity.md>) · filed 2026-09-19

Eleven steps, in the order of `docs/v4-design.md`. Each one builds and its package's tests pass before the next starts.

## Where the work lands

| Step | Files | What |
|---|---|---|
| 1 | `internal/project/` (from `internal/vault/`), delete `internal/vault/` | the entity: identity v4, layout, templates, `Init`, `Open`, `Locate`, `FindAbove`, `Save`, `EnsureFolders`, routing, page skeletons, the lock, the host repository, the engine scope |
| 2 | `internal/gitx/gitx.go` | `Repo.Scope []string`; `Status`, `Dirty`, `AddAll`, `LsFiles`, `Named`, `excluding` honor it |
| 3 | `internal/txn/`, `internal/capture/`, `internal/ledger/`, `internal/lint/`, `internal/describe/` | `*project.Project` in place of `*vault.Vault`; `describe` excludes `atlas/`; `capture` drops `via` |
| 4 | `internal/threads/` | the stage folders under `threads/`; the callout links; the board; the CSS snippet; `Migrate` |
| 5 | `internal/home/`, `internal/registry/`, `internal/refresh/`, `internal/vaults/` → `internal/manage/`, `internal/place/` | config v4, one kind, `ReasonV3Split`, one walk up |
| 6 | `internal/mcpserver/` | seventeen tools; `project` absorbs `vault` and `mode`; the `vault` argument becomes `project` |
| 7 | `internal/hooks/` | one session-start; the guard over `wiki/`, the cards, the board, and a new file in a stage folder |
| 8 | `internal/cli/` | the command table; `upgrade` with its four cases |
| 9 | `internal/tui/` | one list, five keys |
| 10 | `skills/`, `agents/`, `README.md`, `CLAUDE.md`, `docs/usage.md`, `.claude-plugin/` | the words, and 4.0.0 |
| 11 | — | `make test`, then this repository upgrades itself |

## Order and why

Steps 1 to 3 are one change in effect: nothing compiles between deleting
`internal/vault` and retargeting the engine, so they land together and
`make test` runs once at the end of step 3. Steps 4 and 5 are independent of
each other and both depend on 1. Steps 6 to 9 are the four callers and can
land in any order once 5 is done. Step 10 is prose and follows the code, so
no skill describes a tool that does not exist yet.

## How it is tested

- Every package keeps its own tests, retargeted. `internal/project` takes the
  union of `vault_test.go` and `project_test.go`.
- New: `gitx` proves a scoped `Status` and `AddAll` ignore a dirty file
  outside the scope. `txn` proves an apply in a work folder with uncommitted
  code and an edited thread document commits neither, and that `undo` leaves
  both alone.
- New: `upgrade` has one test per migration case, including the refusal, each
  over a temporary atlas with real git.
- `lint.TestNewVaultHasNoFindings` holds the merged template to a clean
  report.
- The MCP tools stay tested in-process over the in-memory transport; the
  count of tool names is asserted, so a forgotten tool fails the build.
- By hand at the end: this repository upgrades itself, a session starts in it,
  an ingest runs from the one inbox, a thread moves a stage, and `undo` takes
  the ingest back with the code untouched.

## Risks

- **The rename touches every package.** `*vault.Vault` appears in eight of
  them. Mechanical, but the compiler is the only check on the ones with thin
  tests (`internal/actions`, `internal/wizard`).
- **The git scope is the one subtle piece.** A pathspec that is too wide
  sweeps the user's code into a `manual` commit; one that is too narrow
  leaves a wiki file out of its own operation. The two new `txn` tests are
  the guard.
- **Migration moves the user's files.** `upgrade` prints its whole plan and
  asks before it moves anything, uses `git mv` inside one repository, and
  copies rather than moves across repositories, leaving the old folder on
  disk.
- **This repository is the first patient.** Its own threads and `atlas_kb`
  migrate by the same command a user would run, so a failure there is a
  failure found before release.

## Progress

### 2026-09-19 — the entity, the engine, and the atlas

Steps 1 to 5 landed together, because nothing compiles between deleting
`internal/vault` and retargeting the engine. `internal/vault` merged into
`internal/project`, which now owns the identity file (v4: no `kind`, no
`knowledge`, `scope` folded into `description`), the layout of both halves, the
merged template with one CSS snippet, and the engine's git scope.
`internal/vaults` became `internal/manage`. `registry`, `refresh`, and `place`
lost their second kind.

Two changes the design did not foresee:

- `gitx.Repo` gained `Scope`, a list of pathspecs under `Prefix`, and `AddAll`
  leaves out a scoped path that names nothing, because a project holds folders
  an operation may never write. `inbox/` had to join `EngineScope`: an ingest
  removes the sources it has filed.
- `undo` cannot use git's revert, which needs a clean working tree the user's
  code rarely has. It now reads the paths the operation changed, refuses when
  one of them changed since (`gitx.Unchanged`), and restores each from the
  commit's parent (`gitx.RestoreFrom`). This is a better undo than the old one:
  exact, and impossible to widen by accident.

The `config` operation kind went with the `mode` tool. `project.UpdateConfig`
is the one writer of `project.json`, and `txn.allowed` refuses that path for
every kind.

### 2026-09-19 — the surfaces

Seventeen tools: `vault` folded into `project`, `mode` into `project` as well,
and `route` keeps the read half. The hooks have one session-start form and a
guard that refuses the cards, the board, `project.json`, and a new file in a
stage folder, while a stage document's prose stays open to Edit. The CLI lost
`new-knowledge`, `adopt`, `link`, `unlink`, `remove`, and `mode`. The view is
one list of projects with a Problems tab while there is one.

`lint` runs over the project's folder and reads only `wiki/` as pages; a bare
name resolves to a wiki page first (`preferWiki`), because a thread's documents
repeat one file name by design.

### 2026-09-20 — the migration, and this repository

`atlas-obsidian upgrade` covers the four cases, with one test each. Three details
the design did not have:

- `git mv` needs both paths relative to the repository's own top with symlinks
  resolved, or the move silently falls back to a copy. On macOS `/var` and
  `/private/var` made every test copy instead of move.
- An empty folder `init` made is in the way of `git mv`; the move removes it
  first and refuses a destination that holds anything.
- A knowledge base absorbed from inside the work leaves only the atlas's own
  files behind, so `CleanAbsorbed` removes the folder and says so; a folder that
  still holds anything of the user's is named and left alone.

`make test` passes. Then the end to end, twice on a clone and once here: the
upgrade absorbed `atlas_kb` into `atlas/atlas-obsidian/`, moved the stage folders
under `threads/`, and committed the whole move as one `setup` commit, with
`git log --follow` still tracing a moved wiki page back through the knowledge
base's own history. With the code dirty and a thread document dirty, an `apply`
committed the wiki page alone and an `undo` took it back alone; both edits
survived. The session-start hook, the board, and `lint` all read the upgraded
project.

Left for the user: the plugin is still 3.0.0 in Claude Code, so this session's
MCP server refuses the v4 layout. Commit, then
`claude plugin marketplace update` and `claude plugin update`.
