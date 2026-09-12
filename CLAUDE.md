# claude-atlas

A Go CLI that wraps claude-obsidian: installs it through Claude Code, creates
vaults in one confirmation, and reports on every vault from one Obsidian page.

Read `README.md` first. This file holds what the code and README do not say.

## Sources of truth

| Thing | Location |
|---|---|
| Design spec (brainstorm, not a contract) | `docs/spec.md` |
| Setup experience notes | `docs/setup-experience.md` |
| The product atlas wraps | the installed plugin, `~/.claude/plugins/cache/agricidaniel-claude-obsidian/claude-obsidian/<version>/` |

## Why the project exists

claude-obsidian is good and the author wants to use it as-is. Two things hurt:
vault init takes several commands with copied hashes and timestamps, and
nothing shows many vaults at once. Atlas is the fix for both, built beside the
product and never inside it. Setup quality comes first.

The author is moving into a fresh set of vaults. Migration of old vaults is
out of scope; `new-vault --from` exists only for vaults made by hand later.

## Three rules

1. Atlas never writes into a vault. `new-vault` delegates every write to
   claude-obsidian's own init; after that atlas only reads.
2. A vault never learns that atlas exists. A leaf records a vault path; the
   vault records nothing.
3. Atlas never stores a fact it can compute. Project pages under `tree/` are
   authored; `~/.claude-atlas/state/` is derived and rebuilt in full by
   `refresh`.

## Layout

```
cmd/claude-atlas/       main
internal/cli/           argument parsing and one method per subcommand
internal/wizard/        the setup flow
internal/claudecode/    Claude Code's plugin registry and `claude plugin`
internal/product/       locate and run the claude-obsidian CLI
internal/tree/          project pages (frontmatter) and derived state files
internal/refresh/       derive state, render Overview.md
internal/pages/         About.md and Reference.md from templates/*.md, paths filled in at write time
internal/vaults/        create and register vaults (new-vault)
internal/tui/           Bubble Tea screens: new-vault, manage-vaults, view
internal/home/          ~/.claude-atlas and config.json
internal/console/       prompts and step lines
internal/testutil/      finds a real claude-obsidian for integration tests
```

`~/.claude-atlas/` holds only internal state the user rarely opens: config and
derived state. Anything the user views lives under `~/Documents`: the atlas
vault (default `~/Documents/Atlas`, a plain Obsidian vault, not a
claude-obsidian wiki) and the vaults directory (default `~/Documents/Vaults`).

The tree is the user's: under `tree/`, every folder is a category with no data
of its own, and every markdown file is a project pointing at one vault. Users
make, nest, and move these by hand in Obsidian. `refresh` must tolerate any
file it cannot read as a project and report it instead of failing.

## Constraints

- Dependencies: `gopkg.in/yaml.v3` for project frontmatter (Obsidian's property
  editor writes real YAML), and Bubble Tea, Bubbles, and Lip Gloss for the
  interactive screens. Nothing else. Keep `go.mod` at the lowest Go version
  those need; do not pull `golang.org/x/*` modules at `@latest`, they can
  require a newer Go than the rest of the graph.
- TUI models keep all logic in `Update`, so tests drive them with `tea.KeyMsg`
  values and read `View()`; nothing in `internal/tui` touches a terminal
  except `Run*`.
- `product.TestedVersion` names the claude-obsidian release atlas was verified
  against. Atlas does not pin the install; it warns when the versions differ.
- `refresh` is read-only toward every vault, offline, and idempotent.
- Atlas edits a project page only through `tree.UpdateFrontmatter`, which
  changes the named keys and nothing else: other properties, comments, key
  order, and the body survive. Never re-render a page from the struct.
- Moving a vault directory happens only from `manage-vaults` after an explicit
  y/n; repointing to a vault that already exists needs no confirmation.
- Tests never install a plugin or touch a real `~/.claude-atlas`. Integration
  tests find claude-obsidian through `internal/testutil` (the installed plugin,
  or `CLAUDE_ATLAS_TEST_PRODUCT`) and skip otherwise.
- Prose follows the user's global writing guide: short sentences, active voice,
  no stock phrases.

## claude-obsidian facts verified against v2.2.0

- A marketplace install copies the whole release into the plugin cache,
  including `claude_obsidian/` and `scripts/claude-obsidian.py`. The skills
  call that script through `${CLAUDE_PLUGIN_ROOT}`.
- The GitHub repo's `main` branch is a valid marketplace named
  `agricidaniel-claude-obsidian`.
- `installed_plugins.json` records `installPath` and `version` per plugin id,
  as a list or a single object.
- `init` needs the vault's parent directory to exist.
- `init` is dry-run by default and prints `approved_plan_sha256`; apply repeats
  the same `--generated-at` and `--operation-id` plus the hash.
- `doctor --vault` exits 1 with JSON when a check fails; parse stdout anyway.
- `lint --format json` returns full finding lists; counts live under
  `summary.category_counts` and `summary.pages_scanned`.
- `seed_pages` is not a lint category; atlas counts `status: seed` frontmatter
  itself.
- `wiki/hot.md` "Active Threads" is prose; treat it as best effort.
- `obsidian://open?path=` only opens vaults Obsidian already knows. It cannot
  register a new vault, so atlas never tries to launch Obsidian.

## Build and test

```
make build      # bin/claude-atlas
make install    # go install into $(go env GOPATH)/bin
make test
```

## Open questions

- Registration: hand-written only, or a `scan` that finds
  `.claude-obsidian.json` files and proposes them?
- Archived leaves: hidden or dimmed on the atlas page?
- A Claude Code skill for the bird's-eye conversation is the planned next step;
  MCP is not planned.
- Distribution: a Homebrew tap once the command set settles.
