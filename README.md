# claude-atlas

Knowledge vaults for Claude Code, and one view across all of them.

[![License: MIT](https://img.shields.io/badge/license-MIT-2563eb.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8.svg?logo=go&logoColor=white)](go.mod)
[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-7c3aed.svg)](.claude-plugin/plugin.json)
[![Version](https://img.shields.io/badge/version-0.2.1-d97745.svg)](.claude-plugin/plugin.json)

## About

Reading piles up faster than notes get written, and what Claude learns in one
session is gone by the next. claude-atlas gives Claude Code a place to put
knowledge that lasts: an Obsidian vault it can fill and consult.

Drop a paper, a transcript, or a design note into the vault's inbox. Claude
reads it, writes linked pages that cite the source, and files the original
where it cannot change. Ask the vault a question later and the answer cites
its own pages. Every change Claude makes is a plan you see first and one git
commit you can undo.

Vaults work best small: one course, one project, one area of life. The atlas
is the page that shows all of them at once, so a free afternoon starts with a
glance rather than a search.

- **Ingest from an inbox.** Files in, cited pages out. Sources are kept as
  immutable copies; the inbox holds only what is still waiting.
- **Ask the vault.** Answers come from the pages and name their evidence, or
  say what is missing.
- **Review, then commit.** Claude builds a plan, you see what it will create,
  update, and remove, then the change lands as one commit. Undo reverts it.
- **Your edits are safe.** Anything you change in Obsidian is committed under
  its own message before an operation runs.
- **Health check.** Dead links, orphans, pages missing from the index, empty
  sections, and frontmatter gaps, on demand.
- **One view across vaults.** Heat, idle days, open threads, unfinished work,
  and the priority you declared, side by side, with linked git repos and
  material folders counted as activity.
- **Native Obsidian.** Plain Markdown and JSON, wikilinks, callouts, Canvas
  boards, and Bases views.

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
claude-atlas plugin into Claude Code, creates the atlas vault, and creates your
first knowledge vault:

```
  home             create     ~/.claude-atlas
  plugin           install    claude-atlas@nathanaday-claude-atlas from nathanaday/claude-atlas via `claude plugin`
  atlas vault      create     ~/Documents/Atlas
  vaults dir       use        ~/Documents/Vaults
  first vault      create     ~/Documents/Vaults/welcome

Proceed? [Y/n]
```

Run setup again at any time; finished steps are skipped.

## Usage

Create a vault. With no arguments, `new-vault` asks for a name, a category, and
a one-line purpose:

```bash
claude-atlas new-vault
claude-atlas new-vault sensor-triage --category work --purpose "Sort field sensor faults."
```

Put a source in the inbox and start Claude Code inside the vault:

```bash
cp ~/Downloads/dinov2.pdf ~/Documents/Vaults/sensor-triage/inbox/
claude-atlas open-claude sensor-triage
```

In the session, the skills are on the slash menu:

```text
/claude-atlas:wiki-ingest     read what is in the inbox and write pages
/claude-atlas:wiki-query      answer from the vault, with citations
/claude-atlas:save            keep an answer or decision as a page
/claude-atlas:wiki-lint       check the wiki's health
```

Claude shows a preview of every change before it applies it. Look at what
happened, or take an operation back, from the terminal:

```bash
claude-atlas history sensor-triage
claude-atlas undo sensor-triage ingest-20260912-150405-ab12
```

Open a vault, or the atlas, in Obsidian:

```bash
claude-atlas open-vault sensor-triage
claude-atlas open-vault
```

See every vault at once. `view` is an interactive tree: `o` opens a vault in
Obsidian, `c` starts Claude Code in it. `refresh` rebuilds the overview page.

```bash
claude-atlas view
claude-atlas refresh
```

Link the folders a project works with. Commits in a linked repo and new files
in a linked folder count as activity on the project:

```bash
claude-atlas link sensor-triage ~/code/sensor-triage
claude-atlas link sensor-triage ~/Documents/sensor-datasheets
```

Bring in a vault you already have, including one made with claude-obsidian:

```bash
claude-atlas adopt ~/Documents/MyKnowledgeVault --category personal
```

Check the installation:

```bash
claude-atlas doctor
```

## Inside a vault

```
sensor-triage/
├── inbox/                    sources waiting to be ingested
├── .raw/captured/            immutable copies of ingested sources
├── .git/                     one commit per operation
└── wiki/
    ├── index.md              the catalog
    ├── log.md                what happened, newest first
    ├── hot.md                recent context, handed to Claude at session start
    ├── overview.md           the stable big picture
    ├── sources/ entities/ concepts/ questions/ sessions/
    └── meta/ledgers/source-ledger.json
```

Two filing modes: `generic` files pages by type into the folders above; `lyt`
keeps atomic notes in `wiki/notes/` and navigates them through Maps of Content.
Change it with `claude-atlas mode sensor-triage lyt`.

## The atlas

```
~/Documents/Atlas/            an Obsidian vault; open it like any other
├── Overview.md               every project, its heat, threads, and signals
├── About.md                  orientation
├── Reference.md              every command with examples
└── tree/                     yours: folders are categories, files are projects
```

Each project page under `tree/` holds your intent: purpose, priority, state,
what it is blocked on. Everything else is derived from the vault on every
refresh, so the overview never goes stale. The most useful line on it is where
the two disagree: a project marked `high` whose vault has been cold for weeks.

## Conventions

Full reasoning in [docs/core-design.md](docs/core-design.md).

- One operation, one commit. Claude plans, you review, the core commits.
  Claude's own file tools are refused inside the wiki.
- Your edits come first. Hand edits are committed before any operation, so
  undo never touches them.
- The core owns what it can derive: the log, the source ledger, the history,
  the health check. Claude writes pages.
- The atlas never writes into a vault, and no vault knows the atlas exists.

## Documentation

- Design and decisions — [docs/core-design.md](docs/core-design.md)
- Skills — [skills/](skills/), one `SKILL.md` per skill
- Plugin manifest — [.claude-plugin/plugin.json](.claude-plugin/plugin.json)

External:

- Obsidian — https://obsidian.md
- Claude Code plugins — https://code.claude.com/docs/en/plugins
- MCP Go SDK — https://github.com/modelcontextprotocol/go-sdk

## Credits

The vault layout, the inbox workflow, and the skills derive from
[claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian) by
AgriciDaniel (MIT), which follows
[Andrej Karpathy's LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f).
Obsidian syntax references draw on
[kepano/obsidian-skills](https://github.com/kepano/obsidian-skills).
claude-atlas is released under the [MIT License](LICENSE).
