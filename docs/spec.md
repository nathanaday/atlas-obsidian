---
title: "Portfolio Fabric Spec"
type: concept
status: developing
created: 2026-09-11
updated: 2026-09-11
tags:
  - spec
  - portfolio
  - atlas
  - claude-obsidian
  - workflow
domain: Tooling
aliases:
  - portfolio fabric
  - atlas
  - the fabric
---

## Purpose

Many small vaults, one view across them.

Each vault stays self-contained: one course, one paper, one project, one area of
life. A separate tool sits above them and answers portfolio questions that no
single vault can answer. The driving question is:

> I have a free afternoon. What should I pick up?

A correct answer needs the state of every vault at once, plus intent that no
vault records: what matters right now, and why.

**Name: `atlas`** — a bound collection of maps, which is what this is. It lives
at `~/.atlas/` as a standalone project directory. It is not a vault and has no
wiki of its own.

## What the tool already gives us

claude-obsidian has no cross-vault feature. `role` accepts only the value
`vault`, and the product's own test suite treats vault isolation as a safety
property to enforce. Atlas is therefore new work, built beside the product and
never inside it.

That costs nothing, because every vault already publishes what atlas needs at
fixed paths:

| Signal | Source | Machine-readable |
|---|---|---|
| Identity | `.claude-obsidian.json` | yes |
| Operation history | `wiki/log.md` | headings only |
| Open threads | `wiki/hot.md`, "Active Threads" | prose |
| Unfinished work | `lint --format json` | yes |
| Page maturity | frontmatter `status` | yes |
| Hand-editing | file mtimes under `wiki/` | yes |

## Three rules

**1. Atlas never writes into a vault.**

It reads. It never applies a transaction, never adds a file, never asks a vault
to carry portfolio metadata. A vault must remain valid and complete when atlas
is deleted. This keeps claude-obsidian upgradable, keeps each vault portable,
and lets atlas read a vault owned by someone else.

**2. A vault never learns that atlas exists.**

The relationship points one way. A leaf node records a path to a vault; the
vault records nothing. Nothing inside a vault changes when it joins or leaves
the map, so a vault can be registered in a second atlas, or in none, with no
migration.

**3. Atlas never stores a fact it can compute.**

Every portfolio tracker fails the same way: the tracker goes stale, and the
thing meant to report staleness becomes the stalest object in the system. At
that point it lies with confidence, which is worse than silence.

So state splits by who can maintain it:

- **Authored** — priority, purpose, definition of done, what you are blocked on.
  A machine cannot derive these. They change monthly, not daily. You write them.
- **Derived** — last touched, heat, open counts, unfinished work. Never written
  by hand. Recomputed on every refresh, and safe to delete at any time.

The two live in separate files so the rule is enforceable by inspection.

## Structure

A tree of directories under `~/.atlas/tree/`. Every node carries the same two
files, so a reader needs one format regardless of depth.

```text
~/.atlas/
├── atlas.json                 tool settings
├── refresh                    the scraper
└── tree/
    ├── node.json              root
    ├── technical/
    │   ├── node.json
    │   ├── work/
    │   │   ├── node.json
    │   │   └── sensor-triage/          leaf
    │   │       ├── node.json           authored; points at a vault
    │   │       ├── state.json          derived
    │   │       └── outputs/            decks, images, exports
    │   ├── university/
    │   └── personal-projects/
    └── life/
        ├── node.json
        └── household/
```

**Leaf nodes** point at exactly one vault by absolute path. The vault lives
wherever it lives — beside a code repository, in a sync folder, on an external
disk — and nothing about atlas constrains that. **Cluster nodes** own only
children; their derived state aggregates the subtree.

A node keeps a directory rather than a line in a table for one reason:
`outputs/`. Decks, diagrams, and exports for one thing collect in one place,
next to the intent that produced them.

The filesystem carries the hierarchy, so re-parenting a node is `mv`.

### `node.json` — authored

```json
{
  "schema": "atlas.node.v1",
  "id": "sensor-triage",
  "name": "Sensor Triage",
  "kind": "leaf",

  "vault": "/Users/nathanaday/vaults/sensor-triage",

  "purpose": "One paragraph. Why this exists and what changes if it succeeds.",
  "definition_of_done": "What finished looks like. Empty for an open-ended area.",

  "priority": "high",
  "state": "active",
  "blocked_on": "",
  "review_after": "2026-10-01",

  "repos": ["https://github.com/example/sensor-triage"],
  "outputs": "outputs/"
}
```

`vault` is required on a leaf and absent on a cluster. `priority` is one of
`high`, `normal`, `low`, `someday`; `state` is one of `active`, `paused`,
`blocked`, `archived`.

`priority` and `state` are declarations of intent, not observations. A project
can be `priority: high` and stone cold; that gap is the most useful signal atlas
produces.

`review_after` nudges you to revisit intent. It is not a deadline.

### `state.json` — derived

