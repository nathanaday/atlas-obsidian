# Usage

Every command, with examples. `claude-atlas help` prints the short form.

## Three layers

`claude-atlas` on its own opens the whole atlas as one screen (`claude-atlas
view` says the same explicitly). Everything it does is also one command, and
one tool from a Claude Code session, so scripts, muscle memory, and the
`atlas` skills all work:

| In `view` | Command | Tool, from a Claude Code session |
|---|---|---|
| Enter on a project | `show NAME` | `atlas`, `status` |
| `o` open the project's folder in Obsidian | `open-vault NAME` | — |
| `c` start Claude Code in the work | `open-claude NAME [--thread ID]` | — |
| `n` open a new thread | `thread PROJECT new TEXT` | `thread` |
| `R` refresh | `refresh` | `atlas` with `refresh` |
| `←` `→` switch tabs; `h` shows every key; `q` quits | — | — |
| — | `init [PATH]`, `edit NAME`, `forget NAME` | `project` |
| — | `describe PROJECT` | `stage` with `snapshot`, then the `describe` skill |
| — | `ingest PROJECT [PATH...]` | `stage`, then the `wiki-ingest` skill |
| — | `threads [PROJECT]`, `thread PROJECT new\|show\|file\|close\|set\|reopen …`, `phase PROJECT …` | `threads`, `thread`, `phase` |
| — | `config new-days N` | `settings` |
| — | `stub PROJECT [TITLE...]` | `stub` |
| — | `lint`, `history`, `undo`, `recover`, `upgrade`, `apply` | `lint`, `history`, `undo` |
| — | `setup`, `doctor`, `info`, `version` | — |

## One thing

A **project** is a folder `atlas/<name>/` inside your work, and it holds both
halves of what the project knows:

- the **wiki** under `wiki/`, with sources, entities, and concepts, an
  `inbox/` to drop things into, `ideas/` for your own scratch notes, and
  `.raw/captured/` for the immutable copy of every source it ingested. Every
  change to `wiki/` is one reviewed operation and one git commit.
- the **threads** under `threads/`, one line of work each, with a document per
  stage and the phases that order them.

The folder takes the project's name, so Obsidian shows each project by its own
name. It has no git of its own: the repository that holds your work tracks it
like any other folder, and the wiki's commits are scoped to the wiki's own
paths, so an operation never touches your code.

## Make a project

```bash
cd ~/code/webapp
claude-atlas init                                   # name: the folder's; asks for a description
claude-atlas init --name "Web App" --description "The customer-facing web application." --mode lyt
claude-atlas init ~/code/notes --no-git             # another folder, and no repository
```

`init` writes `atlas/<name>/` in the current folder: `project.json`, `wiki/`
with its four pages and an empty source ledger, `threads/` with its stage
folders, `inbox/`, `ideas/`, and `.obsidian/` with the CSS snippet that gives
the wiki's folders their colors and each thread stage its callout color and
icon. It commits all of it as one `setup` operation and lists the work folder
in the atlas config.

The folder's name is the project's name, less the characters Obsidian cannot
use; `edit --name` moves the folder to match. When the work is in no git
repository, `init` runs `git init` there on `main`; `--no-git` skips that, and
then the wiki has no history and no operation can run until the work is a
repository. It leaves a repository as it is, and it makes no repository in a
folder inside another one. In a terminal with no flags it asks for the name and
the description. It refuses an `atlas/<name>/` that already holds something, a
folder that is a project already, a folder inside another project, and a folder
the holding repository ignores. A work folder holds one project.

The description is one to three sentences: what the work is, and what its wiki
should remember. `wiki-ingest` and `wiki-query` read it to judge what belongs.

Then:

```bash
claude-atlas edit webapp --name "Web App" --description "…" --mode lyt
claude-atlas show webapp
claude-atlas forget webapp                          # drop it from the config; atlas/webapp/ stays
claude-atlas describe webapp                        # stage a snapshot for the describe skill
```

