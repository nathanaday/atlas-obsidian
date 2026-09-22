---
type: receipt
thread: thr-20260922-5519
title: "Consolidate overlapping member pages into the parent wiki"
outcome: completed
created: 2026-09-22
---

> [!receipt] Consolidate overlapping member pages into the parent wiki · completed
> [Stub](<../stubs/Consolidate overlapping member pages into the parent wiki.md>) → [Spec](<../specs/Consolidate overlapping member pages into the parent wiki.md>) → [Plan](<../plans/Consolidate overlapping member pages into the parent wiki.md>) → **Receipt**
> `thr-20260922-5519` · [Thread](<../archive/Consolidate overlapping member pages into the parent wiki.md>) · filed 2026-09-22

## Delivered

- `overlap`, a read-only tool and command that scores every pair of pages
  across a hub's origins (its own wiki and each mirror) by name, content, and
  the names they link, and reports duplicate and related pairs, shared link
  names, and shared tags, each with evidence and whether a hub page already
  settles it. Deterministic, bounded, under half a second over usc-f26.
- The `wiki-merge` skill: sync check, `overlap`, read only the candidates,
  one proposal the user answers in a line, then the pointers in each member
  as that member's own `merge` operation, the pages here as one, then sync.
- `plan` takes `project`; the `merge` kind; the pointer (`moved_to`,
  `moved_to_project`); the mirror's redirect, which drops a moved page and
  sends every member link to the hub's page.
- `lint.Vault.Pages`, `NameKey`, `Near`, `NearKeys`; the references, the
  router, the members design's "Merge" section, usage, README, CLAUDE.md;
  5.5.0 in every manifest.

## Verified

- Unit tests in lint, overlap, mirror, txn, mcpserver, and cli; `go vet` and
  `make test` clean.
- The report over usc-f26 puts the pair the stub names first, once names
  score their shared words and the template headings stop counting. A
  synthetic 1032-page hub scores 488k pairs in about three seconds.
- A read-only dry run of the skill in usc-f26 (`claude -p` with
  `--plugin-dir`) produced the proposal in the skill's shape: one bridge,
  no upgrade, and reasons for what it left. It declined to upgrade a
  member's describing page, which the skill now says outright.

## Left

- The installed plugin stays at 5.3.0 until `main` is pushed: the
  marketplace cache clones GitHub, not the checkout. Noted in CLAUDE.md.
- A posting-list cut for hubs past a few thousand pages, and overlapping
  threads across members, in the design's "Left for later".
