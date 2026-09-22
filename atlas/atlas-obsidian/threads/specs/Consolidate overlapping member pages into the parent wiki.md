---
type: spec
thread: thr-20260922-5519
title: "Consolidate overlapping member pages into the parent wiki"
created: 2026-09-22
---

> [!spec] Consolidate overlapping member pages into the parent wiki
> [Stub](<../stubs/Consolidate overlapping member pages into the parent wiki.md>) → **Spec** → [Plan](<../plans/Consolidate overlapping member pages into the parent wiki.md>) → Receipt
> `thr-20260922-5519` · [Thread](<../Consolidate overlapping member pages into the parent wiki.md>) · filed 2026-09-22

## The problem

Two members of a hub write about the same things without knowing it. Each wiki
grew inside its own project, so cs513-course holds "CS513 Course Project" and
cs513-project holds "cs513-project", and neither links to the other. The overlap
is visible only from the hub, which mirrors both. Today nothing finds it: the
hub's own wiki is empty, and reading every mirrored page into the context
window to look for overlap costs more than it finds.

The user wants a merge that is cheap, accurate, and transparent: the code finds
the candidates, the model reads only those, and the user sees one statement of
every page that moves and every page that is new before anything is written.

## Done looks like

1. **A read-only `overlap` tool** on the atlas MCP server, and an `overlap`
   command, that reads the hub's wiki as it sits on disk (its own pages and the
   mirrors under `wiki/projects/`) and returns a bounded, ranked, deterministic
   report of candidates across origins:
   - **pairs** of pages from two different origins that look like the same
     thing (`duplicate`) or about neighbouring things (`related`), with the
     scores that put them there (name, content, links), the evidence (shared
     terms, shared link names), and whether the hub already settled the pair
     (a hub page with the same name, or a hub page that links to both);
   - **shared names**: link targets that pages in two or more origins name and
     that resolve to no page in the hub, or to a different page in each origin;
     these are bridge pages that already have a name;
   - **shared tags** across origins;
   - the origins with their page counts, and a `next` line for the model.
   The whole report is small enough to read in one call for wikis of a
   thousand pages. No model call is needed to produce it.
2. **A `wiki-merge` skill** that runs the procedure: confirm the mirrors are
   current, call `overlap`, triage the report, read only the candidate pages,
   and propose one merge plan to the user, in one message: the pages to
   upgrade into the hub, the bridge pages to create, and the links each new
   page makes. On a yes, it writes in the same pattern as ingest: plans the
   user has seen, then apply. On a no or a partial yes, it does what was
   approved.
3. **An upgrade moves a page's content into the hub and leaves a pointer
   behind.** The hub gains the page, written by the model from the member
   versions, filed by the hub's mode, listed in the hub's index, and citing
   the members' source pages through the mirror. Each member page it replaces
   becomes a pointer: the same file, its frontmatter gaining `moved_to` (the
   path in the hub) and `moved_to_project` (the hub's id), its body one line
   saying where the page went. The member's links to it still resolve.
4. **Sync honours the pointer.** When a member page's `moved_to_project` is
   this hub, sync does not mirror it and rewrites every member link to it to
   the hub's page. In the hub the graph has one page, and every member's link
   lands on it. In any other hub the pointer is mirrored as written.
5. **A bridge is a new hub page** that links to the member pages it joins,
   with full vault paths into the mirrors, and joins the hub's index. Nothing
   in a member changes.
6. **Writes into a member are the member's own operations.** The `plan` tool
   gains `project`, as the thread tools have it, so the skill plans the
   pointers in the member and the new pages in the hub as separate operations,
   each one commit in its own repository, each with kind `merge`. `apply`
   applies the plan where it was made.
7. **Re-running is cheap.** A settled pair is marked in the report, and the
   skill skips it unless asked. A member added later to a hub that merged
   before is handled by the same run: `overlap` takes `member` to look only at
   pairs that involve one origin.

## The experience

The user sees one statement, in the shape of the ingest preview, before any
write: "Upgrade 2 pages into usc-f26: CS513 Course Project (from cs513-course
and cs513-project) … Create 1 bridge page: Reactive components, linking …
Leave 3 candidates as they are: …". Every page named is a path they can open.
They answer yes, no, or by naming the items. Then the apply reports, the
sync runs, and the report ends with the operation ids.