Each edit writes the identity file and commits it as a `setup` operation. **A
new name moves `atlas/<name>/`.** The name keeps every character; the folder
takes it cleaned for a path, so `Web App: v2` files as `Web App- v2`. A taken
folder refuses the whole edit. The work folder never moves; it is yours.

`forget` drops the project from the config; the folder stays, and a session
started in it lists the project again. Deleting `atlas/<name>/` is how a
project ends.

**A moved project heals itself.** The config holds the work folder's path.
Move the folder, then start a session in it: the hook finds the project's id
listed at another path and rewrites the entry. A clone on another machine is
added the same way. Until then the view shows the project as missing.

**Describe the work.** The wiki knows nothing about the code until a page says
what it is. `describe` writes a snapshot of the work into `inbox/`, then offers
to start Claude Code with `/claude-atlas:describe`, which reads the snapshot
and the work and writes `wiki/entities/<name>.md`: what the project is, how it
is built and laid out, what it has delivered, the concepts it introduces. The
snapshot holds the work's CLAUDE.md and README, its files (folders only past
2000), the first heading of every markdown file under `docs/`, and, once a page
exists, the git log since that page's commit. The session hook says when the
page is missing or has fallen behind, and `thread-receipt` offers the update
when a thread completes.

## Threads and phases

A thread is one line of work: a bug, a feature, a chore. It moves through four
stages, and each stage is a document in its own folder under
`atlas/<name>/threads/`:

| Stage | Folder | The document holds |
|---|---|---|
| stub | `threads/stubs/` | where the thread begins, in your words |
| spec | `threads/specs/` | what will be true when the thread is done, and why |
| plan | `threads/plans/` | how the work will go, then its progress |
| receipt | `threads/receipts/` | how the thread ended: completed or killed |

The stage is never set. It is the furthest document that exists, so a thread
moves only when its document is filed, and every change of stage leaves a page
you can open. A stage may be skipped. A receipt closes the thread.

The thread's card, `threads/<Title>.md`, holds its id (like
`thr-20260917-3f2a`), priority, phase, and what it waits on. It links and
embeds every document, so one page shows the thread end to end. The card of a
closed thread moves to `threads/archive/`. The core renders the board,
`threads/threads.md`, from the cards and the documents: the open threads by
stage, the phases, the closed threads. In Obsidian each document opens with a
callout card in its stage's color that links the thread's other documents. A
session started in the work sees the open threads at its start, so a thread
lives across sessions.

PROJECT is a name or a path, and `.` is the project you are in. ID is a
thread's id, its title, or the start of its title.

Open a thread without ceremony, from anywhere:

```bash
claude-atlas thread webapp new "Filter vehicle false alarms"
claude-atlas thread webapp new "Write the fault taxonomy" --priority high --phase "Alarm quality"
claude-atlas thread . new "Cars trip the alarm at dusk." --title "Filter vehicle false alarms"
```

Or drop a note into the project's `inbox/`; the next session's
`/claude-atlas:thread-stub` opens a thread from each note and removes the note.

File a document to move the thread. The text comes from `--text`, from
`--file PATH`, or from stdin:

```bash
claude-atlas thread webapp file "Filter vehicle" spec --file spec.md
claude-atlas thread webapp file "Filter vehicle" plan --text "1. Mask the road. 2. Replay the July set."
cat receipt.md | claude-atlas thread webapp file "Filter vehicle" receipt --outcome completed
claude-atlas thread webapp close "Filter vehicle" "Shipped in 3.2; the July set replays clean."
claude-atlas thread webapp close "Drop jQuery" "Already gone since 3.1." --killed
claude-atlas thread webapp reopen "Drop jQuery"          # deletes the receipt
```

`close` files the receipt: completed, or killed with `--killed`. A document
that exists is never filed again; edit the page.

See what is open, read one thread, and change a card:

```bash
claude-atlas threads webapp
claude-atlas threads                                # every project
claude-atlas threads webapp --all                   # the closed ones too
claude-atlas threads webapp --stage plan
claude-atlas threads webapp --json
claude-atlas thread webapp show thr-20260917-3f2a   # its state and the path of each document
claude-atlas thread webapp show "Filter vehicle" --json
claude-atlas thread webapp set "Filter vehicle" --phase "Alarm quality" --priority high
claude-atlas thread webapp set "Filter vehicle" --blocked "the July field data"
claude-atlas thread webapp set "Filter vehicle" --blocked ""     # unblock
claude-atlas thread webapp set "Filter vehicle" --title "Filter vehicles at dusk"
```

A new title renames the card and every document.

A phase is a named slice of the timeline: a page under `threads/phases/` with a
goal and an order. Threads name their phase; a phase never lists its threads. A
phase is finished when every thread in it is closed.

```bash
claude-atlas phase webapp create "Alarm quality" --goal "False alarms under 1 per camera-day." --order 1
claude-atlas phase webapp rename "Alarm quality" --to "Alarm precision"
claude-atlas phase webapp reorder "Alarm precision" --order 2
claude-atlas phase webapp remove "Alarm precision"      # refused while a thread names it
```

The tools write the cards, the board, and each document's frontmatter and first
callout. The rest of a document is prose Claude writes with its ordinary
editing tools, and you may edit any page by hand in any editor. Delete a
document by hand and the thread moves back to the stage before it. Off limits
to the model: the cards and the board (the pages directly under `threads/`),
`project.json`, and a new file written straight into a stage folder, because a
new document comes from the `thread` tool, which gives it the thread's id. The
plugin's `touched` hook runs after every Write and Edit: an edit to a stage
document marks its thread as updated today.

Work a thread in a session. The skills carry the ceremony: `thread` shows the
board, routes, and reviews; `thread-stub` opens a thread; `thread-spec`
researches, asks only what it cannot find, and files the spec; `thread-plan`
files the approach; `thread-run` does the work, commits and tests along the
way, and writes progress in the plan; `thread-receipt` closes the thread and
offers the wiki what the work taught. `open-claude --thread` starts the session
in the work with the skill for the thread's next stage as the first message:

```bash
claude-atlas open-claude webapp --thread thr-20260917-3f2a
claude-atlas open-claude webapp --thread "Filter vehicle"    # a title prefix works
```

`show` counts a project's threads by stage and signals the blocked ones and the
stale ones, which have a plan and no update for 14 days.

## Work in a session

A session started anywhere inside a project's work is that project's session.
Its start says which project this is, what its wiki holds, whether a page
describes the work, what threads are open, and what waits in the inbox:

```text
claude-atlas: project webapp at ~/code/webapp (git, main)
Description: The customer-facing web application for the fire-detection product.
Wiki: atlas/webapp/wiki · 140 pages · generic mode
The work is described in wiki/entities/webapp.md at fc70d93, 12 commits behind. The describe skill brings the page up to date.
Search the wiki (the wiki-query skill) before answering from the code alone. Change wiki pages only through the atlas MCP tools (plan, then apply).
Open threads: 4 (plan 1, spec 1, stub 2) in 2 phases. A thread moves stub, spec, plan, receipt, and each stage is a document the thread tool files; the thread skills say how. Work that belongs to a thread goes on its documents.
- [plan] Filter vehicle false alarms (thr-20260917-3f2a) · Alarm quality · high · updated 2026-09-17
Inbox: 2 sources for the wiki-ingest skill, 1 note for the thread-stub skill.
```

In the session, the skills are on the slash menu:

| Skill | What it does |
|---|---|
| `/claude-atlas:wiki` | orient and route to the right skill |
| `/claude-atlas:work` | take a change from a sentence to a thread with a plan, then work it |
| `/claude-atlas:thread` | show the board by stage, change a card, create or change a phase, review the board |
| `/claude-atlas:thread-stub` | open a thread from a sentence, from a note in `inbox/`, or in another project |
| `/claude-atlas:thread-spec` | research, ask only what reading cannot answer, file the spec |
| `/claude-atlas:thread-plan` | explore the code with the spec in hand, file the plan |
| `/claude-atlas:thread-run` | do the work, commit and test along the way, write progress in the plan |
| `/claude-atlas:thread-receipt` | close as completed or killed, file the receipt, offer the wiki what was learned |
| `/claude-atlas:describe` | describe the work in the wiki from a snapshot, or bring its page up to date |
| `/claude-atlas:wiki-ingest` | read the sources in the inbox and write cited pages |
| `/claude-atlas:wiki-query` | answer from the wiki, with citations |
| `/claude-atlas:save` | keep an answer or decision as a page |
| `/claude-atlas:wiki-lint` | check the wiki's health |
| `/claude-atlas:wiki-mode` | read or change the filing mode |
| `/claude-atlas:wiki-fold` | roll up log entries |
| `/claude-atlas:canvas` | create and update Obsidian Canvas boards |
| `/claude-atlas:obsidian-bases` | draft Bases `.base` views |
| `/claude-atlas:obsidian-markdown` | Obsidian syntax help |
| `/claude-atlas:think` | a structured review before a consequential change |
| `/claude-atlas:atlas` | every project, refresh, settings |
| `/claude-atlas:atlas-project` | make this folder a project; rename it, change its description or mode, forget it |

Claude shows a preview of every change to the wiki before it applies it. Each
applied change is one git commit. Every tool acts on this session's project;
`threads`, `thread`, `phase`, and `stage` take `project` to reach another
project the atlas lists.

## Ingest sources

Drop a source in the project's inbox and start Claude Code in the work:

```bash
cp ~/Downloads/paper.pdf ~/code/webapp/atlas/webapp/inbox/
claude-atlas open-claude webapp
```

The inbox takes both kinds of thing you drop in. A document is a source for
`wiki-ingest`; a short note is a thread for `thread-stub`. The `inbox` tool
hints at which each file looks like, from its kind and its size, and the skills
say what they will do before they do it.

Or point the atlas at a file or folder outside the project. What is new is
copied into `inbox/`; the originals stay where they are. A file whose bytes the
project already holds is skipped, so a folder that grows over time can be
ingested again and only its new files cost anything. The project remembers the
folders it staged from, and an `ingest` with no path stages what is new in every
one of them.

```bash
claude-atlas ingest webapp ~/Papers
claude-atlas ingest webapp ~/Papers/paper.pdf
claude-atlas ingest webapp                   # every folder ingested before
claude-atlas ingest webapp ~/Papers --dry-run
claude-atlas ingest webapp ~/Papers --no-claude
```

After staging, the command offers to start Claude Code with
`/claude-atlas:wiki-ingest` as its first message, so the review and the apply
happen in the session. `--no-claude` stages and stops. Nothing is ingested
until that session runs the skill.

The first time Claude Code opens a folder it asks whether you trust it, with
`No, exit` selected. Choose `Yes`; pressing Enter on the default quits.

## History and undo

```bash
claude-atlas history webapp
claude-atlas undo webapp ingest-20260912-150405-ab12
```

Inside the work, the name can be omitted:

```bash
cd ~/code/webapp
claude-atlas history
claude-atlas lint
```

`undo` puts back every page the operation wrote, as a new commit, and touches
nothing else; your code and the thread documents are never in its way. It
refuses when one of those pages changed after the operation, because putting the
page back would throw that change away.

## Health check

```bash
claude-atlas lint webapp
claude-atlas lint webapp --json
claude-atlas lint webapp --strict     # exit 1 when there are findings
```

Lint reads `wiki/` only. The thread documents are pages of another kind, and
the `threads` tool reports what it cannot read among them.

## Stubs and wanted pages

A **wanted page** is a title one or more pages link to that nobody has written
yet. Lint reports it, and a session's start line names a few and counts the
rest:

```text
Stubs: 2 pages to fill (Backpropagation, Loss Landscape). Wanted: 1 linked page does not exist yet (Contrastive Learning). Fill or stub them with the wiki-lint skill.
```

