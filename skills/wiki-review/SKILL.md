---
name: wiki-review
description: "Check the wiki's health and report what is wrong, without changing it. Quick review is deterministic and cheap: lint's structure findings, near-duplicate pages, pages that cite no source, stubs, and wanted pages, from the tools alone. Deep review sends read-only reviewers over sections of the wiki for gaps, errors, contradictions, and stale claims, and costs more. Use for review the wiki, wiki health, lint, audit the wiki, find orphans, find dead links, find duplicates, what is wrong with the wiki, deep review, check the wiki for mistakes, find gaps, find contradictions. Fixing what it finds is wiki-edit."
---

# Review the wiki

Say what is wrong with the wiki, with evidence, and change nothing. Two
depths: the **quick** review reads tool reports and no pages, so it costs
little and can run often; the **deep** review reads pages, through workers,
and judges what code cannot: whether a claim is right, current, supported,
and consistent. Fixing is `wiki-edit`'s, for the findings the user picks.

Tools: `status`, `lint`, `overlap`. Reads
[provenance.md](../wiki/references/provenance.md) for the deep review.

Choose the depth from the request: "lint", "health", "quick" mean quick;
"deep", "mistakes", "gaps", "contradictions", "is it right" mean deep. When
unclear, run the quick review and offer the deep one.

## Quick review

1. Call `status`, then `lint`, then `overlap` with `within: true` (and, in a
   hub, `member` set to this project's own name, so the mirrors' own
   duplicates stay the members' business).
2. Report from the tools alone, grouped by what the user loses:
   - **Broken navigation**: dead links, stale index entries, ambiguous links
     and duplicate basenames, with path and line.
   - **Unreachable pages**: orphans, and pages no index or map lists.
   - **Duplicates**: `overlap`'s `duplicate` pairs inside this wiki, with
     their scores and shared terms; the open `related` pairs only when their
     content score is high. A settled pair is one page linking the other,
     such as a page and the source it cites: leave it out.
   - **Weak evidence**: lint's `uncited` pages, which cite no source.
   - **Unfinished**: stubs to fill, wanted pages, empty sections, missing
     properties, ledger errors.
3. Keep the tools' paths, lines, and counts. Say when a finding may be
   intended (an orphan kept on purpose, a duplicate basename in two
   folders); do not guess intent from a finding alone. Read a page only when
   the user asks about one finding.

## Deep review

The deep review reads the wiki's pages against their sources. Say its cost
first, in one line: the pages, the sections, the workers.

1. Run the quick review first; its findings go in the report and the
   workers skip them.
2. Divide the wiki into sections of at most 15 pages: a folder, a map of
   content, or a cluster of pages that link each other. Put each section's
   most-linked page first. Leave out stubs and the mirrors under
   `wiki/projects/`.
3. Send one `wiki-review` worker per section, in parallel, in waves of at
   most eight. Give each the project's folder, the section's pages, the
   project's description, and the quick review's findings for those pages.
   In a host without workers, review the sections yourself, one at a time.
4. Each worker returns findings of five kinds, each with the page, the
   claim, and the evidence: **error** (a claim its cited source does not
   say, or says otherwise), **contradiction** (two pages that disagree),
   **stale** (a claim a newer source or page overtakes), **unsupported** (a
   material claim with no citation), **gap** (a subject the pages lean on and
   no page covers).
5. Merge the findings: drop repeats across sections, join contradictions two
   workers each saw half of, and rank by what a reader would get wrong.

Keep evidence apart from inference: a finding says what was read, where, and
what follows from it. Never report a source as saying what you did not read
in it.

## Report

Lead with the count by kind and the three findings that matter most. Then
the findings as a numbered list: kind, page and line, the evidence, and the
fix you would propose, one line each. Say what the review did not cover.
Write no report into the wiki; the user may keep one with `wiki-save`.

## Hand off

`wiki-edit` for the findings the user picks: it repairs, renames, combines,
and seeds wanted pages. `wiki-ingest` for a gap a source would fill.
`atlas-merge` for duplicates across a hub's members.
