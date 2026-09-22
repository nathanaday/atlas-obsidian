---
name: wiki-edit
description: "Change wiki pages that exist, as one reviewed operation: rewrite or correct a page, rename or move it with every link that names it updated, split a page that grew too big, combine two pages about one thing, restructure folders after a mode or profile change, repair what wiki-review found, and seed the pages the wiki's links want. Use for edit this page, fix this page, rename the page, move the page, split this page, combine these pages, merge these two pages, reorganize the wiki, fix the dead links, fix the lint findings, repair, stub the wanted pages, fill the stubs. New knowledge from a source is wiki-ingest; from the conversation, wiki-save."
---

# Edit the wiki

Change what the wiki already holds, and keep every link whole while doing it.
An edit adds no evidence: a claim it writes is one a page or a source in the
vault already supports. The user sees every changed page before it lands.

Tools: `status`, `lint`, `route`, `stub`, `plan`, `apply`. Reads
[operations.md](../wiki/references/operations.md),
[syntax.md](../wiki/references/syntax.md), and
[frontmatter.md](../wiki/references/frontmatter.md).

## 1. Scope

1. Call `status`. Name the pages the edit touches, in the user's terms, and
   read each one in full.
2. Find every page that links them: Grep for the file name, the title, and
   each alias, in `[[...]]` and in markdown links, across `wiki/`. A mirrored
   page under `wiki/projects/` is changed in its own project, never here; say
   so when the user names one.
3. Pick the kind: `repair` for fixes to what review or lint found, `markdown`
   for every other edit. One operation per request.

## 2. The edits

- **Rewrite or correct.** Draft the complete new page. Keep the properties a
  page has, and set `updated` when the content changed. A correction names
  the source that supports it; with none, mark the claim unsupported instead
  of changing it ([provenance.md](../wiki/references/provenance.md)).
- **Rename or move.** Delete the old path and create the new one with the
  same content and the new `title`; put the old title in `aliases`, so an
  old link in a mirror or in the user's notes still resolves by alias. Then
  rewrite every link to it: keep each link's display text, heading, and
  block reference. Call `route` for the new path when the type changes.
  When the ledger lists the page for a source (`wiki/meta/ledgers/source-ledger.json`),
  the plan's `sources` entry for that source gives its `pages` with the new
  path.
- **Split.** One page per idea. The first keeps the path, so links still
  land; it links the new pages under a short summary. Move each claim with
  its citation. Links that meant the moved part now point to the new page.
- **Combine** two pages of this wiki about one thing. Keep the fuller path;
  merge the content without losing a claim or a citation; the other title
  becomes an alias; delete the other page and rewrite its links. Pages of two
  members of a hub are `atlas-merge`'s.
- **Restructure** after a mode or profile change: a complete move map (old
  path, new path) for every page, shown before any plan, then the moves as
  above. Past about 60 pages, split the map into operations by folder.
- **Repair** a finding from `wiki-review` or `lint`: a dead link gets its
  nearest page, or the text without the link, or a stub when the page is
  wanted; an orphan gets a link from the index or a page that should name
  it; missing properties are added; an empty section is filled from the
  page's sources or removed. Fix only what the user picked.
- **Seed wanted pages.** `stub` with no titles seeds every page the wiki's
  links want; with `titles` and a `type`, the ones the user names. It is one
  commit with no preview; undo takes it back. Say which pages it will seed
  first.

Every new page joins the index or a map of content in the same plan; a
removed page leaves it; a moved page's entry follows it.

## 3. Preview and apply

One plan: the changed pages, every page whose links changed, and the index.
Show the user, as a list: the paths created, replaced, and deleted; for a
rename or move, the old path and the new, and the count of pages whose links
follow; every warning. A delete or a replace of a page the user did not name
needs their yes. Then `apply`, and report the operation id.

Then run `lint` and report any new dead link or orphan the edit caused. Fix
one the edit caused in a second plan, with the user's yes; leave the rest to
`wiki-review`.

## Hand off

`wiki-review` for the wiki as a whole; `atlas-project` to change the mode
itself.
