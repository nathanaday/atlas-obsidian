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
`vault new` makes another vault in one confirmation. `refresh` reads every
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
- Go 1.24 or newer, to build the binary

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
  atlas vault      create     ~/.claude-atlas/atlas
  vaults dir       use        ~/Documents/Vaults
  first vault      create     ~/Documents/Vaults/welcome (claude-obsidian init)

Proceed? [Y/n]
```

The claude-obsidian step runs the two commands from that project's own install
guide: `claude plugin marketplace add` and `claude plugin install`. The plugin
carries the whole product, skills and CLI alike, so nothing else is fetched.

When setup finishes it prints the atlas path and your first vault's path. Open
either one in Obsidian with "Open folder as vault". Run setup again at any
time; finished steps are skipped.

### Usage

Create a vault. It lands in your vaults directory, the plan is shown, you
confirm once, and the vault is registered in the atlas:

```bash
claude-atlas vault new sensor-triage --parent work --purpose "Sort field sensor faults."
```

Register a vault you already have:

```bash
claude-atlas vault add ~/Documents/OldVault --priority high
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
~/.claude-atlas/
├── config.json               vaults directory, plugin id, marketplace
└── atlas/                    an Obsidian vault
    ├── Atlas.md              generated: one table across every vault
    └── tree/
        └── work/
            └── sensor-triage/
                ├── node.md       you write this: purpose, priority, blockers
                ├── state.json    refresh writes this: heat, idle days, counts
                └── outputs/      decks, images, exports for this project
```

`node.md` is intent. Its frontmatter holds the fields the atlas reads, so you
can edit priority and state in Obsidian's property panel; the body is yours.
`state.json` is observation. They are separate files so a stale tracker can
never masquerade as a fresh one. The most useful line on the atlas page is
where the two disagree: a vault you marked `high` that has been cold for six
weeks.

The tree is plain directories. To move a project under a different area, `mv`
its directory and run `refresh`.

## Configuration

Everything lives in `~/.claude-atlas/config.json`:

```json
{
  "schema": "claude-atlas.config.v1",
  "vaults_dir": "/Users/you/Documents/Vaults",
  "atlas_vault": "/Users/you/.claude-atlas/atlas",
  "claude_obsidian": {
    "plugin": "claude-obsidian@agricidaniel-claude-obsidian",
    "marketplace": "AgriciDaniel/claude-obsidian"
  }
}
```

Set `claude_obsidian.path` to a claude-obsidian checkout to use it instead of
the installed plugin. `CLAUDE_ATLAS_HOME` or `--home` moves the home
directory.

## Conventions

Full reasoning in the [design spec](docs/spec.md).

- The atlas never writes into a vault. `vault new` delegates every write to
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
