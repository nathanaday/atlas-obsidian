# Members: knowledge that crosses projects

Status: designed and built 2026-09-21. The plugin and the binary were 5.3.0.
This is the mission `v4-design.md` left for
later, under "Left for later". It replaces both sketches there, the reader
and the hub.

## What changes and why

v4 gave every project one wiki, and a week of use showed that a wiki per
project is the right unit for work inside one project. It is the wrong unit
for work across several. Two microservices each know their own contract and
nothing of the ecosystem they serve. The knowledge of how they meet lives in
no wiki, and an agent asked to change both has no page to read.

A project may now list **members**: other projects, by id. The project's wiki
gains a **mirror** of every member's wiki, and the project's own pages may
link into the mirrors with ordinary wikilinks. The project's own pages are the
fabric: the ecosystem overview, the concept page that says how one service
calls another, the canvas that spans both. Opened in Obsidian, the wiki shows
the whole graph: the fabric, every member's pages, and the links between
them. An agent in a session there reads one wiki.

Three designs were weighed.

| Design | What it gives | Why not |
|---|---|---|
| A copy, refreshed | One vault, the whole graph, backlinks, rename safety | Bytes on disk twice |
| Symlinks into the members | No copy | Obsidian advises against symlinks, does not watch changes through them, and ignores some. Sync services skip them |
| Federation: the hub routes, links cross vaults by URI | No copy, no sync | A wikilink resolves only inside its vault. A URI is not a graph edge, has no backlink, and breaks on rename |

The copy won. Text is small, and the cost that looked large, the merge, is
not there: the mirror is derived, so nothing merges. This is how Antora builds
one site from many repositories, and how a data warehouse differs from a
federated query. The source keeps its pages and its history; the aggregate is
a build.

## Principles

1. **There is no hub entity.** Any project may list members. A project with
   members is a hub only in the sense that its wiki carries mirrors. Its
   threads, its inbox, and its own pages work as they do in any project.
2. **A member never knows.** Sync reads a member's wiki and writes only the
   project that lists it. Nothing is written into a member. A member may sit
   in any number of hubs.
3. **The mirror is derived.** Code owns `wiki/projects/`. The guard refuses
   Write, Edit, and `plan` there. Sync is idempotent: the same members in the
   same state produce no change. This is rule 4 of `CLAUDE.md` over a whole
   folder.
4. **The closure is flat, and each project appears once.** A hub mirrors the
   transitive closure of its members, but every project contributes only its
   own pages, never its mirrors. A grandchild lands at `projects/<name>/` in
   the top hub, not under the mid hub's mirror. Self is excluded from the
   closure, so sync always terminates. A cycle is refused when the member
   list is edited, and sync and status refuse one that arrived another way.
   Lint reads the folder alone and reports what the folder shows: a member
   with no mirror, a mirror with no root page.
5. **Pages, not bytes.** Sync copies markdown and canvas files. It skips
   `.raw/`, the ledger, `log.md`, `hot.md`, and the member's own `projects/`.
   A citation resolves because the source page is mirrored; the captured
   bytes are not.
6. **Git is the history, not the transport.** Sync reads a member's working
   tree, so it assumes nothing about how the member is versioned. When the
   member's engine has a HEAD, sync records the sha as a fact, the way the
   describe page records a commit. `git subtree` would be the tool if the
   mirror were a byte copy; every rewrite the transform makes would conflict
   with the next subtree merge.
7. **Limits are fixed and small.** A project lists at most 128 members, and a
   closure holds at most 128 projects. Either overrun refuses the whole sync
   rather than mirroring a part.

## Layout

```text
atlas/<hub>/
├── project.json                  identity; gains "members": ["<id>", ...]
└── wiki/
    ├── index.md  log.md  hot.md  overview.md
    ├── sources/  entities/  concepts/  canvases/     the hub's own pages: the fabric
    └── projects/                 derived; the guard refuses writes here
        ├── projects.md           the index of the mirrors, generated
        └── <name>/               one member's wiki, transformed
            ├── <name>.md         the member's index.md, renamed
            ├── overview.md
            ├── sources/  entities/  concepts/  canvases/
            └── ...               whatever the member's mode lays out
```

`projects/projects.md` follows the rule that a folder's index page takes the
folder's name. It lists each mirrored project: name, id, the commit recorded,
and a link to the mirror's root page. It records no time, so a sync with
nothing new writes nothing on any day; the hub's history says when each sync
ran. It is the hub graph's entry into every member.

## The mechanism

```text
sync(hub):
  closure = resolve(hub.members)   ids to projects via the atlas config; DFS with a visited set; self excluded
  refuse when len(closure) > 128, or when two projects in it share a folder name
  desired = {}
  for m in closure:
    for f in walk(m.wiki) minus the excluded set:
      desired["projects/<m.name>/" + f] = transform(m, f)
  desired["projects/projects.md"] = index(closure)
  write each file whose bytes differ; delete each file under projects/ not in desired
  commit once, as a sync operation, through the engine
```

