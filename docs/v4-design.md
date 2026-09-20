# Atlas v4: one project, with its wiki inside it

Status: designed and built 2026-09-19. The plugin and the binary are 4.0.0.

This document replaces the two-entity model of `v3-design.md`: the separate
knowledge base, `link`, `unlink`, and every rule that begins "a project uses
one knowledge base". The engine rules of `core-design.md` stand, and so does
the thread model of `threads-design.md`, with its folders moved under
`threads/`.

## What changes and why

A week of use on real projects found the split between a project and a
knowledge base to be the heaviest thing left in the design.

- **Two vaults per project, and a question every time.** The threads are in
  the work, the wiki is somewhere else, and often not in the same parent
  folder. Before opening Obsidian the user has to decide which vault holds
  what they want. The author of the tool hesitated at that decision; a new
  user has no chance.
- **Two inboxes.** A file dropped in the wrong one does nothing.
- **A link to maintain.** `init --knowledge`, `link`, `unlink`, a knowledge
  base id inside `project.json`, a knowledge base that is not on this
  machine, and a session that has to report all of it.
- **The sharing never happened.** One knowledge base serving many projects
  was the reason for the split. In practice each project's knowledge was
  about that project.

Sharing knowledge between projects stays the mission. It returns as its own
feature, built over projects that each hold their own wiki, and it is out of
scope here. See "Left for later".

v4 has one entity.

| Thing | v3 | v4 |
|---|---|---|
| Project | `atlas/<name>/` in the work: threads, no engine | `atlas/<name>/` in the work: threads **and** the wiki, with the engine |
| Knowledge base | a separate Obsidian vault, linked by id | the `wiki/` folder of a project |
| `link`, `unlink`, `--knowledge`, `--no-knowledge` | tools, CLI flags, skills | gone |
| Two inboxes | one for sources, one for thread notes | one `inbox/` |
| Two session kinds | project, knowledge base | one |
| Two tabs in the view | Knowledge, Projects | one list |

## The project

One folder in the work holds everything the atlas knows about it:

```text
webapp/                              a repository, or a folder of documents
├── ...                              the user's work, untouched
└── atlas/
    └── webapp/                      the project; open this in Obsidian
        ├── project.json             identity
        ├── wiki/                    the knowledge base
        │   ├── index.md  log.md  hot.md  overview.md
        │   ├── sources/  entities/  concepts/  canvases/
        │   └── meta/ledgers/source-ledger.json
        ├── threads/                 the state of the work
        │   ├── threads.md           the board, generated
        │   ├── <Title>.md           the card of an open thread, generated
        │   ├── archive/<Title>.md   the card of a closed thread
        │   ├── stubs/  specs/  plans/  receipts/
        │   └── phases/
        ├── inbox/                   anything the user drops in
        ├── ideas/                   scratch; nothing reads it
        ├── .raw/captured/<sha256>.ext   immutable copies of ingested sources
        ├── .vault-meta/             runtime, git-ignored
        └── .obsidian/               the wiki and stage snippets
```

Four visible folders at the top: `wiki/`, `threads/`, `inbox/`, `ideas/`.
The thread folders move under `threads/`, so opening the vault shows the
halves and not nine siblings. `phases/` goes with them, because a phase
groups threads.

### The identity file

`atlas/<name>/project.json`, schema `claude-atlas.project.v4`:

```json
{
  "schema": "claude-atlas.project.v4",
  "id": "b3e0f5a2-9c14-4d6e-8a7b-2f1e0c9d8b7a",
  "name": "webapp",
  "description": "The customer-facing web application for the fire-detection product. Its wiki holds the alarm pipeline, the false-alarm sources, and the field tests they come from.",
  "mode": "generic",
  "created": "2026-09-19"
}
```

- `knowledge` is gone. `kind` is gone; there is one kind.
- `mode` comes from the knowledge base's identity file and keeps its meaning:
  `generic` files a new page by type, `lyt` keeps atomic notes and Maps of
  Content.
