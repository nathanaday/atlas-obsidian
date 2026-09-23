# atlas-obsidian

A wiki and the state of the work, in every project.

[![License: MIT](https://img.shields.io/badge/license-MIT-2563eb.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8.svg?logo=go&logoColor=white)](go.mod)
[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-7c3aed.svg)](.claude-plugin/plugin.json)
[![Codex plugin](https://img.shields.io/badge/Codex-plugin-10a37f.svg)](.codex-plugin/plugin.json)
[![Version](https://img.shields.io/badge/version-5.6.0-d97745.svg)](.claude-plugin/plugin.json)

## About

An agent session starts with no memory of the last one. You explain the project
again, and the agent reads the same code to reach the conclusions it reached last
week.

atlas-obsidian keeps what a project knows inside the project: Markdown pages that
git tracks and Obsidian opens, in a folder beside your code. A hook runs when you
start Claude Code or Codex anywhere in the repository. It hands the session the
project's description, the wiki's recent context, the open threads, and the files
waiting in the inbox.

![Every project on one screen](docs/examples/TUI-project-nav.png)

*`atlas-obsidian` with no arguments: every project the atlas lists, the member
links between them, and a disk sized by each wiki's page count.*

Two pieces install separately: a Go binary that makes a folder of work into a
project and serves its MCP tools, and a plugin for Claude Code and Codex that
carries the skills and hooks. Both agents read the same files.

## What a project holds

One folder, `atlas/<name>/`, sitting beside your code.

```
webapp/                      your repository
├── ...                      your work, untouched
└── atlas/
    └── webapp/
        ├── project.json     name, description, filing mode, threads on or off
        ├── wiki/            the pages, and the log of every change to them
        ├── threads/         stubs, specs, plans, receipts; optional
        ├── inbox/           what you drop in: sources, and notes
        └── ideas/           your scratch notes, which nothing reads
```

**A wiki that cites its sources.** Put a PDF, an article, or a note in `inbox/`.
The agent reads it, keeps an unmodified copy under `.raw/captured/`, and writes
pages that cite that copy. Ask a question later and the answer names the page it
used and the captured file that page cites.

**Threads that track the work.** A thread is one bug, feature, or chore. It moves
through four documents: a stub in your own words, then a spec, a plan, and a
receipt. No field records the stage. The furthest document that exists is the
stage, so every change of state is a page you can open and read.

### Sources too large for one context window

![Ingesting a 351-page slide deck](docs/examples/ingesting-huge-slide-deck.png)

Past about 40 PDF pages or 2500 lines, `wiki-ingest` splits a source at its
content breaks and sends each range to a read-only worker, eight at a time. A
worker reads only its range and returns the claims it found, each with the page
or line it came from. It writes no pages. The session groups those claims by
subject, drops the repetitions, keeps every locator, and files one plan for your
approval. Above, a 351-page slide deck is split into eight ranges.

### The pages link to each other

Ingestion writes ordinary wikilinks, so Obsidian's graph view and its backlinks
work on the result. A source page links to the concepts it covers, and each
concept page lists the sources that cite it.

| Hovering a source page | Hovering a concept page |
|---|---|
| ![The graph around a paper](docs/examples/huge-kb-focus-1.png) | ![The graph around a concept](docs/examples/huge-kb-focus-2.png) |

### One wiki over several projects

A project may list other projects as members. Its wiki then mirrors each member's
wiki under `wiki/projects/<name>/`, and its own pages link into those mirrors
with ordinary wikilinks. Obsidian draws one graph over every project in the list,
and a session in the hub reads one wiki.

![A hub wiki over four member projects](docs/examples/huge-kb-high-level.png)

```bash
atlas-obsidian edit platform --add-member svc-a --add-member svc-b
atlas-obsidian sync platform
```

The mirrors are derived: sync rewrites them as one operation, the guard refuses
edits there, and nothing is ever written back into a member. The members' threads
are mirrored too, so the hub's board lists every open thread across all of them.
When two members wrote about one thing, `overlap` finds the pair and
`atlas-merge` proposes the page that replaces both. Design and reasons:
[docs/members-design.md](docs/members-design.md).

## Quickstart

### Prerequisites

- [Claude Code](https://claude.com/claude-code) or
  [Codex CLI](https://learn.chatgpt.com/docs/cli), with `claude` or `codex` on
  your PATH. Codex needs a release with `codex plugin` and `/hooks` support.
- git, and Go 1.24 or newer to build the binary.
- macOS or Linux.
- [Obsidian](https://obsidian.md) is optional. The files are plain Markdown and
  read fine anywhere, but the folder is laid out for it and ships a CSS snippet
  that colors each kind of folder.

### Install

```bash
git clone https://github.com/nathanaday/atlas-obsidian.git
cd atlas-obsidian
make install

atlas-obsidian setup --agent claude --plugin-source "$PWD"   # or --agent codex
```

`make install` puts `atlas-obsidian` in `$(go env GOPATH)/bin`, usually
`~/go/bin`; add that to your PATH if it is not there. Use `make install` rather
than `go install` so the binary is stamped with its version, which
`atlas-obsidian doctor` compares against the plugin's.

Setup prints the changes it will make and asks before making them. Run it once
per agent if you use both. Codex users then open `/hooks` in a new session, trust
the Atlas hooks, and restart; Codex ignores hooks until they are trusted. Check
the result with `atlas-obsidian doctor --agent claude` or `--agent codex`.

### Make a project

```bash
cd ~/code/webapp
atlas-obsidian init --description "The customer-facing web app."
```

This writes `atlas/webapp/` beside your code, commits it, and records the folder
in the atlas config. A folder that is not in a git repository becomes one, since
the wiki needs a history to commit into; pass `--no-git` to decline.

Two filing modes decide where a new page lands. `generic`, the default, files by
type into `wiki/sources/`, `wiki/entities/`, and `wiki/concepts/`. `lyt` keeps
atomic notes in `wiki/notes/` and navigates them through Maps of Content in
`wiki/mocs/`. Switch with `atlas-obsidian edit webapp --mode lyt`; pages already
written stay where they are.

For a repository with history, run `/atlas-obsidian:atlas-onboard` in a session
instead. It makes the project, describes the work in the wiki, proposes a
structure, and turns the TODO and FIXME lines, roadmap notes, and open issues it
finds into threads you pick from.

## Usage

Open a thread and start a session on it:

```bash
atlas-obsidian thread webapp new "Filter vehicle false alarms" --priority high
claude   # or: codex
```

Add a source to the wiki:

```bash
cp ~/Downloads/paper.pdf ~/code/webapp/atlas/webapp/inbox/
claude   # or: codex
```

Then choose a skill. Every skill is named `<noun>-<verb>` on one of three nouns:
`atlas-` acts on projects, `wiki-` on what a project knows, `thread-` on what it
does. In Codex, type `$` and pick from the list.

| Claude Code | Codex | What it does |
|---|---|---|
| `/atlas-obsidian:atlas` | `$atlas` | every project, and the map of every skill |
| `/atlas-obsidian:thread-work` | `$thread-work` | writes a thread's next document: spec, plan, then receipt |
| `/atlas-obsidian:wiki-ingest` | `$wiki-ingest` | turns the sources in the inbox into cited pages |
| `/atlas-obsidian:wiki-query` | `$wiki-query` | answers from the wiki, changing nothing |
| `/atlas-obsidian:wiki-review` | `$wiki-review` | checks the wiki, quick or deep |

`/atlas-obsidian:thread-work Filter vehicle false alarms` writes that thread's
spec and stops for your approval, then writes the plan, then does the work. All
21 skills and how they fit: [docs/skills.md](docs/skills.md).

Run `atlas-obsidian` with no arguments for the view above: each project is a
node, each member link an edge, positioned by a force simulation. `l` switches
the lens between wiki, threads, and version control. From the selected node, `o`
opens Obsidian, `c` starts your agent in the work folder, `i` opens your IDE, and
`t` opens a terminal there.

Every command and its options: [docs/usage.md](docs/usage.md).

## Patterns and conventions

Full reasoning in [docs/core-design.md](docs/core-design.md).

- **One operation, one commit.** A hook refuses the agent's file-edit tools
  anywhere under `wiki/`. The agent prepares a plan instead; you see what it
  creates, replaces, and deletes, and approving it makes one git commit.
  `atlas-obsidian undo` takes that commit back by restoring the files it touched,
  and refuses if you have changed one of them since.
- **The folder is yours.** Anything you typed in Obsidian is committed first, as
  its own operation, so an undo can never reach your edits. Every git command an
  operation runs is limited to `wiki/`, `.raw/`, `inbox/`, and `project.json`, so
  an apply never stages your source code.
- **Code owns what code can derive.** The wiki's log, the ledger of sources, the
  thread cards, the board that lists them, and the opening callout of each thread
  document are all written by the binary. The agent writes the prose.
- **Ids travel; paths stay.** A project's `project.json` holds no path. The paths
  live in `~/.atlas-obsidian/config.json`, which `init` appends to. The atlas
  never scans your disk, so a project can live anywhere. Move the folder, and the
  next session started inside it repairs the entry.

## Documentation

- Every command and its options — [docs/usage.md](docs/usage.md)
- How the engine works and why — [docs/core-design.md](docs/core-design.md)
- The layout of a project — [docs/v4-design.md](docs/v4-design.md)
- How threads work — [docs/threads-design.md](docs/threads-design.md)
- The skills, how they fit, and what each owns — [docs/skills.md](docs/skills.md)
- One wiki over several projects — [docs/members-design.md](docs/members-design.md)

External: [Obsidian](https://obsidian.md) ·
[Claude Code plugins](https://code.claude.com/docs/en/plugins) ·
[Codex plugin packaging](https://developers.openai.com/plugins/build/plugins) ·
[MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)

## Credits

The vault layout, the inbox workflow, and the skills derive from
[claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian) by
AgriciDaniel (MIT), which follows
[Andrej Karpathy's LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f).
Obsidian syntax references draw on
[kepano/obsidian-skills](https://github.com/kepano/obsidian-skills).
atlas-obsidian is released under the [MIT License](LICENSE).
