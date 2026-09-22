# Modes and profiles

The **mode** decides where a new page goes. It is a field of `project.json`:
`atlas-project` changes it, and `route` applies it to every new page. A
**profile** is a set of folders, page types, and index sections for one kind
of work, laid over the mode. `atlas-onboard` proposes one; `wiki-edit`
applies a later change of structure.

## The modes

| Mode | New pages | Navigation |
|---|---|---|
| `generic` (default) | `wiki/sources/`, `wiki/entities/`, `wiki/concepts/` by type | `wiki/index.md` |
| `lyt` | `wiki/notes/`, one idea per note, whatever the type | `wiki/mocs/*.md`; `wiki/index.md` is the home map |

A change of mode routes future pages only. It moves no page and rewrites no
link. Moving the pages that exist is a separate `wiki-edit` operation with a
complete move map.

In `lyt` mode:

- a note holds one idea and fits on a screen; split what grows past that;
- a note's `mocs:` property names the maps that reach it;
- a map of content (MOC) is a place to navigate from, not a container; notes
  stay flat in `wiki/notes/`;
- every new note joins at least one MOC in the same operation.

## Profiles

Every profile keeps the invariants: sources captured under `.raw/captured/`,
pages under `wiki/`, `index.md`, `log.md`, and `hot.md` as the lifecycle
pages, the ledger under `wiki/meta/ledgers/`, one operation per change.

### Website or content system

Suggested routes: `wiki/pages/`, `wiki/structure/`, `wiki/audits/`,
`wiki/keywords/`, and `wiki/entities/`.

Useful page properties include the source URL, lifecycle status, canonical URL,
last verified date, and internal-link counts. Treat crawl and analytics exports
as sources; do not assert live indexing or HTTP status without current evidence.

### Software repository

Suggested routes: `wiki/modules/`, `wiki/components/`, `wiki/decisions/`,
`wiki/dependencies/`, and `wiki/flows/`.

Record repository-relative paths, purpose, status, dependency relationships,
and evidence from code, tests, issues, or primary documentation. Generated
architecture pages must distinguish observed behavior from inference.

### Business or project

Suggested routes: `wiki/stakeholders/`, `wiki/decisions/`,
`wiki/deliverables/`, `wiki/intel/`, and `wiki/meetings/`.

Record decision date, owner, status, rationale, and source record. Preserve
conflicting recollections as contested evidence instead of rewriting history.

### Personal knowledge vault

Suggested routes: `wiki/goals/`, `wiki/learning/`, `wiki/people/`,
`wiki/areas/`, and `wiki/resources/`.

Default to local-only processing. Confirm privacy and retention before capturing
messages, health, finance, relationship, or voice data. Do not save a whole
conversation when the user asked to preserve only one insight.

### Research

Suggested routes: `wiki/papers/`, `wiki/concepts/`, `wiki/entities/`,
`wiki/syntheses/`, and `wiki/gaps/`.

Track claim support, contradictions, authority, independence, freshness, and
risk in the ledgers. A paper summary is secondary to the captured paper and
does not automatically validate its claims.

### Book or course companion

Suggested routes: `wiki/chapters/`, `wiki/concepts/`, `wiki/people/`,
`wiki/exercises/`, and `wiki/reflections/`.

Capture only material the user is authorized to store. Prefer concise
source-linked notes over reproducing copyrighted chapters or transcripts.

## Applying a profile

1. Read the mode, the index, and a few representative pages.
2. Draft the map: the folders, the page types, the properties, and the index
   sections, and name any conflict with a page that exists.
3. Show the map. On a yes, seed only the pages the work already needs, as
   stubs (`wiki-edit`), and add the index sections in the same operation.
4. Let use decide the rest. An empty folder tree is not a structure.