- `scope` is gone, folded into `description`. Two prose fields close in
  meaning made the user answer the same question twice. One field says what
  the work is and what its wiki should remember; `wiki-ingest` and
  `wiki-query` read it where they read the scope today. `upgrade` joins the
  project's description and the knowledge base's scope into it, so no text
  is lost.
- The file holds no path, as before. The atlas config holds every path.
- The project folder is `links.CleanName` of the name. A rename moves the
  folder and refuses when the new one is taken, as in 3.0.0.

### Two halves, one folder

The line between the halves is not the folder; it is who may write.

| Half | Paths | Who writes | Safety |
|---|---|---|---|
| The wiki | `wiki/`, `.raw/`, the source ledger, `project.json` | `plan` then `apply`, and `capture`; the guard refuses Write and Edit | one operation, one commit; `undo`; `lint` |
| The work state | `threads/` | the `thread` and `phase` tools write the cards, the frontmatter, the callouts, and the board; the model writes the prose of a document with Edit | the guard refuses the cards, the board, and a new file in a stage folder |
| Neither | `inbox/`, `ideas/`, `.obsidian/` | the user; `stage` and `capture` for `inbox/` | none needed |

`txn.allowed` already confines every operation kind to `wiki/`, so the
engine's write scope needs no change. What changes is git.

### Git

The project's wiki commits into the repository that holds the work, on the
branch the user is on. That is what v3 already did for a knowledge base
inside a project, and now it is the only case.

- The engine's git commands are scoped to the paths the engine owns:
  `atlas/<name>/wiki/`, `atlas/<name>/.raw/`, `atlas/<name>/project.json`.
  `gitx.Repo` gains a `Scope` of pathspecs under its prefix.
- The consequence: `apply`'s first step, which commits hand edits as a
  `manual` operation, sees only the wiki. The user's code and the thread
  documents `thread-run` is in the middle of writing are never swept into a
  `manual` commit, and `undo` can never revert them.
- Wiki commits and code commits interleave in one history. A page ingested
  on a feature branch reaches `main` when the branch does. The project's
  knowledge travels with its code: it clones with it, pushes with it, and a
  teammate gets it.
- `init` runs `git init` in a work folder that is in no repository, as in
  2.1.0. Nothing nests a repository inside another.
- `describe` counts how far the work moved by excluding `atlas/` from the
  log, in the place where it excluded the knowledge base.

### The inbox

One `inbox/`. The user drops a paper, a note to self, a CSV, a screenshot.
The `inbox` tool lists each item with its size, its kind, whether it is
captured already, and what it looks like: a **source** to ingest or a
**note** to open as a thread. The rule is the file, not a folder:

- A document (`.pdf`, `.epub`, `.csv`, a long `.md`, an image, an archive)
  is a source. `wiki-ingest` captures it and writes cited pages.
- A short `.md` or `.txt` note is a thread note. `thread-stub` opens a
  thread from it and removes the note.

Both skills show what they will do before they do it, so a wrong guess costs
one word from the user. The guess is a hint in the tool's output, never a
rule in code that moves a file on its own.

## Sessions

One kind of session. `place.Resolve` walks up from the working directory to
the nearest `atlas/<name>/project.json`, after an explicit path and
`CLAUDE_ATLAS_VAULT`. There is no second marker to compare distances with,
no `KnowledgeError`, and no knowledge base to resolve through the registry.
A session anywhere inside the work is the project's session.

The session-start hook:

```text
claude-atlas: project webapp at ~/code/webapp (git, main)
Wiki: 140 pages · generic · last operation ingest 2026-09-18
Search the wiki (the wiki-query skill) before answering from the code alone.
Change wiki pages only through plan, then apply.
Open threads: 4 (plan 1, spec 1, stub 2) in 2 phases.
- [plan] Filter vehicle false alarms (thr-20260917-3f2a) · high · Alarm quality · updated 2026-09-18
- …
Inbox: 2 sources, 1 note.
<vault-context> hot.md </vault-context>
```

