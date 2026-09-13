# claude-atlas

Knowledge vaults for Claude Code, and one view across all of them.

## About

A vault is an Obsidian folder that Claude Code fills with source-cited pages.
You drop a paper, a transcript, or a design note into the vault's inbox; Claude
reads it, writes linked pages, and files the source. Later you ask the vault a
question and get an answer that cites its own pages. Every change is one
reviewed operation and one git commit, so nothing is lost and anything can be
undone.

Vaults work best when each is small and about one thing: one course, one
project, one area of life. claude-atlas makes a new one in one confirmation and
keeps a single page, the atlas, that shows every vault side by side: how recently
it moved, what is unfinished, and the priority you gave it.

The workflow and the skills come from
[claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian); see
Lineage below. claude-atlas replaces its Python core with a Go binary and an
MCP server, and uses git in the vault as the safety mechanism.

## Quickstart

### Prerequisites

- [Claude Code](https://claude.com/claude-code) with the `claude` command on
  your PATH
- [Obsidian](https://obsidian.md)
- git
- Go 1.24 or newer, to build the binary

### Install

```bash
git clone https://github.com/nathanaday/claude-atlas.git
cd claude-atlas
go install ./cmd/claude-atlas
```

`go install` puts the binary in `$(go env GOPATH)/bin`, usually `~/go/bin`. Add
that directory to your PATH if it is not there.

### Setup

```bash
claude-atlas setup
```

Setup shows its plan and asks before it does anything:

```
claude-atlas setup

  home             create     ~/.claude-atlas
  plugin           install    claude-atlas@nathanaday-claude-atlas from nathanaday/claude-atlas via `claude plugin`
  atlas vault      create     ~/Documents/Atlas
  vaults dir       use        ~/Documents/Vaults
  first vault      create     ~/Documents/Vaults/welcome

Proceed? [Y/n]
```

The plugin step registers this repository as a Claude Code marketplace and
installs the plugin from it. The plugin holds the skills, the hooks, and a
small script that runs the binary you installed; the binary serves the MCP
tools. Run setup again at any time; finished steps are skipped.

### Usage

Create a vault. With no arguments, `new-vault` walks you through it: a name, a
category from your tree or a new one, a one-line purpose, confirm.

```bash
claude-atlas new-vault
claude-atlas new-vault sensor-triage --category work --purpose "Sort field sensor faults."
```

Put a source in the vault's `inbox/` and start Claude Code inside the vault:

```bash
cp ~/Downloads/dinov2.pdf ~/Documents/Vaults/sensor-triage/inbox/
claude-atlas open-claude sensor-triage
```

Then, in the session:

```text
/claude-atlas:wiki-ingest
```

Claude captures the file, reads it, drafts pages, and shows you a preview of
what it will create and update before it applies anything. Ask the vault
something with `/claude-atlas:wiki-query`, keep an answer with
`/claude-atlas:save`, check the wiki's health with `/claude-atlas:wiki-lint`.

Open a vault, or the atlas, in Obsidian:

```bash
claude-atlas open-vault sensor-triage
claude-atlas open-vault
```

See every vault at once:

```bash
claude-atlas view          # navigate the tree; o opens Obsidian, c opens Claude Code
claude-atlas refresh       # rebuild Overview.md from every vault
```

Bring in a vault you already have, including one made by claude-obsidian:

```bash
claude-atlas adopt ~/Documents/MyKnowledgeVault --category personal
```

Look at a vault's history, or take an operation back:

```bash
claude-atlas history sensor-triage
claude-atlas undo sensor-triage ingest-20260912-150405-ab12
```

Check the installation:

```bash
claude-atlas doctor
```

## What a vault holds

```
sensor-triage/
├── .claude-atlas.json        identity and filing mode
├── .git/                     one commit per operation
├── inbox/                    sources waiting to be ingested
├── .raw/captured/            immutable copies of ingested sources
└── wiki/
    ├── index.md              the catalog, kept current by every operation
    ├── log.md                what happened, newest first; written by the core
    ├── hot.md                recent context, handed to Claude at session start
    ├── overview.md           the stable big picture
    ├── sources/ entities/ concepts/ questions/ sessions/
    └── meta/ledgers/source-ledger.json
```

Everything is plain Markdown and JSON you can open in Obsidian. Two filing
modes exist: `generic` files pages by type into the folders above; `lyt` keeps
atomic notes in `wiki/notes/` and navigates them through Maps of Content.

## What the atlas holds

```
~/Documents/Atlas/            an Obsidian vault; open it like any other
├── Overview.md               generated: every project, its heat, threads, and signals
├── About.md                  orientation
├── Reference.md              every command with examples
└── tree/                     yours: folders are categories, files are projects

~/Documents/Vaults/           where new-vault puts each vault
~/.claude-atlas/              config and derived state; rarely opened
```

Each project page under `tree/` is intent: purpose, priority, state, what it
is blocked on. The derived state is rebuilt on every refresh. The most useful
line on the overview is where the two disagree: a project marked `high` whose
vault has been cold for six weeks.

## Configuration

`~/.claude-atlas/config.json`:

```json
{
  "schema": "claude-atlas.config.v1",
  "vaults_dir": "/Users/you/Documents/Vaults",
  "atlas_vault": "/Users/you/Documents/Atlas",
  "plugin": {
    "id": "claude-atlas@nathanaday-claude-atlas",
    "source": "nathanaday/claude-atlas"
  },
  "claude_code": {
    "command": "claude",
    "session_context": true
  }
}
```

`claude_code.prompt` sends a first message on every `open-claude`, for example
`/claude-atlas:wiki`. `session_context` controls whether the plugin's
session-start hook hands Claude the vault's `hot.md`. `plugin.source` can be a
local checkout while developing. `CLAUDE_ATLAS_HOME` or `--home` moves the
home directory.

## Conventions

Full reasoning in [docs/core-design.md](docs/core-design.md).

- One operation, one commit. Claude builds a plan, you see the preview, the
  core commits. Write and Edit are refused under `wiki/` by a hook.
- The vault is yours. Edits you make in Obsidian are committed under their own
  message before an operation runs, so undo never touches them.
- Code owns what code can derive: the log, the source ledger, the history, the
  lint report. Claude writes pages.
- The atlas never writes into a vault, a vault never learns the atlas exists,
  and the atlas never stores a fact it can compute.

## Documentation

- Core design — [docs/core-design.md](docs/core-design.md)
- Original spec — [docs/spec.md](docs/spec.md)
- Skills — [skills/](skills/), one `SKILL.md` per skill
- Plugin manifest — [.claude-plugin/plugin.json](.claude-plugin/plugin.json)

## Lineage

The vault layout, the inbox workflow, the operation discipline, and the skills
derive from [claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian)
by AgriciDaniel, MIT licensed, whose design follows
[Andrej Karpathy's LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f).
The Obsidian syntax references draw on
[kepano/obsidian-skills](https://github.com/kepano/obsidian-skills).
claude-atlas is MIT licensed.
