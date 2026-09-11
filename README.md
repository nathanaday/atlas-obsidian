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

claude-atlas fixes both. One `setup` command installs a pinned claude-obsidian
release, registers its skills with Claude Code, and creates your first vault.
`vault new` makes another vault in one confirmation. `refresh` reads every vault
and writes a single page, the atlas, that you open in Obsidian: heat, idle days,
open threads, unfinished work, and your own declared priority for each vault,
side by side.

The atlas never writes into a vault, and no vault knows the atlas exists.
Delete the atlas and every vault is untouched.

## Quickstart

### Prerequisites

- Python 3.11 or newer
- [Obsidian](https://obsidian.md)
- [Claude Code](https://claude.com/claude-code), so the `claude-obsidian` skills can be installed

### Setup

```bash
git clone <this repository> claude-atlas
cd claude-atlas
python3 -m claude_atlas setup
```

Setup shows its plan and asks before it does anything:

```
claude-atlas setup

  home             create     ~/.claude-atlas
  claude-obsidian  download   v2.2.0 (3.1 MB from github.com; verified by sha256)
  Claude Code      install    claude-obsidian@agricidaniel-claude-obsidian into Claude Code
  atlas vault      create     ~/.claude-atlas/atlas
  vaults dir       use        ~/Vaults
  first vault      create     ~/Vaults/welcome (claude-obsidian init)

Proceed? [Y/n]
```

When it finishes it prints two Obsidian links: the atlas, and your first vault.
Run it again at any time; finished steps are skipped.

To get a `claude-atlas` command on your PATH:

```bash
pip install -e .
```

### Usage

Create a vault. The plan is shown, you confirm once, and the vault is
registered in the atlas:

```bash
claude-atlas vault new sensor-triage --parent work --purpose "Sort field sensor faults."
```

Register a vault you already have:

```bash
claude-atlas vault add ~/Documents/MyKnowledgeVault --priority high
```

Rebuild the atlas page from every vault, then open it:

```bash
claude-atlas refresh
claude-atlas open
```

Start working in a vault with Claude Code:

```bash
cd ~/Vaults/sensor-triage && claude
# then /claude-obsidian:wiki
```

Check the installation:

```bash
claude-atlas doctor
```

## What the atlas holds

```
~/.claude-atlas/
├── config.json
├── claude-obsidian/          pinned release, verified against its SHA256SUMS
└── atlas/                    an Obsidian vault
    ├── Atlas.md              generated: one table across every vault
    └── tree/
        └── work/
            └── sensor-triage/
                ├── node.json     you write this: purpose, priority, blockers
                ├── state.json    refresh writes this: heat, idle days, counts
                └── outputs/      decks, images, exports for this project
```

`node.json` is intent. `state.json` is observation. They live in separate
files so a stale tracker can never masquerade as a fresh one. The most useful
line on the atlas page is where the two disagree: a vault you marked `high`
that has been cold for six weeks.

The tree is plain directories. To move a project under a different area, `mv`
its directory and run `refresh`.

## Conventions

Full reasoning in the [design spec](docs/spec.md).

- The atlas never writes into a vault. It reads. A vault stays complete when
  the atlas is deleted.
- A vault never learns the atlas exists. A leaf records a path to a vault; the
  vault records nothing.
- The atlas never stores a fact it can compute. Derived state is regenerated on
  every refresh and safe to delete.
- Standard library only. No parser dependency stands between you and your data.

## Documentation

- Design spec — [docs/spec.md](docs/spec.md)
- claude-obsidian — https://github.com/AgriciDaniel/claude-obsidian
- Obsidian URI scheme, used to open vaults — https://help.obsidian.md/Extending+Obsidian/Obsidian+URI