`hot.md` returns to every session, project or not, because there is only one
kind now. The line "this project has no page in the wiki; the describe skill
writes it" stays: a project still describes its own work in its own wiki.

## The atlas

The atlas is the list of projects on the machine and nothing else.
`config.json`, schema `claude-atlas.config.v4`:

```json
{
  "schema": "claude-atlas.config.v4",
  "projects": ["~/code/webapp", "~/code/fw", "~/Documents/thesis"],
  "plugin": {}, "claude_code": {}, "heat": {}
}
```

- `knowledge` is gone. A v3 config loads and every path in `knowledge` is
  reported by `doctor` as a knowledge base that `upgrade` has not absorbed
  yet.
- `registry.Scan` reads `atlas/<name>/project.json` under every path in
  `projects` and nothing else. One kind, so `registry.Kind`,
  `ReasonNotVault`, `ReasonV2Project`, and `ReasonNotProject` collapse to
  `ReasonUnreadable`, `ReasonSchema`, `ReasonMissing`, `ReasonFlat`, and
  `ReasonV3Split`, the new code for a v3 project that still names a
  knowledge base.
- An entry carries the project's path, its name, its description, its mode,
  its page count, its inbox counts, its thread counts by stage, its phases,
  its last operation, and whether it has a page for itself.
- Ids still travel and paths still stay. A session in a moved or cloned
  project heals its own entry by id (`manage.Register`).

### The view

One list, one row per project, sorted by heat. A row shows the name, the
path, the page count, the open thread count with its furthest stage, the
inbox count, and a mark when the path is gone. Enter expands the row in
place with what `show` prints. Problems are a section at the end.

| Key | Command |
|---|---|
| Enter | `show NAME` |
| `o` | `open-vault NAME`, Obsidian on `atlas/<name>/` |
| `c` | `open-claude NAME`, Claude Code in the work folder |
| `n` | a new thread from one line |
| `R` | `refresh` |
| `q` | quit |

The Knowledge tab, the Projects tab, and the tab keys go. The view is one
screen again.

## Tools

Seventeen, down from twenty.

| Tool | Change |
|---|---|
| `status` | one shape: the project, its wiki, its threads, its inbox, git, versions |
| `inbox` | one inbox; each item carries a hint, source or note |
| `capture`, `plan`, `apply`, `undo`, `history`, `route`, `lint`, `stub` | unchanged, against the project's wiki; the optional `vault` argument becomes `project` |
| `threads`, `thread`, `phase` | unchanged but for the moved folders; `threads` no longer reports across the projects of a knowledge base |
| `stage` | stages files, or the project's own snapshot, into `inbox/` |
| `project` | absorbs `vault` and `mode`: init here, rename, edit the description, set the mode, forget, describe |
| `atlas` | every project with its state; `refresh` |
| `settings` | unchanged |
| `vault`, `mode` | removed |

`project` takes the write half of what `vault` did, because there is one
entity to create and edit. `mode` folds into it because a mode is one more
field of the identity file; `route` keeps the read half, which is where a
new page of a type lands.

## The CLI

| Command | Does |
|---|---|
| `init [PATH] [--name] [--description] [--mode] [--no-git]` | make a folder a project: `atlas/<name>/` with its wiki, its threads, and a first commit |
| `adopt PATH` | make an existing Obsidian or claude-obsidian vault a project: move its `wiki/`, `inbox/`, `ideas/`, and `.raw/` into `atlas/<name>/` |
| `upgrade [NAME \| --all] [--absorb PATH]` | 3.x to 4.0: absorb the knowledge base, move the thread folders under `threads/` |
| `edit NAME [--name] [--description] [--mode]` | the identity file, as one `config` operation |
| `forget NAME` | drop it from the config; the folder stays |
| `describe NAME` | stage the snapshot; the skill writes the page |
| `threads`, `thread`, `phase` | unchanged |
| `ingest NAME [PATH…]` | stage files into the project's inbox |
| `lint`, `stub`, `history`, `undo`, `recover`, `apply` | unchanged, against a project |
| `list`, `show NAME`, `refresh`, `doctor`, `config`, `info`, `version` | the atlas |
| `open-vault NAME`, `open-claude NAME [--thread ID]`, `view` | launching |
| `setup`, `mcp`, `hook` | unchanged |
| `new-knowledge`, `link`, `unlink`, `remove`, `mode` | removed; `init`, `edit`, and `forget` cover them |

