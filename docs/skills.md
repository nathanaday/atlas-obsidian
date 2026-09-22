# The skill system

Status: designed and built 2026-09-22, with 5.6.0. This document is the
source of truth for the skills: the concept, the map, the workflow, who owns
what, and the shape every skill follows. `internal/plugin` holds the plugin's
files to it.

## One concept

A project knows things and does things.

- The **wiki** is what a project knows: sources, entities, concepts, cited and
  linked, changed only by reviewed operations.
- The **threads** are what a project does: one line of work each, moving stub,
  spec, plan, receipt, one document per stage.
- The **atlas** is every project on the machine and the links between them.

Every skill is a verb on one of those three nouns, named `<noun>-<verb>`. Each
noun has one **home** skill, named for the noun, that orients in that half,
holds its contract, and routes inside it. The `atlas` home is the front door
and holds the one full map. A user who knows the three nouns can guess every
skill's name.

## The map

| Category | Home | Verbs |
|---|---|---|
| atlas | `atlas`: every project, what is wrong, the settings; the full map | `atlas-onboard`, `atlas-project`, `atlas-merge` |
| wiki | `wiki`: the project and its wiki, the operation contract | `wiki-ingest`, `wiki-query`, `wiki-save`, `wiki-edit`, `wiki-describe`, `wiki-review`, `wiki-fold`, `wiki-canvas`, `wiki-base` |
| thread | `thread`: the board, the quick moves, the phases | `thread-work`, `thread-stub`, `thread-spec`, `thread-plan`, `thread-run`, `thread-receipt` |

| Skill | Owns | Writes through |
|---|---|---|
| `atlas` | seeing every project; the settings | `settings`, `atlas` refresh |
| `atlas-onboard` | a folder becoming a project: init, the first description, the structure, the work in flight | `project` init, then `wiki-describe`, `thread` |
| `atlas-project` | changing a project: name, description, mode, threads on or off, members, sync, forget | `project` |
| `atlas-merge` | what a hub's members hold in common: upgrades and bridges | `plan` kind `merge`, here and in members |
| `wiki` | orientation in a project; the operation contract | nothing |
| `wiki-ingest` | turning a source into pages, of any size | `capture`, `plan` kind `ingest` |
| `wiki-query` | answering from the wiki | nothing |
| `wiki-save` | keeping something from the conversation | `plan` kind `save` |
| `wiki-edit` | changing pages that exist: rewrite, rename, move, split, combine, repair; seeding wanted pages | `plan` kinds `markdown` and `repair`, `stub` |
| `wiki-describe` | the page that describes the work | `stage` snapshot, `capture`, `plan` kind `ingest` |
| `wiki-review` | the wiki's health: quick and deterministic, or deep with reviewers | nothing |
| `wiki-fold` | a rollup of the log | `plan` kind `fold` |
| `wiki-canvas` | canvas boards | `plan` kind `canvas` |
| `wiki-base` | Bases views | `plan` kind `base` |
| `thread` | the board, priority, phase, block, title; phases; a review of the board | `thread`, `phase` |
| `thread-work` | moving a thread, or a sentence, to its next stage and on | the stage skills |
| `thread-stub` | opening a thread | `thread` |
| `thread-spec` | what done means | `thread` stage spec |
| `thread-plan` | how the work will go | `thread` stage plan |
| `thread-run` | doing the plan, with commits and progress | the work's own tools; Edit on the plan |
| `thread-receipt` | verifying and closing | `thread` stage receipt |

No two skills own the same verb, and no operation kind has two owners outside
the one exception: `wiki-describe` writes kind `ingest`, because the snapshot
it cites is a captured source like any other.

## The workflow

```text
a folder ──atlas-onboard──▶ a project
                              │
      ┌───────────────────────┴────────────────────────┐
      ▼ what it knows                                  ▼ what it does
  wiki-ingest  (sources in)                        thread-stub
  wiki-save    (the conversation in)                   │ thread-work conducts
  wiki-describe (the work itself)                  thread-spec ─ gate: the user agrees
  wiki-query   (answers out)                       thread-plan ─ gate: the user agrees
  wiki-review ─▶ wiki-edit (fix)                   thread-run
  wiki-fold, wiki-canvas, wiki-base                thread-receipt ─▶ wiki-save, wiki-describe
      │
      └──atlas-project (members) ──▶ atlas-merge   (several projects, one hub)
```

- A **blank folder** or a long-lived repository enters through
  `atlas-onboard`: it makes the project, describes the work, proposes the
  wiki's structure from the profiles in `modes.md`, and turns the work in
  flight (TODO and FIXME markers, roadmap documents, open issues) into
  thread stubs the user picks.