The model reads pages by the handful, not the wiki. The report is the map;
the pages it names are the territory the model walks.

## Out of scope

- Merging within one project. `overlap` compares across origins only.
- Semantic bridges the text does not support. "Linux" and "macOS" become
  "Operating systems" only when both pages share vocabulary or a link name;
  the tool surfaces the pair, the model names the bridge.
- Moving captured sources or the ledger. A hub page cites a member's source
  page through the mirror; the bytes stay in the member.
- A reverse merge, from hub to member.
- Threads. Overlapping threads are a separate question.

## Constraints

- `wiki/projects/` stays derived: nothing but sync writes there, and the
  upgrade never targets a mirror path.
- Every write goes through `plan` and `apply`; the kind `merge` writes only
  under `wiki/` and never under `wiki/projects/`.
- The report is deterministic for one wiki: the same pages give the same
  report, ties broken by path.
- No new dependency. Tokenizing, TF-IDF, cosine, and edit distance are small
  and live in the binary.
- A new project reports no overlap; a hub with no members reports its origins
  and nothing else and says why.

## Decisions

- **The report is computed from the mirrors, not the members' working trees.**
  The mirror is what the hub sees, sync keeps it current, and the tool needs
  no config. The skill runs sync first when status says a member is not
  mirrored.
- **Pointer, not deletion.** Deleting a page in the member breaks every link
  to it and asks the model to rewrite each page that linked it. The pointer
  keeps the member's link graph whole with one write per moved page, and sync
  turns it into a redirect in the hub. The user asked for removal; the pointer
  removes the content and keeps the address.
- **Content similarity is TF-IDF cosine over the page's words, with the
  title, aliases, and headings weighted higher; name similarity is lint's
  normalized name key and edit distance; link similarity is Jaccard over the
  names each page links.** These are the standard cheap measures and they
  compose into one score. BM25 buys little at this size and would be a
  second implementation of the same idea.
- **One score, two kinds.** A pair is a `duplicate` when the names match or
  the content score is high, `related` otherwise. The kind is a hint for the
  skill's triage; the scores stay in the report so the model can disagree.
- **`plan` gains `project` rather than a merge tool that writes two
  repositories at once.** Two repositories are two commits whatever wraps
  them, and the user must see each. A `plan` in the member is the same
  reviewed operation as anywhere else.
- **The tool is `overlap`, the skill is `wiki-merge`.** The tool is the fact:
  the overlap between the origins. The skill is the verb. The kind on the
  operations is `merge` so history reads right.

Options turned down: a symlink or embed instead of a hub page (the mirror
design already rejected links across vaults); an embedding model (a network
call and a dependency, for a gain the report does not need yet); comparing
members' working trees live (needs the config in the tool, and the mirror is
the hub's truth).

## Verification

- `overlap` unit tests over a fixture hub with three origins: an exact
  duplicate by name, a duplicate by alias, a related pair by shared terms, a
  shared wanted name, a settled pair (hub page links both), and a moved page
  that is excluded. The report is asserted in full, twice, for determinism.
- Mirror tests: a member page with `moved_to_project` naming the hub is not
  mirrored and links to it are rewritten to the hub's page; naming another
  project, it is mirrored as written.
- Server tests: `overlap` over the in-memory transport; `plan` with `project`
  plans in the member and `apply` commits there; kind `merge` refuses a mirror
  path.
- CLI test for the `overlap` command.
- A run over usc-f26 and its four members, read only, to see that the report
  names the CS513 project pair the stub records and stays under a screen.

## Open questions, answered by the work

- The thresholds. The usc-f26 run settled them: a pair appears at a
  combined score of 0.1, and is a `duplicate` at name ≥ 0.8 or content
  ≥ 0.6. Two names that share half their words score their word overlap
  (0.5 to 0.99), which is what puts "CS513 Course Project" and
  "cs513-project" at the top of the report; the exact key and the edit
  distance alone missed them. The template headings (Overview,
  Relationships, Sources) are not emphasized, or every entity page of one
  wiki looks like every entity page of another.
- Whether the member's index should say a page moved. No: the pointer keeps
  the link alive, and the skill never touches a member's other pages.
