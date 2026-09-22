---
name: wiki-fold
description: "Roll up a range of the wiki's log into one fold page: extractive, bounded, and the same every time for the same entries; previewed first, applied as one operation on request. Use for fold the log, run a fold, log rollup, roll up log entries, what happened in the wiki lately, commit the fold. Never changes the entries or the pages they name."
---

# Fold the log

`wiki/log.md` gets one entry per operation, so it grows without end. A fold
is a page that rolls up a range of entries: what changed, which pages, and
the themes across them, each line traced to its entry. It adds a page and
changes nothing else. Never fold a fold, and never fold unasked.

Tools: `status`, `plan`, `apply`. Reads
[fold-template.md](references/fold-template.md) and
[operations.md](../wiki/references/operations.md).

## Select a bounded range

Use batch exponent `k` with size `2^k`; default to `k=4`. An explicit entry
range may override it. If fewer entries exist than requested, report the
shortfall and stop rather than folding a partial batch.

Read the selected log entries completely. Read referenced child pages only when
the log lacks enough context: target 0-10 reads, hard ceiling 15. A missing page
stays an explicit `page_missing` record.

Derive the structural id only from inputs:

```text
fold-k{K}-from-{EARLIEST-DATE}-to-{LATEST-DATE}-n{COUNT}
```

If `wiki/folds/{FOLD_ID}.md` already exists, return a no-op. Replacing it needs
an explicit request and a `replace` write the user reviews.

## Draft extractively

Follow [fold-template.md](references/fold-template.md). Every child log entry
has one deterministic `child_key` in frontmatter and exactly one matching row in
the Child Entries table. Do not deduplicate children by page; the final Child
Pages list may be deduplicated.

Every outcome names its source entry. Every number is verifiable in the selected
entry. A cross-entry theme names at least two contributing entries. Prefer
"ambiguous in source" or "source missing" to invention. When a child page and a
log entry disagree, keep both and name the mismatch; the log entry is the fold's
primary source.

Check before proposing: deterministic id and exact count; frontmatter and table
bijection; numeric traceability; a source for every outcome and theme; no change
to any child, source, or ledger.

A fold adds no new evidence, so it never upgrades an assessment or creates a
source record. Report discovered contradictions for later review.

## Preview by default

Return the complete draft, id, child range, read budget, and proposed paths
without touching the vault. Workers may check child entries and return
extracts; only you assemble the fold.

When the user says to apply, build one plan of kind `fold` following
[operations.md](../wiki/references/operations.md):

- `wiki/folds/{FOLD_ID}.md`, `create` by default;
- `wiki/index.md`, listing the fold under a Folds section.

Do not touch `wiki/hot.md`. The core writes the log entry from your summary.
Show the preview, apply on approval, and report the operation id.

## Hand off

`wiki-review` for contradictions the fold found between an entry and a page.