`stub` creates a seed page for each: frontmatter and the section headings its
type usually carries, left for a session or a person to fill in.

```bash
claude-atlas stub webapp
claude-atlas stub webapp "Backpropagation" "Loss Landscape" --type concept
```

With no title, `stub` seeds every wanted page and every empty page a link
already points to. `--type` sets the type for a title that names none: concept
or entity; in `lyt` mode note or moc as well. The default is concept, or note
in `lyt` mode.

## Filing mode

`generic` files pages by type into `wiki/sources/`, `entities/`, and
`concepts/`. `lyt` keeps atomic notes in `wiki/notes/` and navigates them
through Maps of Content in `wiki/mocs/`.

```bash
claude-atlas edit webapp --mode lyt
```

Changing the mode affects future pages only.

## Recover and upgrade

If an operation was interrupted, the project says so at the next session start.
Restore it:

```bash
claude-atlas recover webapp
```

`upgrade` brings a project or a knowledge base made by an older version to this
one. It moves your files, so it always shows what it will do and asks first,
and it uses `git mv` where one repository holds both sides, so the history
follows.

```bash
claude-atlas upgrade                                 # the project or knowledge base you are in
claude-atlas upgrade webapp
claude-atlas upgrade ~/Vaults/papers                 # a knowledge base, by path
claude-atlas upgrade webapp --absorb ~/Vaults/kb     # name the knowledge base to take as the wiki
claude-atlas upgrade --all
```

Four cases:

1. **A 3.x project whose knowledge base is inside the work.** Its `wiki/`,
   `.raw/`, `inbox/`, and `ideas/` move into `atlas/<name>/`, its scope joins
   the project's description, its mode becomes the project's, and its entry
   leaves the config.
2. **A 3.x project whose knowledge base is elsewhere.** The same, by copy. The
   old folder stays on disk for you to delete.
3. **One knowledge base that several projects used.** Refused, with the
   projects named. Upgrade one of them, which absorbs it; the others take
   `--absorb PATH` to copy it, or start with an empty wiki.
4. **A knowledge base no project used.** Its folder becomes a project, with
   that knowledge base as the wiki.

In every case the stage folders move under `threads/`, the task pages of 2.x
become threads (Idea becomes the stub, Plan and Progress the plan, Outcome the
receipt; `done` becomes `completed` and `cancelled` becomes `killed`), a project
that 2.2.0 made directly in `atlas/` moves into `atlas/<name>/`, and the CSS
snippet is rewritten. A page it cannot read stays where it is and is named.

Until a folder is upgraded, every command and every tool refuses it with the
command to run.

If you opened an old knowledge base or the old `atlas/` folder in Obsidian,
remove it from Obsidian's vault list and open `atlas/<name>/` instead.

## Open in Obsidian

Opens the project's folder by name or path, or the one you are in with no
argument. If Obsidian does not know the folder yet, the command offers to
register it; Obsidian quits and relaunches so it sees the new entry.

```bash
claude-atlas open-vault webapp
claude-atlas open-vault
```

## The atlas

`claude-atlas` with no command, or `claude-atlas view`, is one screen: every
project, warmest first, with its path, its wiki's page count, its open thread
count and current phase, what waits in its inbox, and a mark when the path is
missing. Enter expands an entry in place with what `show` prints. A Problems
tab appears only while the atlas holds a folder it could not read. The view is
for seeing what exists and getting there; creating and changing things is the
CLI's and the session's job.

| Key | What it does |
|---|---|
| `←` `→` | previous tab, next tab |
| `↑` `↓` | move |
| Enter | expand the entry under the cursor, or collapse it |
| `o` | open the project's folder in Obsidian |
| `c` | start Claude Code in the work |
| `n` | open a new thread |
| `R` | refresh in the background |
| `h` | show every key; again to hide them |
| `q` | quit |

```bash
claude-atlas
claude-atlas view
```