`new-knowledge` goes because `init` is the one way in. `remove` and `forget`
were the same command for two entities; one stays, named `forget`. `mode`
becomes `edit --mode`.

## Skills

| Skill | Change |
|---|---|
| `wiki` | routes the knowledge half of one project; setup points at `init` |
| `wiki-ingest` | one inbox: it ingests the sources and leaves the notes for `thread-stub`; the agent's brief carries the project, and `via` is gone from capture |
| `wiki-query`, `save`, `wiki-lint`, `wiki-fold`, `canvas`, `obsidian-bases`, `obsidian-markdown`, `think` | unchanged in substance; one place to write to |
| `wiki-mode` | reads and sets the mode through `project` |
| `thread`, `thread-stub`, `thread-spec`, `thread-plan`, `thread-run`, `thread-receipt` | unchanged but for the folders; `thread-receipt` offers the project's own wiki what the work taught, with no other project to consider |
| `describe` | unchanged: the project's page in its own wiki, under `wiki/entities/` |
| `atlas` | every project on the machine, the problems, the settings |
| `atlas-project` | absorbs `atlas-knowledge`: init here, rename, edit the description, set the mode, adopt a vault, forget, describe |
| `atlas-knowledge` | removed |
| `work` | one sentence to a thread with a plan in this project, then the work. The cross-project road goes with the split |

## Migration

`claude-atlas upgrade` is one command with one report and no surprises. It
refuses rather than guesses, and it moves files with `git mv` when one
repository holds both sides, so the history follows.

Four cases:

1. **The knowledge base is inside the work.** `git mv` its `wiki/`,
   `.raw/`, `inbox/`, and `ideas/` into `atlas/<name>/`, join its scope into
   the project's description, take its mode, delete its identity file, and
   drop its entry from the config. Its folder is removed when nothing but the
   atlas's own files is left in it, and named when anything of the user's is.
2. **The knowledge base is elsewhere and serves this project alone.** The
   same, by copy, then the old folder is left on disk untouched and named in
   the report. The user deletes it.
3. **One knowledge base serves several projects.** Refused, with the list of
   projects. `upgrade NAME --absorb PATH` absorbs it into the one project the
   user names, by copy; the others get an empty wiki and keep their threads.
   Nothing is deleted.
4. **A knowledge base no project uses.** `upgrade` makes its folder a
   project: `atlas/<name>/` inside it, with `wiki/`, `inbox/`, `ideas/`, and
   `.raw/` moved in by `git mv`, and the scope as the description.

In every case the thread folders move under `threads/`, every document's
first callout and every card is rewritten by `Sync`, and the CSS snippet is
rewritten for the new folder colors.

A project the user has not upgraded is refused by `project.Open` with
`ErrSplit`, scanned as `ReasonV3Split`, and named by the hook, `status`, and
`doctor` with the command to run. Nothing moves the user's files without it.

## Rules

The three sets of rules become one set of five. A project has two halves, and
the halves have different rules, which is the point of the line between them.

1. **One operation, one commit, in the wiki.** `plan` validates, the user
   sees the preview, `apply` commits. There is no other write path into
   `wiki/`; the guard refuses Write and Edit there. The engine's git commands
   are scoped to the paths it owns, so an operation and an undo never touch
   the work.
2. **The folder is the user's.** Apply commits hand edits in the wiki as
   `manual` operations first, so a rollback never touches what the user typed
   in Obsidian. Every other folder is the user's outright.
