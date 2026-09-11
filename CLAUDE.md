# claude-atlas

A CLI that wraps claude-obsidian: it installs a pinned release, creates vaults
in one confirmation, and reports on every vault from one Obsidian page.

Read `README.md` first. This file holds what the code and README do not say.

## Sources of truth

| Thing | Location |
|---|---|
| Design spec (brainstorm, not a contract) | `docs/spec.md` |
| Setup experience notes | `docs/setup-experience.md` |
| The product atlas wraps | `~/.claude-atlas/claude-obsidian` after setup; a dev checkout is at `~/claude-obsidian` |
| The user's real vault | `~/Documents/MyKnowledgeVault` |

## Why the project exists

claude-obsidian is good and the author wants to use it as-is. Two things hurt:
vault init takes several commands with copied hashes and timestamps, and
nothing shows many vaults at once. Atlas is the fix for both, built beside the
product and never inside it. Setup quality comes first; decisions made there
guide the rest.

## Three rules

1. Atlas never writes into a vault. `vault new` delegates every write to
   claude-obsidian's own init; after that atlas only reads.
2. A vault never learns that atlas exists. A leaf records a vault path; the
   vault records nothing.
3. Atlas never stores a fact it can compute. `node.json` is authored,
   `state.json` is derived and regenerated in full by `refresh`.

## Layout

```
claude_atlas/
  cli.py       argparse; one cmd_* per subcommand
  wizard.py    the setup flow
  product.py   pinned release: download, sha256 verify, extract, run the CLI
  plugin.py    claude plugin marketplace add + install
  tree.py      node.json / state.json
  refresh.py   derive state, render Atlas.md
  vaults.py    vault new / vault add
  home.py      ~/.claude-atlas and config.json
```

`~/.claude-atlas/` is tool state and is per machine; `atlas/` inside it is a
plain Obsidian vault (not a claude-obsidian wiki) so the user can open the
generated table.

## Constraints

- Runtime is standard library only. pytest is the one dev dependency.
- Python 3.11+, matching claude-obsidian.
- Bump `product.PRODUCT_VERSION` and `product.RELEASE_SHA256` together. The
  hash is the sha256 of the release zip. Setup refuses a mismatch.
- `refresh` is read-only toward every vault, offline, and idempotent.
- No test may read or write a real `~/.claude-atlas` or a real vault. Tests
  that need the product find it through `tests/conftest.py` and skip otherwise.
  Tests never download.

## claude-obsidian facts verified against v2.2.0

- The CLI is `python3 <release>/scripts/claude-obsidian.py`. Nothing is on
  PATH.
- `init` needs the vault's parent directory to exist.
- `init` is dry-run by default and prints `approved_plan_sha256`; apply repeats
  the same `--generated-at` and `--operation-id` plus the hash.
- `doctor --vault` exits 1 with JSON when a check fails; parse stdout anyway.
- `lint --format json` returns full finding lists; counts live under
  `summary.category_counts` and `summary.pages_scanned`.
- `seed_pages` is not a lint category; atlas counts `status: seed` frontmatter
  itself.
- The release zip contains `.claude-plugin/marketplace.json`, so the extracted
  directory is a valid local marketplace named `agricidaniel-claude-obsidian`.
- `wiki/hot.md` "Active Threads" is prose; treat it as best effort.

## Running tests

```
python3 -m pytest
```

Integration tests need an extracted release. After `setup` they find it in
`~/.claude-atlas`; otherwise set `CLAUDE_ATLAS_TEST_PRODUCT` and
`CLAUDE_ATLAS_TEST_RELEASE_ZIP`.

## Open questions

- Should `node.json` become `node.md` with frontmatter, so intent is edited in
  Obsidian's property panel? It would need a small frontmatter parser.
- Registration: hand-written only, or a `scan` that finds
  `.claude-obsidian.json` files and proposes them?
- Archived leaves: hidden or dimmed on the atlas page?
- A Claude Code skill for the bird's-eye conversation is the planned next step
  after setup settles; MCP is not planned.
