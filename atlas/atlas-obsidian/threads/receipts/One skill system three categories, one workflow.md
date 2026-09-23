---
type: receipt
thread: thr-20260922-d2b2
title: "One skill system: three categories, one workflow"
outcome: completed
created: 2026-09-22
---

> [!receipt] One skill system: three categories, one workflow · completed
> [Stub](<../stubs/One skill system three categories, one workflow.md>) → [Spec](<../specs/One skill system three categories, one workflow.md>) → [Plan](<../plans/One skill system three categories, one workflow.md>) → **Receipt**
> `thr-20260922-d2b2` · [Thread](<../archive/One skill system three categories, one workflow.md>) · filed 2026-09-22

## Delivered

Twenty-one skills in three categories, each a verb on atlas, wiki, or
thread, with a home skill per noun (5.6.0). `docs/skills.md` is the design:
the concept, the map, the workflow, who owns each verb and each operation
kind, the form of a skill, the references, the agents, and how the skills
are tested.

- New: `atlas-onboard`, `wiki-edit`, `wiki-review` (quick and deep),
  `thread-work`; the large-source pipeline in `wiki-ingest`; the review step
  in `thread-receipt`; agents `wiki-review` and `thread-review`.
- Renamed: `wiki-save`, `wiki-describe`, `wiki-canvas`, `wiki-base`,
  `atlas-merge`, `thread-work`.
- Folded: `wiki-mode` (into `atlas-project` and `modes.md`), `wiki-lint`
  (into `wiki-review`), `obsidian-markdown` (into `references/syntax.md`),
  `think` (its evidence rules into `thread-spec` and `thread-plan`), and the
  two Obsidian references into `obsidian.md`.
- Binary: `overlap` inside one wiki (`within`, with linked pairs settled and
  open pairs first), lint's `uncited` pages, a capture's `pages`, `lines`,
  and `outline`, and the snapshot's TODO and FIXME markers.
- Code and docs follow: the hook lists the skills by category, a thread
  launches with `thread-work`, and the hints, usage, README, and CLAUDE.md
  name the new skills.

## Verified

- `internal/plugin` tests the files to the design; `make test` and
  `go vet` pass.
- PDF page counts match Spotlight on five real PDFs of 1 to 351 pages.
- End to end in a scratch atlas home, recorded in `docs/skills.md`:
  onboarding a blank folder and a repository, a 2686-line ingest through
  four workers, quick and deep reviews over planted defects, an edit that
  repaired three findings, `thread-work` from a stub through both gates to a
  reviewed receipt, and 18 of 18 plain requests routed to the intended skill.
- What the runs taught went into the skills: a structure is seeded with its
  links, thread files wait for the work's commit, and linked pairs inside one
  wiki are settled.

## Left

- A staged file is committed as a hand edit (`thr-20260922-9ee5`).
- The installed plugin updates only after `main` is pushed.