3. **A thread's stage is never set.** It is the furthest document that
   exists, and the `thread` tool filing a document is the only way a thread
   moves. A receipt closes it. A thread names its phase; a phase never lists
   its threads.
4. **Code owns what code can derive.** `wiki/log.md`, the source ledger, the
   thread cards, the first callout of every document, and the board. The
   model writes prose; it never targets a derived page.
5. **Ids travel; paths stay.** `project.json` holds no path. The config holds
   every project's work folder and nothing else. A session heals its own
   entry by id. `state/registry.json` is derived, and `refresh` rebuilds it.

## Build order

1. **The entity.** Merge `internal/vault` into `internal/project`: the v4
   identity file, the layout with `wiki/` and `threads/`, the merged
   `.obsidian/` template, `Init`, `Open`, `Locate`, `FindAbove`, `Save`,
   `EnsureFolders`, mode routing, page skeletons, the lock, the host
   repository, and the engine's scope. Delete `internal/vault`.
2. **Git scope.** `gitx.Repo` gains `Scope []string`; `Status`, `Dirty`,
   `AddAll`, `LsFiles`, and `Named` honor it.
3. **The engine on a project.** `txn`, `capture`, `ledger`, `lint`, and
   `describe` take a `*project.Project`. `describe` excludes `atlas/`.
4. **Threads under `threads/`.** The folder constants, the relative links in
   every callout, the board, the CSS snippet, and `Migrate`.
5. **The atlas.** `config.v4` with `projects` alone; `registry` with one
   kind and `ReasonV3Split`; `refresh`; `internal/vaults` becomes
   `internal/manage` with one entity's create, adopt, edit, register, heal,
   and forget; `place` with one walk up.
6. **The server.** Seventeen tools; `project` absorbs `vault` and `mode`;
   every tool's `vault` argument becomes `project`.
7. **The hooks.** One session-start, the guard over the new paths, `touched`,
   `stop`.
8. **The CLI.** The table above, and `upgrade` with its four cases.
9. **The view.** One list, five keys; delete the tabs.
10. **The words.** Skills, agents, `README.md`, `CLAUDE.md`, `docs/usage.md`.
    Plugin and binary to 4.0.0.
11. `make test`, then this repository upgrades itself and the pilot begins.

## Decisions

| Question | Decision |
|---|---|
| The one entity's name | a project. "Knowledge base" now means the `wiki/` folder of a project |
| Where it lives | `atlas/<name>/` inside the work, always |
| Whether a project may have no wiki | no. Every project has one; a wiki of four template pages costs nothing |
| Where the wiki's history lives | the repository that holds the work, scoped to the engine's paths |
| What a branch does to the wiki | pages follow the branch, like every other file in the work |
| The thread folders | under `threads/`, with the cards and the board |
| `scope` | folded into `description`; `upgrade` joins them |
| `mode` | stays, in `project.json` |
| The inbox | one, with a hint per item and no folder rule |
| `link`, `unlink`, many knowledge bases per project | gone with the split |
| Sharing knowledge between projects | left for later, over projects that each hold a wiki |
| The `vault` and `mode` tools | folded into `project` |
| Migration | `upgrade`, four cases, `git mv` where one repository holds both, a refusal where one knowledge base serves several projects |
| A project the user has not upgraded | refused by name with the command to run; nothing moves on its own |

## Left for later

The mission: knowledge that crosses projects. Two sketches, neither built:

- **A reader.** The atlas config lists every project. A `search` tool, or
  `wiki-query` with a wider net, reads the wiki of another project named by
  the user. Read-only, no link to maintain, nothing shared by accident.
- **A hub.** One project's wiki is marked as shared, and other projects may
  cite it. The citation is a page in the citing wiki that names the hub's
  page, so nothing depends on the hub being on this machine.

Also left: a Threads tab; links between threads; an Obsidian Base over the
cards; a Stop hook line when the session changed the work and touched no
thread; many wikis in one project, if one is ever not enough.
