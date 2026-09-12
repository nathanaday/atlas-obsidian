# claude-atlas

One view across many [claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian)
vaults, and a painless way to create them.

## About

claude-obsidian turns Claude Code into a careful knowledge worker: every vault
keeps its sources, cites its claims, logs its operations, and lints itself. It
works best when each vault is small and about one thing: one course, one paper,
one project.

Two things get in the way once you have more than one vault.

Creating a vault takes several commands. The CLI is not on your PATH, the
dry-run prints a long plan, and you copy a hash and a timestamp from that plan
into the apply command. It is safe, but it is enough friction to make you
hesitate before starting a new one.

Nothing shows all your vaults at once. Each vault knows its own state; none of
them can answer "I have a free afternoon, what should I pick up?"

claude-atlas fixes both. One `setup` command installs the claude-obsidian
plugin into Claude Code, creates the atlas, and creates your first vault.
`new-vault` makes another vault in one confirmation. `refresh` reads every
vault and writes a single page, the atlas, that you open in Obsidian: heat,
idle days, open threads, unfinished work, and your own declared priority for
each vault, side by side.

The atlas never writes into a vault, and no vault knows the atlas exists.
Delete the atlas and every vault is untouched.

## Quickstart

### Prerequisites

- [Claude Code](https://claude.com/claude-code); the `claude` command must be on your PATH
- [Obsidian](https://obsidian.md)
- Python 3.11 or newer, which claude-obsidian itself runs on
- Go 1.24.2 or newer, to build the binary

### Install

```bash
git clone <this repository> claude-atlas
cd claude-atlas
go install ./cmd/claude-atlas
```

`go install` puts the binary in `$(go env GOPATH)/bin`, usually `~/go/bin`.
Add that directory to your PATH if it is not there yet.

### Setup

```bash
claude-atlas setup
```

Setup shows its plan and asks before it does anything:

```
claude-atlas setup

  home             create     ~/.claude-atlas
  claude-obsidian  install    claude-obsidian@agricidaniel-claude-obsidian from AgriciDaniel/claude-obsidian via `claude plugin`
  atlas vault      create     ~/Documents/Atlas
  vaults dir       use        ~/Documents/Vaults
  first vault      create     ~/Documents/Vaults/welcome (claude-obsidian init)

Proceed? [Y/n]
```

The claude-obsidian step runs the two commands from that project's own install
guide: `claude plugin marketplace add` and `claude plugin install`. The plugin
carries the whole product, skills and CLI alike, so nothing else is fetched.

When setup finishes it prints the atlas path and your first vault's path. Run
`claude-atlas open-vault` to open the atlas in Obsidian. Run setup again at
any time; finished steps are skipped.

### Usage

Create a vault. With no arguments, `new-vault` walks you through it: type a
name, pick a category from the ones in your tree or name a new one, add a
one-line purpose, confirm. The vault is created in your vaults directory and
its project page lands in the tree.

```bash
claude-atlas new-vault
```

The same thing without prompts, for scripts:

```bash
claude-atlas new-vault sensor-triage --category work --purpose "Sort field sensor faults."
```

Open the atlas, or a project's vault, in Obsidian. Obsidian only opens folders it
already knows, so the command offers to register the folder. Obsidian reads that
list only at launch, so it quits and relaunches when it is running. The registry
write is validated, backed up, and atomic; if the file does not look as expected
nothing is written and the folder is revealed for "Open folder as vault" instead.

```bash
claude-atlas open-vault
claude-atlas open-vault sensor-triage
```

Navigate the atlas as a tree. Categories nest three layers deep on screen;
anything deeper opens on Enter. Each project is a card with its heat, page
count, and unfinished work; Enter shows every detail and `o` opens the vault in
Obsidian.

```bash
claude-atlas view
```

Browse and edit what is registered. Open a project to rename it, change its
purpose, priority, or state, move it to another category, repoint or move its
vault, or remove it from the atlas. Removing never touches the vault on disk.

```bash
claude-atlas manage-vaults
```

Register a vault you already have:

```bash
claude-atlas new-vault --from ~/Documents/OldVault --priority high
```

Rebuild the atlas page from every vault:

```bash
claude-atlas refresh
```

Show every path the atlas uses, and the claude-obsidian version it found:

```bash
claude-atlas info
```

Start working in a vault with Claude Code:

```bash
cd ~/Documents/Vaults/sensor-triage && claude
# then /claude-obsidian:wiki
```

Check the installation:

```bash
claude-atlas doctor
```

## What the atlas holds

```
~/Documents/Atlas/            an Obsidian vault; open it like any other
├── Overview.md               generated: every project, its heat, threads, and signals
├── About.md                  orientation: what each part of the atlas is
├── Reference.md              every claude-atlas command with examples
└── tree/                     yours: folders are categories, files are projects
    ├── university/
    │   └── cs566/
    │       ├── capstone.md
    │       └── course-material.md
    └── personal/
        └── reading-list.md

~/Documents/Vaults/           where `new-vault` puts each vault

~/.claude-atlas/              internal; you rarely open this
├── config.json               paths, plugin id, marketplace
└── state/                    derived state per project, mirroring tree/; safe to delete
```

Each project page is intent. Its frontmatter holds the fields the atlas
reads, so you edit priority and state in Obsidian's property panel; the body
is yours. The derived state lives outside the vault and is rebuilt on every
refresh, so a stale tracker can never masquerade as a fresh one. The most
useful line on the overview is where the two disagree: a project you marked
`high` whose vault has been cold for six weeks.

Arrange `tree/` however you think: make folders, nest them, move files
between them. Run `refresh` afterward.

## Configuration

Everything lives in `~/.claude-atlas/config.json`:

```json
{
  "schema": "claude-atlas.config.v1",
  "vaults_dir": "/Users/you/Documents/Vaults",
  "atlas_vault": "/Users/you/Documents/Atlas",
  "claude_obsidian": {
    "plugin": "claude-obsidian@agricidaniel-claude-obsidian",
    "marketplace": "AgriciDaniel/claude-obsidian"
  }
}
```

Setup asks for both directories and accepts `--atlas-vault` and
`--vaults-dir`. Set `claude_obsidian.path` to a claude-obsidian checkout to
use it instead of the installed plugin. `CLAUDE_ATLAS_HOME` or `--home` moves
the home directory.

## Conventions

Full reasoning in the [design spec](docs/spec.md).

- The atlas never writes into a vault. `new-vault` delegates every write to
  claude-obsidian's own init; after that the atlas only reads.
- A vault never learns the atlas exists. A leaf records a path to a vault; the
  vault records nothing.
- The atlas never stores a fact it can compute. Derived state is regenerated on
  every refresh and safe to delete.
- claude-obsidian is installed once, through Claude Code, the way its own
  documentation describes. The atlas finds it there and never carries a copy.

## Documentation

- Design spec — [docs/spec.md](docs/spec.md)
- Setup experience notes — [docs/setup-experience.md](docs/setup-experience.md)
- claude-obsidian — https://github.com/AgriciDaniel/claude-obsidian