- **Knowledge enters** as sources (`wiki-ingest`), as conversation
  (`wiki-save`), or from the work itself (`wiki-describe`). It leaves as
  answers (`wiki-query`). `wiki-review` finds what is wrong; `wiki-edit`
  fixes it.
- **Work** moves through the thread stages. `thread-work` is the conductor:
  it reads a thread's stage and runs the next stage skill, and stops at the
  two gates, the spec and the plan, for the user's yes. A closed thread
  offers the wiki what it taught, through `wiki-save` or `wiki-describe`.
- **Several projects** become one hub through `atlas-project` (members) and
  `atlas-merge` (what they hold in common).

## The shape of a skill

Every SKILL.md follows one shape, so a reader finds the same thing in the
same place:

1. Frontmatter: `name` (the folder), and `description`: what it does in one
   sentence, then `Use for` and the words that trigger it. Where a nearby
   skill owns a nearby verb, the description says which.
2. A heading and one paragraph: what the skill does and why.
3. A line `Tools:` with the MCP tools it calls, and the references it reads.
4. The procedure, as numbered steps where order matters.
5. Where writes happen, the preview the user sees and the yes it waits for.
6. A last section, `Hand off`: the skills that come next.

A skill is advice; a rule that must hold is a tool's refusal or a hook
(`CLAUDE.md`, "Tool, skill, or hook"). A skill names its tools by their short
names; the host adds the prefix.

## References

References live with the home of their half and load only when a skill
names them.

| Reference | Holds | Read by |
|---|---|---|
| `skills/wiki/references/operations.md` | the plan and apply contract; each kind and its owner | every skill that writes the wiki |
| `skills/wiki/references/provenance.md` | the ledger, source rules, claim rules | `wiki-ingest`, `wiki-save`, `wiki-describe`, `wiki-query`, `wiki-review` |
| `skills/wiki/references/frontmatter.md` | page properties | every skill that writes a page |
| `skills/wiki/references/syntax.md` | Obsidian Flavored Markdown | every skill that writes a page |
| `skills/wiki/references/modes.md` | generic and lyt, and the domain profiles | `atlas-onboard`, `atlas-project`, `wiki-edit` |
| `skills/wiki/references/obsidian.md` | the snippet, graph groups, plugins, the Web Clipper | `wiki` |
| `skills/wiki-ingest/references/large-sources.md` | the map-reduce pipeline | `wiki-ingest` |
| `skills/wiki-fold/references/fold-template.md` | the fold page | `wiki-fold` |
| `skills/wiki-canvas/references/canvas-spec.md` | JSON Canvas | `wiki-canvas` |
| `skills/thread/references/threads.md` | the stages, the pages, the tools, phases, freshness | every thread skill |

## Agents

Agents are read-only workers a skill sends when a task splits. They read and
report; only the skill that sent them plans, applies, or files.

| Agent | Sent by | Does |
|---|---|---|
| `wiki-ingest` | `wiki-ingest` | reads one source and drafts pages, or reads one range of a large source and extracts its claims |
| `wiki-review` | `wiki-review` (deep) | reads one section of the wiki and reports gaps, errors, contradictions, and stale claims, with evidence |
| `thread-review` | `thread-receipt` | reads a thread's spec and plan and the diff of its work, and reports what the work misses or breaks |

## Efficiency

Code finds, the model decides. Where a question has an answer two correct
runs agree on, a tool answers it, and the skill reads the tool's report
before any page:

- `lint`: structure, wanted pages, stubs, and pages that cite no source.
- `overlap`: pages that look alike, across a hub's members or, with
  `within`, inside one wiki.
- `capture`: a source's pages, lines, and outline, which decide whether it is
  read whole or split across workers.
- `stage` with `snapshot`: the work's instructions, files, docs, and its TODO
  and FIXME markers.

A skill then reads only the pages the report names.

## What changed in 5.6.0

| Before | After | Why |
|---|---|---|
| `save`, `describe`, `canvas`, `obsidian-bases` | `wiki-save`, `wiki-describe`, `wiki-canvas`, `wiki-base` | the category in the name |
| `work` | `thread-work` | the conductor of the stages; the launch prompts use it |
| `wiki-merge` | `atlas-merge` | it acts across projects |
| `wiki-lint` | `wiki-review` | quick review is lint and more; deep review is new |
| `wiki-mode` | `atlas-project` and `modes.md` | the mode is a project field |
| `obsidian-markdown` | `references/syntax.md` | syntax is a reference every writer reads, not a verb |
| `think` | the evidence rules in `thread-spec`, `thread-plan`, and the deep review | a reasoning method is not a verb on a project |
| — | `atlas-onboard`, `wiki-edit`, `wiki-review` deep, `wiki-ingest` large sources, `thread-receipt` review | the gaps in the workflow |
