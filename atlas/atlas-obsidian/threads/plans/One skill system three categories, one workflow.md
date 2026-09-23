---
type: plan
thread: thr-20260922-d2b2
title: "One skill system: three categories, one workflow"
created: 2026-09-22
---

> [!plan] One skill system: three categories, one workflow
> [Stub](<../stubs/One skill system three categories, one workflow.md>) → [Spec](<../specs/One skill system three categories, one workflow.md>) → **Plan** → [Receipt](<../receipts/One skill system three categories, one workflow.md>)
> `thr-20260922-d2b2` · [Thread](<../archive/One skill system three categories, one workflow.md>) · filed 2026-09-22

## Approach

Binary first, so every skill written after it can name real fields; then the
plugin-files test, written before the renames so it drives them; then the
skills category by category, each rewritten against `docs/skills.md`; then
the code's hints and the docs; then end-to-end runs in a scratch atlas home.

## Slices

1. **Binary.** `overlap.Options.Within` and the tool's `within`; lint's
   `uncited` list (content pages with no `sources` property and no link into
   a sources folder), a signal, not a finding; `capture.Captured` gains
   `pages` (PDF, standard-library parse including flate object streams),
   `lines`, and `outline` (markdown headings with line numbers, bounded);
   the describe snapshot gains a `Markers` section from `git grep` of TODO,
   FIXME, XXX, and HACK, bounded. Tests for each.
2. **The plugin-files test** in `internal/plugin`: folders, names, links,
   named skills, the hook list, the launch prompts.
3. **Moves.** `git mv` the renamed folders, delete the folded ones, move
   `threads.md` under `skills/thread/references/`, add `syntax.md` (from
   obsidian-markdown) to the wiki references, rename the lint agent.
4. **docs/skills.md**, then each skill rewritten to it: the three homes, the
   atlas skills, the wiki skills, the thread skills, the three agents.
5. **Code and docs.** `hooks.Skills` by category, hook hints, launch prompts
   (`ThreadPrompt` sends `thread-work`), CLI and tool messages, usage,
   README, CLAUDE.md, the design docs that name skills. 5.6.0.
6. **End to end** in a scratch home, per the spec's list; fix what the runs
   show; record them here.

## Risks

- `claude -p` runs cost time; each is scoped to one skill and one scratch
  project.
- PDF page counts: compressed cross-reference streams hide `/Type /Page`;
  the parse inflates object streams, and reports 0 (unknown) rather than a
  guess.