**Resolving the tree.** The source of a member is its `wiki/` folder as it
sits on disk. The atlas config maps id to path (`registry.Scan`). A member the
config cannot find is skipped and reported, and its old mirror stays. Deleting
on a failed read would turn a machine without the repository into data loss in
the hub.

**The transform, per markdown page.** Three edits, each a pure function of the
input.

- Frontmatter gains `project` (the member's id), `mirror_of` (the path inside
  the member's wiki), and `commit` when the member's engine has a HEAD. Every
  existing property stays.
- Every wikilink, embed, and markdown link to a page is resolved against the
  member's wiki with the resolver lint uses, then rewritten to a full vault
  path under `projects/<name>/`. A link that already points into `projects/`
  stays as written; that is what lands a mid hub's fabric pages correctly in
  the top hub. A link that does not resolve stays as written, and hub lint
  reports it, as it would in the member.
- The member's `index.md` becomes `projects/<name>/<name>.md`, so no page
  shares a basename with the hub's `wiki/index.md`. Links to it are rewritten
  to match.

A `.canvas` file gets the same rewrite on its file nodes, since it stores
vault paths as JSON.

Full vault paths are what make the mirror robust in Obsidian. They resolve
whatever the hub's link-format setting, and two members that both hold
`entities/Config.md` never collide.

**Reconciling.** There is no merge. The desired set is computed in full and
the folder is made to match it: write on difference, delete on absence, remove
a member's whole folder when it leaves the closure. This is the loop
`threads.Sync` runs. A page a user hand-edited in a mirror is committed as
`manual` by the next apply, as any hand edit in the wiki is, and then
overwritten. The guard makes that rare.

**Committing.** One `sync` operation through the engine, so `history` shows
what each refresh brought in and `undo` restores the previous mirror. Undo of
a sync is useful only until the next sync.

**Triggers.** The hub's session-start hook, and the `sync` command and tool.
`refresh` does not sync, because refresh writes nothing git tracks. Sync never
edits the member list. The list changes only through `project`, and that edit
runs the cycle and size checks before saving.

## What a hub session gets

One wiki with everything: the fabric, every member's pages, ordinary
wikilinks, backlinks, and a `project` property on every mirrored page that
routes back to the member. Routing, the one good part of the federation
design, survives as metadata. Work across members opens threads in the
members through the thread tool's `project` argument, which exists today.

## Consequences accepted

- A top hub sees a mid hub's own fabric pages and the grandchildren
  themselves, never the mid hub's view of a grandchild. Mirrors do not nest,
  and a hub cannot mirror a partial view.
- The mirror's source ledger is not copied. A mirrored source page cites a
  capture that lives in the member.
- Two projects in one closure with the same folder name refuse the sync.
  Rename one.

## Threads cross projects

Added 2026-09-22, with 5.4.0. The same closure, the same transform, and the
same reconcile loop, pointed at `threads/` as well as `wiki/`.

A hub that tracks threads holds `threads/projects/<name>/` for every project
in its closure that tracks them: the member's cards, `archive/`, the four
stage folders, `phases/`, and its board, at the paths they have in the
member, so the relative links inside a document still hold. Every mirrored
page carries `project` and `mirror_of`; a thread page carries no `commit`,
because the threads are outside the engine. One link resolver covers both
halves: a wiki page that cites a thread document, or a thread document that
cites a wiki page, resolves in the hub. The hub's own board ends with one
section per mirrored member that embeds the member's mirrored board, so a
session in the hub reads one page for the whole ecosystem.

`sync` does both halves in one pass over one closure. The wiki half commits
as one `sync` operation, as before. The thread half is plain files, reconciled
the way `threads.Sync` reconciles: write on difference, delete on absence,
remove a member's folder when it leaves the closure. A stray file under
`threads/projects/` goes at the next sync. `mirror.SyncThreads` applies the
thread half alone, for the thread tools to call after a write in a member.

**A thread's writes go to its owner.** `mirror.FindThread` resolves a key in
the project, then in the projects it mirrors, and returns the owner. The
`thread` tool and the CLI's `thread` command run the change there and bring
the hub's thread mirror up to date; the `threads` tool in a hub lists the
member boards after its own, live from the members' working trees, never from
the mirror. A title that matches in two projects is refused with the ids. A
thread the hub owns is an ecosystem-wide thread; nothing new is needed for it.
There is no reverse sync: a hand-made file in a mirror is removed, not moved.

**Threads are an opt-in.** `project.json` gains `threads: true`, and a
project without it has no thread folders, no thread lines in the hook, and
thread tools that refuse with the call that turns them on. A hub mirrors the
threads of a member only when both have them on. Turning them off changes no
file. This is the first step toward a project that holds only the models its
user wants; a user-defined model, with its own stages, fields, templates, and
skills, is left for later.

| Question | Decision |
|---|---|
| The thread mirror's writer | plain files, like every other page under `threads/`; not the engine |
| Where the hub reads member boards for routing | the members' working trees, live; the mirror is for reading in Obsidian |
| How the hub board shows members | an embed of each mirrored board, so the board renders with no member present |
| A hand-made file in a mirror | removed at the next sync; the guard makes it rare |
| The opt-in key | `threads: true`; absent means off; a future model adds its own key |

## Merge: what two members wrote about one thing

Added 2026-09-22, with 5.5.0. Each member wrote its pages alone, so two of
them hold pages about one thing without knowing it: cs513-course's "CS513
Course Project" and cs513-project's "cs513-project" describe one group
project, and neither links the other. The overlap is visible only from the
hub. Reading every mirrored page into a session to look for it costs more
than it finds, so the finding is code and the deciding is the model.

**`overlap` is the finding.** A read-only tool and command over the hub's
wiki as it sits on disk: its own pages are one origin, each mirror another.
It reads the pages as lint reads them (`lint.Vault.Pages`), tokenizes each
into a TF-IDF vector with the title, aliases, and headings weighted, and
scores every pair of pages from two origins three ways: name (equal once
reduced to letters and digits, or a typing distance apart), content (the
cosine of the vectors), and links (the Jaccard index of the names each page
links). A pair appears when a name matches, the content cosine reaches 0.2,
or a quarter of the link names coincide; it is a `duplicate` at a matching
name or a cosine of 0.6, `related` otherwise. Each pair carries its evidence
(the terms and the link names both share) and whether a hub page already
settles it: one named like either page, or one that links both. The report
also lists the link names two or more origins share that resolve to no hub
page, or to a different page in each origin, and the tags they share. It is
bounded (thirty of each by default), deterministic (term ids follow the
alphabet, ties break by path), and needs no config: the mirror is what the
hub sees. `member` narrows it to one origin, for a member adopted into a hub
that merged before.

**`wiki-merge` is the deciding.** The skill reads the report, then only the
pages the candidates name, and proposes one merge in one message: pages to
upgrade, bridges to build, candidates to leave, each with a reason. Two
moves:

- An **upgrade** writes one page in the hub from the member versions, citing
  the members' source pages through the mirror, and leaves a **pointer** in
  each member: the same file, its frontmatter gaining `moved_to` (the path
  in the hub) and `moved_to_project` (the hub's id), its body one line. The
  member's links to it still resolve.
- A **bridge** is a new hub page that links the pages it joins. Nothing in a
  member changes.

**A member changes only as its own operation.** `plan` takes `project`, as
the thread tools do, and the kind `merge` writes under `wiki/` like a save.
The skill plans the pointers in each member, shows the preview, applies
there, then plans the hub's pages and applies here, then syncs. Two
repositories are two commits whatever wraps them, and the user sees each.
This is the one write into a member the design allows, and principle 2 now
reads: sync never writes a member; a merge writes one only as that member's
own reviewed operation.

**Sync honours the pointer.** A member page whose `moved_to_project` is this
hub and whose `moved_to` names a page the hub holds is not mirrored, and
every member link that resolves to it is rewritten to the hub's page
(`mirror.movedInto`). In the hub the graph has one page for the thing, and
every member's link lands on it. A pointer that names another hub, or a page
the hub does not hold, is mirrored as written. `overlap` leaves a moved page
out, so a settled upgrade never comes back as a candidate.

| Question | Decision |
|---|---|
| Delete the member page or leave a pointer | a pointer: deleting breaks every link to it and asks the model to rewrite each page that linked it; the pointer keeps the member's graph whole with one write and becomes a redirect in the hub |
| Compare the mirrors or the members' working trees | the mirrors: what the hub sees, kept current by sync, no config needed |
| The similarity measures | TF-IDF cosine, lint's name key and edit distance, Jaccard over link names: small, standard, in the binary; an embedding model is a dependency and a network call for a gain the report does not need yet |
| One merge tool that writes both repositories, or `project` on `plan` | `project` on `plan`: the same reviewed operation the user knows, one commit per repository, each previewed |
| Where the thresholds live | constants in `overlap`, tuned on a real hub; the report keeps every score so the model can disagree |

## Left for later

- A derived "cited by" callout on a member's page, written by the member's
  own hook, if a member should ever learn which hubs link to it.
- Copying captured sources into the hub, if a citation that cannot be opened
  from the hub turns out to matter.
- A posting-list cut in `overlap` (compare only pages that share a term), if
  a hub past a few thousand pages makes the all-pairs loop slow.
- Overlapping threads across members.
