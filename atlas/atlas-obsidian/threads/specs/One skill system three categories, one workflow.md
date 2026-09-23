---
type: spec
thread: thr-20260922-d2b2
title: "One skill system: three categories, one workflow"
created: 2026-09-22
---

> [!spec] One skill system: three categories, one workflow
> [Stub](<../stubs/One skill system three categories, one workflow.md>) → **Spec** → [Plan](<../plans/One skill system three categories, one workflow.md>) → [Receipt](<../receipts/One skill system three categories, one workflow.md>)
> `thr-20260922-d2b2` · [Thread](<../archive/One skill system three categories, one workflow.md>) · filed 2026-09-22

## The problem

The plugin has 22 skills that grew in three waves: the wiki skills from
claude-obsidian, the thread skills written here, and the atlas skills. They
work, but they do not read as one system:

- Four naming schemes: `wiki-x`, `thread-x`, `atlas-x`, and bare verbs
  (`save`, `describe`, `work`, `think`, `canvas`, `obsidian-markdown`,
  `obsidian-bases`). A user cannot guess a skill's name or find its category.
- Routing lives in three places (`wiki`, `thread`, `atlas`) with overlapping
  tables.
- Overlap: `wiki-mode` repeats the `project` edit; `wiki-lint` and the lint
  agent repeat each other; `obsidian-markdown`'s edit path duplicates `save`;
  `think` is a generic reasoning loop outside the atlas concept.
- Gaps: no skill edits, renames, or repairs an existing page (the `repair`
  and `markdown` kinds have no owner); no content review beyond lint's
  structure checks; no path for a source past fifty pages; no onboarding that
  proposes a structure or gathers the work in flight; no review of finished
  work against its spec before a thread closes; no single way to move a
  thread to its next stage.

## The design concept

A project knows things and does things. The **wiki** is what it knows; the
**threads** are what it does; the **atlas** is every project and the links
between them. Every skill is a verb on one of those three nouns, named
`<noun>-<verb>`, and each noun has one home skill named for the noun that
orients, holds the contract for its half, and routes inside it. The atlas
home is the front door and holds the one full map.

## Done looks like

Twenty-one skills in three categories:

| Category | Home | Skills |
|---|---|---|
| atlas | `atlas` | `atlas-onboard`, `atlas-project`, `atlas-merge` |
| wiki | `wiki` | `wiki-ingest`, `wiki-query`, `wiki-save`, `wiki-edit`, `wiki-describe`, `wiki-review`, `wiki-fold`, `wiki-canvas`, `wiki-base` |
| thread | `thread` | `thread-stub`, `thread-spec`, `thread-plan`, `thread-run`, `thread-receipt`, `thread-work` |

- Renamed: `save` → `wiki-save`, `describe` → `wiki-describe`, `canvas` →
  `wiki-canvas`, `obsidian-bases` → `wiki-base`, `wiki-merge` →
  `atlas-merge`, `work` → `thread-work`.
- Folded: `wiki-mode` into `atlas-project` (the mode is a project field) and
  `references/modes.md`; `wiki-lint` into `wiki-review`; `obsidian-markdown`
  into a syntax reference every writing skill reads; `think`'s evidence frame
  and verification discipline into `thread-spec`, `thread-plan`, and the
  deep review. `think` itself goes.
- New:
  - `atlas-onboard`: a folder, blank or long-lived, becomes a project in one
    guided flow: init (and git when there is none), describe the work,
    propose a wiki structure the user approves, and gather the work in
    flight (TODO and FIXME markers, roadmap documents, open issues) into
    thread stubs the user picks.
  - `wiki-edit`: change pages that exist on request: rewrite, rename or
    move with every link updated, split, combine two pages of one wiki, and
    repair what review found. Owns the `markdown` and `repair` kinds.
  - `wiki-review`: quick review is deterministic: lint, plus overlap inside
    one wiki, plus pages that cite no source; it reads no page unless the
    user asks. Deep review sends read-only reviewers over sections of the
    wiki for gaps, mistakes, contradictions, and stale claims, and returns
    findings with evidence; fixes go to `wiki-edit`.
  - `thread-work`: take a thread, or a sentence, to its next stage, and on
    through the stages while the user agrees, with two gates (the spec and
    the plan) where the user says yes. Replaces `work`.
- `wiki-ingest` handles any size: in full up to about fifty pages; past
  that, a map-reduce pipeline: the binary reports a capture's page count and
  outline, workers extract claims from page ranges in parallel, the
  orchestrator reduces them by entity and concept and writes the pages as
  one operation.
- `thread-receipt` verifies before `completed`: the tests, and for a thread
  that changed code, a fresh reviewer that reads the diff against the spec.
- Three agents, each read-only: `wiki-ingest` (a source, or a range of one),
  `wiki-review` (a section of the wiki), `thread-review` (a diff against a
  spec).
- One document, `docs/skills.md`: the concept, the map, the workflow from a
  blank folder to a closed thread, what each skill owns, the shared
  references.
- References live with their home: `skills/wiki/references/` for the wiki,
  `skills/thread/references/threads.md` for threads.

## The experience

A user who knows the three nouns can guess every skill. Each skill says in
its first lines what it does, which tools it uses, and which skill comes
before and after it. No two skills own the same verb. Every write still
goes through a preview the user approves. The session hook lists the skills
by category.

## Out of scope

- New MCP tools beyond fields on existing ones. The binary gains: `overlap`
  inside one origin, lint's list of pages that cite no source, a capture's
  page count and outline, TODO and FIXME markers in the describe snapshot.
- Changing the operation contract, the thread stages, or the members design.

## Constraints

- No new Go dependency; PDF page counting uses the standard library.
- Every skill works in Claude Code and Codex.
- The launch prompts, the hook text, the CLI hints, and every doc follow the
  renames in the same change.

## Decisions

- **Rename rather than alias.** An alias keeps both names alive in every
  list. The plugin is at 5.x and its user is its author; a clean break now
  costs one release note.
- **`atlas-merge`, not `wiki-merge`.** It acts across projects.
- **`thread-work` keeps `thread-run`.** Run is the execution stage; work is
  the conductor that picks the next stage. The conductor holds no stage
  procedure of its own; it reads the stage skill.
- **One ingest skill.** A large source is a path inside ingest, not a second
  skill with the same verb.
- **`think` goes.** It is a general reasoning method, not a verb on a
  project; the parts that protect decisions move to where decisions are
  made.

## Verification

- A Go test over the plugin files: every skill folder holds a SKILL.md whose
  `name` is the folder, every relative link resolves, every skill a skill,
  agent, hook, launch prompt, or doc names exists, and the hook's list equals
  the folders.
- Unit tests for each binary addition.
- End-to-end runs with `claude -p --plugin-dir` over scratch projects in a
  scratch atlas home (never the user's projects): onboard a blank folder and
  an existing repository, ingest a source past fifty pages, a quick and a
  deep review, an edit with a rename, thread-work from a sentence to a plan.