### How the atlas finds things

The atlas never searches your disk. The config lists every project's work
folder under `projects`, and the scan reads
`atlas/<name>/project.json` in each listed folder. `init` writes an entry and
`forget` drops it. A session started in a project the config does not list adds
it, and one started in a folder that moved heals the entry by id. Every command
scans afresh before it acts.

`refresh` runs the scan, reads each entry, and rewrites
`~/.claude-atlas/state/registry.json`: every project with its page count, heat,
inbox counts, thread counts, phases, the page that describes the work, and what
git says about it. Everything in that file is derived, so deleting it costs one
refresh.

```bash
claude-atlas refresh
```

`list` prints every project: heat, name, and path, then the folders the atlas
could not read. `show` prints everything the atlas holds about one, with the
signals that need attention.

```bash
claude-atlas list
claude-atlas show webapp
```

A command names a project by its name, by its id or an id prefix of eight
characters or more, or by its path. Two projects with one name make the command
ask for the path or the id instead.

## Adopt an existing vault

An Obsidian vault, a vault made with claude-obsidian, or a knowledge base from
an earlier version becomes a project with `upgrade`: its folder becomes the
work, `atlas/<name>/` is created inside it, and its `wiki/`, `inbox/`, `ideas/`,
and `.raw/` move in. Nothing in it is replaced.

```bash
claude-atlas upgrade ~/Documents/MyKnowledgeVault
```

A vault with no `wiki/` folder is a folder of notes, not a wiki: make it a
project with `init`, and move the notes under `wiki/` yourself, in the shape the
filing mode expects.

## Setup and health

```bash
claude-atlas setup
claude-atlas setup --plugin-source ~/projects/software/claude-atlas   # install the plugin from a checkout
claude-atlas setup --no-plugin
claude-atlas doctor
claude-atlas info
claude-atlas version
```

Setup shows its plan and asks before it acts: the atlas home and the plugin in
Claude Code. It creates no project; `claude-atlas init` in your work does that.
`doctor` checks git, Claude Code, the plugin's version against the binary's, and
every project the config lists, naming the ones whose folder is gone, whose
wiki needs recovery, or which wait for `upgrade`.

## Scripts

Apply a plan file without Claude. The file holds the `plan` tool's arguments;
`content_file` may replace `content`.

```bash
claude-atlas apply webapp plan.json
```

## Configuration

`~/.claude-atlas/config.json`:

```json
{
  "schema": "claude-atlas.config.v4",
  "projects": ["/Users/you/code/webapp", "/Users/you/code/fw"],
  "plugin": {
    "id": "claude-atlas@nathanaday-claude-atlas",
    "source": "nathanaday/claude-atlas"
  },
  "claude_code": {
    "command": "claude",
    "session_context": true
  },
  "heat": {
    "new_days": 7
  }
}
```

`claude-atlas config` prints the settings; `claude-atlas config new-days 14`
sets one and refreshes.

| Setting | Effect |
|---|---|
| `projects` | every project's work folder. `init`, `forget`, `upgrade`, and the session hook keep this list |
| `knowledge` | the knowledge bases of 3.x that `upgrade` has not absorbed yet. Nothing adds to it |
| `claude_code.prompt` | a first message sent on every `open-claude`, for example `/claude-atlas:wiki` |
| `claude_code.args` | flags for `claude`, such as `--model` |
| `claude_code.session_context` | whether the session-start hook hands Claude the wiki's `hot.md` |
| `plugin.source` | where `claude plugin marketplace add` gets the plugin: a GitHub slug or a local path |
| `heat.new_days` | how many days after its creation a project shows as new whatever its activity; 7 by default, 0 turns it off |
| `--home DIR`, `CLAUDE_ATLAS_HOME` | use a different home instead of `~/.claude-atlas` |
| `-y`, `--yes` | answer yes to every prompt |

These are the only paths the atlas stores. Everything else it shows comes from
the scan or from `state/registry.json`, which `refresh` rewrites in full.
