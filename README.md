# atlas-obsidian

A wiki and the state of the work, in every project.

[![License: MIT](https://img.shields.io/badge/license-MIT-2563eb.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8.svg?logo=go&logoColor=white)](go.mod)
[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-7c3aed.svg)](.claude-plugin/plugin.json)
[![Version](https://img.shields.io/badge/version-5.1.0-d97745.svg)](.claude-plugin/plugin.json)

## About

Claude Code starts every session knowing nothing. You explain the codebase
again, decide things you already decided, and the paper you meant to read
stays in your downloads folder.

atlas-obsidian keeps that knowledge in the project itself, as plain Markdown.
One folder, `atlas/<name>/`, sits beside your code and holds two things:

- **A wiki.** Drop a PDF, a page, or a note in `inbox/`. Claude reads it and
  writes pages that cite it. Ask those pages later and the answer names its
  evidence.
- **Threads.** One thread is one line of work: a bug, a feature, a chore. It
  moves stub → spec → plan → receipt, and every stage is a document you can
  open. Nothing tracks a thread's state, because the documents are the state.

Start Claude anywhere inside your repository and the session already knows
which threads are open and what the wiki says.

The folder is plain files, so your repository tracks it like any other folder
and Obsidian opens it like any other vault. Nothing is hidden in a database.

- **You review every write.** Claude shows a plan, you approve it, the core
  makes one git commit. `undo` takes it back.
- **Your edits come first.** What you type in Obsidian is committed before any
  operation touches the wiki, so undo can never lose it.
- **The code and the wiki stay separate.** An operation's git commands are
  scoped to the wiki's own paths and never see your source.
- **One screen for every project.** Open any of them in Obsidian, or start
  Claude Code in the work, without leaving it.

## Quickstart

### Prerequisites

- [Claude Code](https://claude.com/claude-code), with `claude` on your PATH
- git
- Go 1.24 or newer, to build the binary
- [Obsidian](https://obsidian.md), optional: the files are Markdown and read
  fine anywhere, but Obsidian is what the folder is laid out for

### Install

```bash
git clone https://github.com/nathanaday/atlas-obsidian.git
cd atlas-obsidian
go install ./cmd/atlas-obsidian
```

The binary lands in `$(go env GOPATH)/bin`, usually `~/go/bin`. Add that
directory to your PATH if it is not there yet.

### Setup

```bash
atlas-obsidian setup
```

This installs the Claude Code plugin. It shows what it will do and asks first.
Run it again whenever you like; finished steps are skipped.

## Usage

Make a repository a project, open a thread, and start work:

```bash
cd ~/code/webapp
atlas-obsidian init --description "The customer-facing web app."
atlas-obsidian thread webapp new "Filter vehicle false alarms" --priority high
claude
```

`init` writes `atlas/webapp/` beside your code and tells the atlas it exists.
`thread ... new` writes the thread's card and its stub, in your words. The
Claude session that follows opens with that thread in view.

From there the skills carry it:

| Ask Claude | What happens |
|---|---|
| `/atlas-obsidian:describe` | writes the wiki page that says what this codebase is |
| `/atlas-obsidian:thread-spec` | turns the stub into a spec: what will be true when it is done |
| `/atlas-obsidian:thread-plan` | reads the code and files the approach |
| `/atlas-obsidian:thread-run` | does the work and keeps the plan current as it goes |
| `/atlas-obsidian:thread-receipt` | closes the thread and offers the wiki what the work taught |

Teach the wiki something:

```bash
cp ~/Downloads/paper.pdf ~/code/webapp/atlas/webapp/inbox/
claude
```

`/atlas-obsidian:wiki-ingest` reads what is waiting and writes pages that cite
it. `/atlas-obsidian:wiki-query` answers from those pages. A short note in the
same inbox becomes a thread instead, through `/atlas-obsidian:thread-stub`.

See every project at once:

```bash
atlas-obsidian
```

Enter expands an entry, `o` opens it in Obsidian, `c` starts Claude Code in the
work, `n` opens a new thread, `R` refreshes.

Every command and option: [docs/usage.md](docs/usage.md).

## Inside a project

```
webapp/                          your repository
├── ...                          your work, untouched
└── atlas/
    └── webapp/                  named after the project; a rename moves it
        ├── project.json         name, description, filing mode
        ├── wiki/                index.md, log.md, hot.md, overview.md,
        │                        then sources/ entities/ concepts/ canvases/
        ├── threads/             threads.md is the board; one card per open thread
        │   ├── stubs/           where a thread begins, in your words
        │   ├── specs/           what will be true when it is done
        │   ├── plans/           how the work will go, then its progress
        │   ├── receipts/        how it ended: completed or killed
        │   ├── phases/          a goal and an order; threads name their phase
        │   └── archive/         the cards of closed threads
        ├── inbox/               what you drop in: sources, and notes
        ├── ideas/               your scratch notes, outside the wiki
        └── .raw/captured/       unmodified copies of everything ingested
```

Open `atlas/webapp/` in Obsidian and both halves are there, each folder in its
own color.

Two filing modes decide where a new wiki page lands. `generic` files by type
into the folders above. `lyt` keeps atomic notes in `wiki/notes/` and navigates
them through Maps of Content. Change it with
`atlas-obsidian edit webapp --mode lyt`.

## The atlas

A project holds its own identity and no path. The paths live in one place:

```
~/.atlas-obsidian/
├── config.json             every project's work folder, the plugin, settings
└── state/registry.json     derived; `atlas-obsidian refresh` rebuilds it
```

The atlas never searches your disk. It knows a project because `init` added its
folder to that list, so a project can live anywhere. Move the folder and the
next session inside it repairs the entry.

## Conventions

Full reasoning in [docs/core-design.md](docs/core-design.md) and
[docs/v4-design.md](docs/v4-design.md).

- **One operation, one commit, in the wiki.** Claude plans, you review, the
  core commits. Claude's own file tools are refused inside `wiki/`.
- **Your edits come first.** Hand edits are committed before any operation, so
  undo never reaches them.
- **The core owns what it can derive:** the log, the source ledger, the thread
  cards and board, the health check. Claude writes prose and nothing else.
- **Ids travel, paths stay.** A project carries its facts; the atlas carries
  the paths and derives the rest.

## Documentation

- Usage: [docs/usage.md](docs/usage.md)
- Design and decisions: [docs/core-design.md](docs/core-design.md), [docs/v4-design.md](docs/v4-design.md)
- Skills: [skills/](skills/), one `SKILL.md` each
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
atlas-obsidian is released under the [MIT License](LICENSE).
