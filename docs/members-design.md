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

## Left for later

- A derived "cited by" callout on a member's page, written by the member's
  own hook, if a member should ever learn which hubs link to it.
- Copying captured sources into the hub, if a citation that cannot be opened
  from the hub turns out to matter.