Regenerated in full by `refresh`. Never hand-edited. Deleting it loses nothing.

```json
{
  "schema": "atlas.state.v1",
  "generated_at": "2026-09-11T20:14:03Z",
  "vault_ok": true,

  "last_operation": "2026-09-04",
  "last_touched": "2026-09-11",
  "days_idle": 0,
  "heat": "hot",

  "pages": 41,
  "open_threads": ["Seed the source ledger with the reading-list papers"],
  "unfinished": { "empty_sections": 41, "seed_pages": 8, "dead_links": 0 }
}
```

`heat` is `hot` under 7 days idle, `warm` under 30, `cold` at 30 or more.

`last_touched` must consider mtimes, not only the log. The log records only
transactions the agent applied; work you type by hand in Obsidian writes no log
entry, and counting it as idleness would make your own writing look like
neglect.

## The refresh command

A script. Deterministic, offline, read-only toward every vault. JSON throughout,
so it needs no parser beyond the standard library.

1. Walk `tree/` and load every `node.json`.
2. Reject the run if two leaves name the same vault path, or if a leaf names a
   path that is not a vault.
3. For each leaf, run `doctor` against the vault path. On failure record
   `vault_ok: false` and continue; one broken or unmounted vault must not stop
   the sweep.
4. Read `wiki/log.md` for the newest operation date; walk `wiki/` for the newest
   mtime; take the later of the two.
5. Run `lint --format json` and keep the summary counts.
6. Parse the "Active Threads" section of `wiki/hot.md`, best effort.
7. Write `state.json` at the leaf.
8. Roll subtree aggregates up into each cluster's `state.json`.

Refresh is idempotent: running it twice with no vault activity changes only
`generated_at`.

A vault that has moved shows as `vault_ok: false` rather than as an error,
because a pointer to a disconnected drive is a normal condition, not a fault.

## The bird's-eye view

A session opened at `~/.atlas/` reads every `node.json` and `state.json` —
roughly a dozen lines per node, so twenty projects fit in context with room to
spare — and answers in conversation. There is no right answer, only a discussion
informed by the whole picture.

The answer worth having comes from crossing the authored and derived halves:

```text
HOT   vlm-edge-benchmark   2d    3 open threads
HOT   MyKnowledgeVault     today 17 papers unread
WARM  work/sensor-triage   9d    1 open thread
COLD  genie-latent-actions 6w    stalled mid-draft
      ^ declared priority: high in March, untouched since
COLD  household            3w
```

That last pairing is the point. Staleness alone is noise; staleness against
declared priority is a decision.

## Deliberately excluded

- **Cross-vault retrieval.** Answering "which projects are similar" means
  indexing every vault into one searchable space, which destroys the isolation
  that makes small vaults worth having. When the question arises, run retrieval
  in the two or three vaults already suspected.
- **Content movement between vaults.** If atlas relocates knowledge, no vault is
  self-contained any more and the whole arrangement collapses back into one wiki
  managing everything.
- **Writing to vaults**, including a portfolio metadata file. See rules 1 and 2.
- **A wiki at the root.** Twenty short node files need no retrieval, no ledgers,
  and no transaction engine. Atlas is a plain project directory.

## Known risks

- **`hot.md` parsing is prose parsing.** "Active Threads" is a convention, not a
  schema, and a product update may change it. Treat open threads as best effort;
  keep every count that matters sourced from `lint --format json`, which is
  schema-backed.
- **Pointers break silently.** A vault can be renamed or moved with nothing to
  notice. `vault_ok` surfaces it at the next refresh, which is the only defense
  a one-way link allows, and an acceptable price for rule 2.
- **Node count.** The design holds at twenty leaves and fails at eighty. Keep
  the unit coarse: one course, not one lecture; one reading list, not one paper.
- **Authored fields rot too, quietly.** `priority` set once and never revisited
  is its own stale fact. `review_after` exists to surface that, and the
  bird's-eye view should report intent older than its review date as suspect.

## Decisions

| Question | Decision |
|---|---|
| Leaf holds its vault, or points at one? | **Points.** Absolute path in `node.json`; no structural coupling. |
| YAML or JSON? | **JSON.** No parser dependency in `refresh`. |
| Does the root need a vault? | **No.** A standalone project directory at `~/.atlas/`. |
| Where does output media live? | In the leaf's `outputs/`, gathered in one place. |

## Phases

1. Tree, both schemas, and `refresh`, against two or three real vaults.
2. The bird's-eye session at the root.
3. Cluster rollups, once enough leaves exist to need them.
4. An interactive view, published as an artifact, if the text view proves
   insufficient.

## Open questions

- **Registration.** Does a vault join the map only by hand-written `node.json`,
  or does atlas offer a `scan` that walks a directory for `.claude-obsidian.json`
  and proposes candidates? Hand-written is safer and slower.
- **Archived leaves.** Does `state: archived` drop a node out of the bird's-eye
  view entirely, or keep it visible and dimmed so cold work stays resurrectable?
