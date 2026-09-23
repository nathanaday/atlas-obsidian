---
name: atlas-merge
description: "Merge the wikis of a hub's members into the hub's own: find the pages two members wrote about one thing and upgrade them into the hub, and build bridge pages that join neighbouring pages across members, as one merge the user approves. Use for merge the members, consolidate the wikis, merge the knowledge bases, what overlaps across projects, find duplicates across projects, bridge the vaults, upgrade pages to the parent, adopt a new member's wiki. Duplicates inside one wiki are wiki-review's."
---

# Merge the members' wikis

A hub mirrors its members' wikis under `wiki/projects/<name>/`. Each member
wrote its pages alone, so two of them hold pages about one thing without
knowing it, and the overlap is visible only here. This skill finds it cheaply,
reads only the pages that matter, and proposes one merge the user approves
before anything is written.

Tools: `status`, `project`, `overlap`, `route`, `plan`, `apply`. Reads
[operations.md](../wiki/references/operations.md) and
[provenance.md](../wiki/references/provenance.md).

The merge has two moves:

- **Upgrade.** Two or more origins hold the same thing. One page in this wiki
  takes their content, written from the member versions and citing the
  members' source pages through the mirror. Each member page becomes a
  pointer: the same file, its frontmatter gaining `moved_to` and
  `moved_to_project`, its body one line saying where the page went. The
  member's own links to it still resolve, and the next sync sends every link
  to the hub's page and mirrors the pointer no more.
- **Bridge.** Two origins hold neighbouring things: Linux in one, macOS in
  another, or two pages that link the same name nobody wrote. One new page in
  this wiki names what they share and links both, with full vault paths into
  the mirrors. Nothing in a member changes.

Everything acts on this session's project; a member changes only through a
`plan` that names it with `project`, as its own operation, with the user's
yes.

## Find the overlap

1. Call `status`. Stop when the project lists no members: there is nothing to
   merge, and `atlas-project` adds members. When a member is `not mirrored
   yet`, or the last sync is older than the members' work, run `project` with
   `action: sync` first, so the report reads current pages.
2. Call `overlap`. Pass `member` with one origin's folder name when the user
   is adopting a new member into a hub that merged before, or asks about one
   member; the report then holds only the pairs that involve it. The report
   is bounded and deterministic:
   - `pairs`: two pages from two origins, `duplicate` (a name matches, or
     the content is nearly the same) or `related` (shared vocabulary or
     shared link names, different names), each with its scores, the shared
     terms and links, and whether a page here already settles it
     (`upgraded_to`: a page here by that name; `bridged_by`: pages here that
     link both).
   - `names`: link targets pages of two or more origins name, resolving to
     no page here, or to a different page in each origin. A bridge that has
     a name already.
   - `tags`: tags two or more origins share.
3. Read the report, not the wiki. Skip every settled pair unless the user asks
   to revisit one. Then read only the pages the candidates name, in full,
   with Read; a candidate is at most two pages plus what they cite. Read the
   index of this wiki and `wiki/hot.md`. Stop reading when the candidates you
   will propose are decided; the pairs below the ones you take are the next
   run's.

The scores are hints. A `duplicate` with a matching name and different
content may be two things that share a word; a `related` pair with a high
content score may be one thing under two names. Decide from the pages.

## Decide each candidate

For each candidate, one of three outcomes, with a reason in one line:

- **Upgrade** when the pages describe one thing and a reader of this wiki
  needs one page for it. Name the page as the members do when they agree;
  choose the fuller name when they do not, and carry the other as an alias.
  Call `route` with the type and title: a `match` here means the hub has the
  page already, so the merge extends it instead of creating it.
- **Bridge** when the pages are two things a reader would want to reach from
  one place. The bridge is a concept page (a note in lyt mode): what the
  things share, how they differ, and a link to each. Prefer a shared name
  from the report when one fits; it is what the members already call it.
- **Leave** when the resemblance is words, or when each member needs its own
  page as it is. Say so; a run that leaves every candidate is a good run.
  Always leave a member's own describing page (`entity_type: project`): it
  is the home of that wiki, and every page there links back to it. Bridge
  to it instead. A source page never upgrades either; the hub page cites it.

Bound the run: propose at most eight upgrades and eight bridges. More waits
for the next run, and the user can ask for it.

## Propose one merge

Show the user everything before any write, in one message they can answer in
a line. In this form:

> **Merge into usc-f26.**
>
> Upgrade 2 pages into this wiki:
> - `wiki/entities/CS513 Course Project.md` ← cs513-course `entities/CS513
>   Course Project.md` and cs513-project `entities/cs513-project.md`. Both
>   describe the group project; the course page has the rubric and the
>   project page has the idea. Each member page becomes a pointer.
> - …
>
> Create 1 bridge page:
> - `wiki/concepts/Reactive components.md`, joining cs513-course `concepts/
>   Synchronous Reactive Component.md` and cs513-course `concepts/
>   Asynchronous Reactive Component.md` with cs513-project `entities/Risk-Aware
>   Planned Driver.md`, which is one.
>
> Leave 3 candidates as they are: Linear Dynamical System and Risk Exposure
> Model share only the word model; …
>
> This writes one operation in each of cs513-course and cs513-project (the
> pointers) and one here (the pages and the index), then syncs. Yes, no, or
> name the items.

Every page named is a path the user can open. On a partial yes, do the items
named. On a no, stop, and say what the next run would look at.

## Write it

Read [operations.md](../wiki/references/operations.md). The kind is `merge`
everywhere. Complete file content for every write.

1. **The members first.** For each member with an upgraded page, one `plan`
   with `project` set to the member, kind `merge`, and one `replace` per
   moved page: the pointer. Keep the page's frontmatter as it was, add
   `moved_to` (the page's path here, under `wiki/`) and `moved_to_project`
   (this project's id, from `status`), set `status: deprecated`, and make the
   body one line: `Moved to the <hub> project as wiki/…; merged from … on
   <date>.` Show the preview, then `apply`. One member, one commit, in the
   member's own repository. Never touch a member's index or its other pages;
   the pointer keeps their links whole.
2. **Then this wiki.** One `plan` without `project`, kind `merge`: the
   upgraded pages, the bridge pages, `wiki/index.md` (or the MOC in lyt
   mode) with every new page listed, and `wiki/hot.md` refreshed. An upgraded
   page says in its body which members it merged from and cites their
   source pages with full vault paths into the mirrors
   (`[[wiki/projects/<name>/sources/…|…]]`). A bridge page links each page
   it joins the same way. Show the preview, then `apply`.
3. **Sync.** `project` with `action: sync`. The mirrors drop the moved pages
   and every member link to them now lands on the pages here. Report the
   operation ids, one per project, and the sync's counts.

The order matters: the pointers exist before the hub's pages are committed,
so a pointer never names a page that is not there when the sync runs; and the
sync runs last, so the mirror never holds a moved page and a hub page with
one name at once.

Never Write or Edit under `wiki/` here or in a member, and never target
`wiki/projects/`: a mirrored page changes in its own project, and the pointer
is that change.

## Report

Say what moved, what was built, what was left and why, and where each
operation landed. Then say what the next run would look at, from the pairs
you did not take, and that `overlap` with `member` narrows a run to one
origin.

`undo` in a project takes back that project's operation; a merge undone here
leaves the pointers in the members until they are undone there.

## Hand off

`wiki-review` for this wiki's own health after a large merge; `atlas-project`
to add or remove members.
