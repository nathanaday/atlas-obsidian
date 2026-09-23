# Threads and phases

A thread is one line of work in a project: an issue, a feature, a chore. It
moves through four stages, and each stage is a document in its own folder
under `atlas/<name>/threads/`. Read this before any thread skill changes a page.

| Stage | Folder | The document holds | Skill |
|---|---|---|---|
| stub | `threads/stubs/` | where the thread begins, in the user's words | `thread-stub` |
| spec | `threads/specs/` | what will be true when the thread is done, and why | `thread-spec` |
| plan | `threads/plans/` | how the work will go, then its progress | `thread-plan`, `thread-run` |
| receipt | `threads/receipts/` | how the thread ended: completed or killed | `thread-receipt` |

`thread-work` moves a thread from one stage to the next by running the stage
skill, and `thread` is the home: the board, the cards, the phases.

## The rule that matters

The stage of a thread is never set. It is the furthest document that exists.
A thread moves to a stage when the `thread` tool files that stage's document,
and in no other way. So every change of stage leaves a page the user can open
in Obsidian. That visibility is the reason the project folder exists. Never
move a thread along in conversation only: file the document, then say where
it is.

A stage may be skipped. A one-line fix goes from stub to plan, or from stub
to receipt. A skipped document may still be filed later. A receipt closes the
thread.

## The pages

```text
atlas/<name>/
├── wiki/                       the project's knowledge, a separate half
├── threads/
│   ├── threads.md              the board, generated
│   ├── <Title>.md              the card of an open thread, generated
│   ├── archive/<Title>.md      the card of a closed thread
│   ├── stubs/<Title>.md
│   ├── specs/<Title>.md
│   ├── plans/<Title>.md
│   ├── receipts/<Title>.md
│   └── phases/<Title>.md
└── inbox/                      what the user drops in: notes, and sources
```

The card holds the thread's identity: `thread_id` (like
`thr-20260917-3f2a`), `title`, `priority` (high, normal, low, someday),
`phase`, `blocked`, `created`, `updated`. It shows `stage` and `outcome`,
which code writes from the documents. Its body links and embeds every
document, so one page shows the whole thread.

A document carries `type` (its stage), `thread` (the id), `title`, and
`created`; a receipt also carries `outcome`. Its first callout, in the
stage's color, is the way to the thread's other documents. Code owns that
callout and rewrites it. Everything under it is prose.

| Who writes | What |
|---|---|
| The `thread` and `phase` tools | the cards, the board, each document's frontmatter and first callout, a new document from `text` |
| Edit | the prose of a document that exists, and of a phase page |
| Nobody by hand in a session | `threads/` and `project.json`; the hook refuses, and it refuses a new file written straight into a stage folder |

The thread files are the work's own files, tracked like the code, and no
tool commits them. They go in the work's next commit; `thread-run` commits
the plan's progress with each slice.

The user may edit any page in any editor. Deleting a document by hand moves
the thread back to the stage before it.

## The tools

| To | Call |
|---|---|
| See the board, or one thread with the path of each document | `threads`, with `id` for one |
| Open a thread | `thread` with `text` (the stub), and optionally `title`, `priority`, `phase`, `from` (an inbox note, removed once the stub exists) |
| File a document | `thread` with `id`, `stage`, `text`; a receipt also takes `outcome`: `completed` or `killed` |
| Change a card | `thread` with `id` and `priority`, `phase`, `blocked`, or `title` |
| Block, unblock | `thread` with `id` and `blocked`: what it waits on, or `""` |
| Open a closed thread again | `thread` with `id` and `reopen`; the receipt is deleted |
| A phase | `phase` with `action` create, rename, reorder, or remove |

`id` is the thread's id, its title, or the start of its title. `text` is
markdown with no frontmatter. A document that exists is never filed again;
revise it with Edit. An Edit on a document marks its thread as updated today;
nothing else needs to say so.

Every tool acts on this session's project. `project` names another project the
atlas lists.

## Threads across projects

A project that lists members mirrors their threads under
`threads/projects/<name>/`, and its board embeds each member's board at the
end. The mirror is read-only: the hook refuses an edit there and names the
owner. A thread named by id or title may belong to a member; the `thread` tool
finds the owner, makes the change there, and brings the mirror up to date, so
call it as you would for the project's own thread. `threads` on such a project
lists the member boards after its own. A thread the project itself owns is an
ecosystem-wide one.

## Off

A project may have threads off (`threads: false` in `project.json`). The
tools then refuse and say so; the `project` tool with `threads: true` turns
them on. Do not open threads elsewhere to get around it.

## Phases

A phase is a named slice of the timeline with an order and a goal, a page
under `threads/phases/`. A thread names its phase; a phase never lists its
threads.
A phase has no status: it is finished when every thread in it is closed and
it holds at least one.

## Freshness

The documents are the only memory between sessions. Before a session ends,
the plan says where the work stopped and what is next. A thread with a plan
and no update for 14 days is stale; `status`, the hook, and the atlas say so.

## The wiki, the other half

An open thread never reaches the wiki. A completed one may: `thread-receipt`
offers what the work taught, and `wiki-save` or `wiki-describe` writes it as
an operation the user sees. The wiki is evidence for a
spec and a plan; the documents are the state of the work. The two halves sit in
one folder and follow different rules: only an operation writes `wiki/`, while a
document's prose is the model's to write with Edit.
