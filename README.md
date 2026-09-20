# claude-atlas

A wiki and the state of the work, in every project.

[![License: MIT](https://img.shields.io/badge/license-MIT-2563eb.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8.svg?logo=go&logoColor=white)](go.mod)
[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-7c3aed.svg)](.claude-plugin/plugin.json)
[![Version](https://img.shields.io/badge/version-4.0.0-d97745.svg)](.claude-plugin/plugin.json)

## About

What Claude learns in one session is gone by the next, and reading piles up
faster than notes get written. claude-atlas gives every project one folder
that holds both halves of what it knows, as plain Markdown you can open in
Obsidian.

One thing, and only one. A **project** is a folder `atlas/<name>/` inside your
work, a repository or a folder of documents:

- Its **wiki**, under `wiki/`. Drop a source in `inbox/` and get linked pages
  that cite it. Ask it later. Every change Claude makes is a plan you see
  first and one git commit you can undo.
- Its **threads**, under `threads/`. A thread is one line of work: a bug, a
  feature, a chore. It moves stub → spec → plan → receipt, and each stage is a
  document in its own folder, so the state of every thread is a page you can
  open.

The folder takes the project's name, so each project you open in Obsidian
shows its own name. It has no git of its own: your repository tracks it like
any other folder, and the wiki's commits are scoped to the wiki's own paths,
so an operation never touches your code.

A session started anywhere inside the work is that project's session: it sees
the open threads, searches the wiki before it answers from the code, and writes
progress in the thread's plan as it goes.

- **Ingest from an inbox.** Files in, cited pages out; sources kept immutable.
- **Ask the wiki.** Answers name their evidence, or say what is missing.
- **Review, then commit.** Preview every change; undo takes it back. Your own
  Obsidian edits are committed first and never touched.
- **Threads beside the code.** Every session starts knowing what is open. The
  pages are plain markdown in your repository, with a colored card per stage in
  Obsidian.
- **The wiki knows the code.** One page says what the work is, how it is built,
  and what it has delivered. Completing a thread offers to update it.
- **One view.** Every project on one screen, with a key to open it in Obsidian
  or start Claude Code in the work.
- **Native Obsidian.** Plain Markdown, wikilinks, Canvas boards, Bases views.

## Quickstart

### Prerequisites

- [Claude Code](https://claude.com/claude-code), with `claude` on your PATH
- [Obsidian](https://obsidian.md)
- git
- Go 1.24 or newer, to build the binary

### Install

```bash
git clone https://github.com/nathanaday/claude-atlas.git
cd claude-atlas
go install ./cmd/claude-atlas
```

The binary lands in `$(go env GOPATH)/bin`, usually `~/go/bin`. Add that
directory to your PATH if it is not there.

### Setup

```bash
claude-atlas setup
```

Setup shows its plan and asks before it does anything. It installs the
claude-atlas plugin into Claude Code. Run it again at any time; finished steps
are skipped.

## Usage

Make each repository or folder you work in a project:

```bash
cd ~/code/webapp
claude-atlas init --description "The customer-facing web app for the fire-detection product."
claude-atlas thread webapp new "Filter vehicle false alarms" --priority high
claude
```

`init` writes `atlas/webapp/` beside your code, with its wiki and its threads,
and tells the atlas the project exists. `thread ... new` writes the thread's
card and its stub. The session in `~/code/webapp` starts with the open thread
in view. `/claude-atlas:describe` writes the page that says what the work is;
`/claude-atlas:thread-spec`, `/claude-atlas:thread-plan`, and
`/claude-atlas:thread-run` carry the thread through, and each files its
document; `/claude-atlas:thread-receipt` closes it and offers the wiki what the
work taught. `claude-atlas threads` lists every open thread by stage, and
`claude-atlas open-claude webapp --thread ID` continues one.

Drop a source in the project's inbox:

```bash
cp ~/Downloads/paper.pdf ~/code/webapp/atlas/webapp/inbox/
claude
```

`/claude-atlas:wiki-ingest` reads the sources waiting there and writes cited
pages; `/claude-atlas:wiki-query` answers from them. Claude shows a preview
before every change, and `claude-atlas undo` takes one back. A short note in
the same inbox becomes a thread instead: `/claude-atlas:thread-stub` opens one
from each.

See everything at once:

```bash
claude-atlas
```

One list of projects. Enter expands an entry, `o` opens it in Obsidian, `c`
starts Claude Code in the work, `n` opens a new thread, `R` refreshes.

Every command, the slash menu, and configuration:
[docs/usage.md](docs/usage.md).

## Inside a project

```
webapp/                            your repository, or a folder of documents
├── ...                            your work, untouched
└── atlas/
    └── webapp/                    the project's name; a rename moves the folder
        ├── project.json           its identity: name, description, filing mode
        ├── wiki/                  the knowledge half
        │   ├── index.md           the catalog
        │   ├── log.md             what happened, newest first
        │   ├── hot.md             recent context, handed to Claude at session start
        │   ├── overview.md        the stable big picture
        │   ├── sources/ entities/ concepts/ canvases/
        │   └── meta/ledgers/source-ledger.json
        ├── threads/               the work half
        │   ├── threads.md         the board, generated: open threads by stage
        │   ├── <Title>.md         the card of an open thread, generated
        │   ├── archive/           the cards of closed threads
        │   ├── stubs/<Title>.md      where a thread begins, in your words
        │   ├── specs/<Title>.md      what will be true when it is done, and why
        │   ├── plans/<Title>.md      how the work will go, then its progress
        │   ├── receipts/<Title>.md   how it ended: completed or killed
        │   └── phases/<Title>.md     a goal and an order; threads name their phase
        ├── inbox/                 what you drop in: sources, and notes
        ├── ideas/                 your scratch notes, outside the wiki
        └── .raw/captured/         immutable copies of ingested sources
```

Open `atlas/webapp/` in Obsidian and both halves are there, each folder in its
own color.

A thread's stage is never set: it is the furthest document that exists. The
`thread` tool files a document, and that moves the thread. A stage may be
skipped, and a receipt closes the thread. The card holds the thread's id,
priority, phase, and what it waits on, and it embeds every document, so one
page shows the thread end to end. Each document opens with a callout card in
its stage's color that links the thread's other documents. The tools write
the cards, the board, and those callouts; Claude writes the prose. You edit any
page by hand.

Only an operation writes `wiki/`: Claude builds a plan, you see the preview,
the core makes one commit. Two filing modes decide where a new page lands:
`generic` files by type into the folders above; `lyt` keeps atomic notes in
`wiki/notes/` and navigates them through Maps of Content. Change it with
`claude-atlas edit webapp --mode lyt`.

## The atlas

A project carries its own identity file, `atlas/<name>/project.json`, and it
holds no path. The atlas keeps the paths in one place:

```
~/.claude-atlas/
├── config.json               every project's work folder, the plugin, heat
└── state/registry.json       derived: every project with its pages, threads,
                              inbox, and the page that describes its work
```

The atlas never searches your disk. It knows a project because the config
lists its work folder, and `init` adds that entry, so a project can live
anywhere. Move a folder and the next session inside it heals its entry by id.
`claude-atlas refresh` rewrites the registry in full, so nothing in it goes
stale.

## Conventions

Full reasoning in [docs/core-design.md](docs/core-design.md) and
[docs/v4-design.md](docs/v4-design.md).

- One operation, one commit, in the wiki. Claude plans, you review, the core
  commits. Claude's own file tools are refused inside `wiki/`, and every git
  command an operation runs is scoped to the wiki's paths.
- Your edits come first. Hand edits in the wiki are committed before any
  operation, so undo never touches them.
- The core owns what it can derive: the log, the source ledger, the thread
  cards and board, the health check. Claude writes pages.
- Ids travel, paths stay. A project holds its own facts and no path; the atlas
  holds the paths, and derives everything else.
- The two halves, one folder, different rules. Only an operation writes
  `wiki/`; a thread's prose is Claude's to write and yours to edit.

## Documentation

- Usage: [docs/usage.md](docs/usage.md)
- Design and decisions: [docs/core-design.md](docs/core-design.md), [docs/v4-design.md](docs/v4-design.md)
- Skills: [skills/](skills/), one `SKILL.md` per skill
- Plugin manifest: [.claude-plugin/plugin.json](.claude-plugin/plugin.json)

External:

- Obsidian: https://obsidian.md
- Claude Code plugins: https://code.claude.com/docs/en/plugins
- MCP Go SDK: https://github.com/modelcontextprotocol/go-sdk

## Credits

The vault layout, the inbox workflow, and the skills derive from
[claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian) by
AgriciDaniel (MIT), which follows
[Andrej Karpathy's LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f).
Obsidian syntax references draw on
[kepano/obsidian-skills](https://github.com/kepano/obsidian-skills).
claude-atlas is released under the [MIT License](LICENSE).
