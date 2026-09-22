---
type: plan
thread: thr-20260922-5519
title: "Consolidate overlapping member pages into the parent wiki"
created: 2026-09-22
---

> [!plan] Consolidate overlapping member pages into the parent wiki
> [Stub](<../stubs/Consolidate overlapping member pages into the parent wiki.md>) → [Spec](<../specs/Consolidate overlapping member pages into the parent wiki.md>) → **Plan** → Receipt
> `thr-20260922-5519` · [Thread](<../Consolidate overlapping member pages into the parent wiki.md>) · filed 2026-09-22

## Approach

Add one read-only package, `internal/overlap`, that indexes the hub's wiki as
lint sees it and scores every cross-origin pair of pages. Lint already parses
every page and resolves every link, so lint's `Vault` gains an exported
`Pages()` that hands over each page's title, aliases, tags, headings, masked
text, and resolved links; overlap tokenizes and scores over that, and never
parses markdown itself. The mirror learns one new frontmatter pair,
`moved_to` and `moved_to_project`, and treats a page that moved into this hub
as a redirect. The server gains the `overlap` tool and `project` on `plan`;
the CLI gains `overlap`; the skill `wiki-merge` runs the procedure. The
alternative, a `merge` tool that writes both repositories, would hide two
commits behind one call and would need its own preview; `plan` with
`project` keeps the contract the user already knows.

## Where the work lands

- `internal/lint/vault.go`: `Vault.Pages()` and a `PageInfo` type (path,
  title, type, aliases, tags, headings, text with code and frontmatter
  masked, links as written and resolved). `LoadVault` keeps the parsed pages.
- `internal/overlap/`: `Run(root, Options) (*Report, error)`. The index:
  origin per page (the hub, or the mirror folder), a token vector per page
  (TF-IDF over lowercased words of three letters or more minus stop words,
  with a light suffix stemmer; title, alias, and heading tokens counted
  three times), a name key per title and alias (lint's `nameKey`, exported
  or duplicated in three lines), and the set of link names each page uses.
  The scores: name (1.0 on an equal key across the two pages' titles and
  aliases, 0.8 within lint's edit-distance limit, else 0), content (cosine),
  links (Jaccard over link names). Combined `0.5·name + 0.35·content +
  0.15·links`; a pair appears at 0.2 and up, `duplicate` at name ≥ 0.8 or
  content ≥ 0.6. Settled: a hub page whose name key equals either page's, or
  a hub page whose resolved links reach both. Shared names: link names used
  by pages of two or more origins that resolve to no hub page or to a
  different page per origin. Shared tags likewise. Excluded pages: the
  index pages, `overview.md`, `hot.md`, `log.md`, the mirror index, each
  mirror's root page, and any page with `moved_to_project`. Report JSON and
  markdown, like lint. `Options{Member, Limit}`; the default limit is 30
  pairs, 30 names, 30 tags.
- `internal/mirror/mirror.go`: `MovedToKey`, `MovedToProjectKey`. In
  `mirrorMember`, a page whose `moved_to_project` is the hub's id is skipped,
  and the replace function maps a link that resolves to it to `moved_to`
  when that path exists in the hub. The member's index and other pages then
  link to the hub's page in the hub.
- `internal/txn/txn.go`: `Merge` kind, in `ModelKinds`, scoped like `save`.
- `internal/mcpserver/server.go`: `overlap` tool with `member` and `limit`;
  `PlanArgs` gains `ProjectArg`, and `plan` resolves the project through
  `projectOf`. `apply` already opens the plan's folder. `ToolNames` gains
  `overlap`; the `plan` description names `merge`.
- `internal/cli/cli.go`: `overlap [PROJECT] [--member NAME] [--json]`, in the
  wiki section of the usage text.
- `skills/wiki-merge/SKILL.md`: the procedure. `skills/wiki/SKILL.md` routes
  to it; `skills/atlas-project/SKILL.md` offers it after a sync; the
  operations reference lists `overlap` and the `merge` kind and `project` on
  `plan`; `hooks.Skills` lists it.
- `docs/usage.md`: the table row and a "Merge" subsection under members.
  `docs/members-design.md`: a "Merge" section with the pointer and the
  redirect. `CLAUDE.md`: the layout line for `internal/overlap` and the
  rule that a merge writes a member only as the member's own operation.
- `.claude-plugin/plugin.json` and `marketplace.json`: 5.5.0.

## Order

1. `lint.Vault.Pages()` with a test. Commit.
2. `internal/overlap` with its fixture tests and the markdown rendering.
   Commit.
3. The mirror's moved-page redirect with tests. The `merge` kind. Commit.
4. The `overlap` tool, `project` on `plan`, the CLI command, tests. Commit.
5. The skill, the references, the docs, the version bump. Commit.
6. Build, install, run `overlap` over usc-f26 read only, tune the
   thresholds if the report is wrong, and record the run in this plan.

## Tests

- `lint`: `Pages()` returns the title, aliases, headings, masked text, and
  resolved links of a fixture page; a page with no frontmatter still lists.
- `overlap`: the fixture hub from the spec; the report asserted in full and
  identical on a second run; `Member` narrows the pairs; a hub with no
  mirrors reports origins and an empty list with a reason.
- `mirror`: the moved page is skipped and links to it land on the hub page;
  a page moved to another project is mirrored as written; a `moved_to` that
  names no hub page leaves the link as it was.
- `txn`: `merge` writes under `wiki/`, refuses `wiki/projects/`.
- `mcpserver`: `overlap` over the transport; `plan` with `project` plans in
  the member and `apply` commits in the member's repository; the hub's
  mirror shows the pointer after sync.
- `cli`: `overlap` prints the pairs and `--json` parses.
- `go vet`, `make test`.

## Risks

- Thresholds are guesses until the usc-f26 run. The tests fix the
  arithmetic, not the numbers; step 6 may change the constants and the
  spec's open question records that.
- A wiki of several thousand pages makes the all-pairs loop quadratic.
  Sparse vectors keep it fast at one thousand; past that, a posting-list
  cut (compare only pages that share a term) is the next step and is left
  out until needed.
- `preferNear` resolves a bare name in a mirrored page to the same mirror.
  The redirect rewrites links at sync from the member's own resolver, before
  the page lands in the hub, so this does not interfere.

## Progress

2026-09-22. Every slice is in, each its own commit on main:

1. `lint.Vault.Pages`, `NameKey`, `Near`, `NearKeys`.
2. `internal/overlap`: the index, the three scores, the shared names and
   tags, the settled marks, the JSON and markdown reports, the tests.
3. The mirror's redirect for a page that moved into the hub
   (`movedInto`); the `merge` kind.
4. The `overlap` tool and command; `project` on `plan`; the server test
   that walks a merge from the report to the pointer to the sync.
5. The `wiki-merge` skill; the operations, frontmatter, and router
   references; the members design's "Merge" section; usage and README;
   CLAUDE.md; 5.5.0 in every manifest.
6. The run over usc-f26 (131 pages, 4 origins, under half a second) put
   the pair the stub names first once names scored their shared words
   and the template headings stopped counting. A synthetic hub of 1032
   pages (twelve copies of itl-ecosystem, the worst case, since every
   pair shares every term) scores 488k pairs in about three seconds; the
   posting-list cut stays in "Left for later".

What the run taught, kept as constants in `overlap`: `ScoreFloor` 0.1,
`NameOverlap` 0.5, `NameDuplicate` 0.8, `ContentDuplicate` 0.6.
